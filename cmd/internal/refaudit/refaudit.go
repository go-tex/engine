// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package refaudit decides whether a REFERENCE PDF can stand as a reference.
//
// Two corruptions are known, and both make the reference's page count a number
// that measures the reference's own defect:
//
//   - a hyperref pdfTeX-driver leak: a source that forces \pdfoutput=1 and is
//     built by an XeTeX-family engine loads hpdftex.def, whose \pdfstartlink,
//     \pdfdest and \pdfinfo arguments are then typeset as body text, on every
//     page;
//   - a junk first page: preamble material (the arguments of an undefined
//     \DeclareUnicodeCharacter, say) typeset before \begin{document} and shipped
//     out as a page of its own, which shifts every later page by one.
//
// Measured over the 154 reference PDFs of the corpus: 10 leak, 3 have a junk first
// page.
package refaudit

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/go-tex/engine/cmd/internal/pdfruns"
)

// leakPatterns are matched against a page's text with every space removed, so a
// string split across show operators or justified apart is still found.
var leakPatterns = []string{
	"namepage.",                  // \pdfdest name{page.1} XYZ
	"attr/Border[",               // \pdfstartlink attr{/Border[0 0 1]...}
	"gotoname",                   // \pdfstartlink ... goto name{...}
	"Creator(LaTeXwithhyperref)", // \pdfinfo{/Creator(LaTeX with hyperref)}
	"/Producer(",                 // \pdfinfo{... /Producer(...)}
}

// SparseLimit is the page-1 non-space character count under which a document of
// several pages is reported as having a junk first page.
const SparseLimit = 80

// Verdict is what one reference PDF answered.
type Verdict struct {
	Pages      int
	LeakPages  int      // pages on which at least one pattern matched
	FirstLeak  int      // first such page, 0 when none
	Patterns   []string // the distinct patterns seen, sorted
	Page1Chars int      // non-space characters on page 1
}

// JunkFirst reports the second corruption.
func (v Verdict) JunkFirst() bool { return v.Pages > 1 && v.Page1Chars < SparseLimit }

// Leaky reports the first corruption.
func (v Verdict) Leaky() bool { return v.LeakPages > 0 }

// Unusable says why the reference cannot be scored against, or "" when it can. A
// leak is named first: it is the more thorough corruption.
func (v Verdict) Unusable() string {
	switch {
	case v.Leaky():
		return fmt.Sprintf("hyperref pdfTeX-driver leak on %d of %d pages (first on page %d)", v.LeakPages, v.Pages, v.FirstLeak)
	case v.JunkFirst():
		return fmt.Sprintf("junk first page (%d characters on page 1 of %d)", v.Page1Chars, v.Pages)
	}
	return ""
}

// Squeeze removes every space-like rune.
func Squeeze(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', ' ':
			return -1
		}
		return r
	}, s)
}

// MatchLeaks returns the patterns found in a page text.
func MatchLeaks(pageText string) []string {
	sq := Squeeze(pageText)
	var out []string
	for _, p := range leakPatterns {
		if strings.Contains(sq, p) {
			out = append(out, p)
		}
	}
	return out
}

// Probe reads one reference PDF. An error means it could not be read, which is
// NOT a clean verdict.
func Probe(path string) (Verdict, error) {
	var v Verdict
	b, err := os.ReadFile(path)
	if err != nil {
		return v, err
	}
	doc, err := pdfruns.Open(b)
	if err != nil {
		return v, fmt.Errorf("%s: %w", path, err)
	}
	v.Pages = doc.PageCount()
	if v.Pages == 0 {
		return v, fmt.Errorf("%s: parsed, but it has no pages", path)
	}
	seen := map[string]bool{}
	for n := 1; n <= v.Pages; n++ {
		runs, err := pdfruns.PageRuns(doc, n)
		if err != nil {
			return v, fmt.Errorf("%s: page %d: %w", path, n, err)
		}
		var sb strings.Builder
		for _, r := range runs {
			sb.WriteString(r.Text)
		}
		if n == 1 {
			v.Page1Chars = len([]rune(Squeeze(sb.String())))
		}
		hits := MatchLeaks(sb.String())
		if len(hits) == 0 {
			continue
		}
		v.LeakPages++
		if v.FirstLeak == 0 {
			v.FirstLeak = n
		}
		for _, h := range hits {
			seen[h] = true
		}
	}
	for p := range seen {
		v.Patterns = append(v.Patterns, p)
	}
	sort.Strings(v.Patterns)
	return v, nil
}
