// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ligatureFolder rewrites the Unicode presentation-form ligatures back to their
// component ASCII letters. gotex keeps a word like "first" as the single glyph
// "ﬁ" in the PDF's /ToUnicode (a tracked limitation of its PDF text layer),
// while a real LaTeX engine decomposes it to "fi"; folding both sides measures
// the CONTENT fairly instead of penalising the ligature ToUnicode gap. Mirrors
// the sed in scripts/fidelity.sh.
var ligatureFolder = strings.NewReplacer(
	"ﬀ", "ff", // ﬀ
	"ﬁ", "fi", // ﬁ
	"ﬂ", "fl", // ﬂ
	"ﬃ", "ffi", // ﬃ
	"ﬄ", "ffl", // ﬄ
	"ﬅ", "st", // ﬅ (long-s t)
	"ﬆ", "st", // ﬆ
)

// tokenSep splits extracted text into candidate words: any run that is not an
// ASCII letter or digit is a separator (after lower-casing).
var tokenSep = regexp.MustCompile(`[^a-z0-9]+`)

// contentWords reduces an engine's extracted PDF text to the SET of comparable
// content words, applying the three normalisations documented at the top of
// scripts/fidelity.sh so the known-expected engine differences do not count
// against gotex:
//
//   - ligatures are folded (ﬁ→fi) so the ToUnicode gap is not penalised;
//   - purely numeric tokens are dropped — this removes the reference's page
//     numbers (gotex defaults to \pagestyle{empty}) and equation numbers, which
//     carry no prose-fidelity signal;
//   - single-character tokens are dropped — isolated one-letter tokens are
//     dominated by math variables (x, n, i…), and gotex renders math as vector
//     paths that carry no extractable text, so counting them would penalise the
//     by-design math difference.
//
// The result is a set (deduplicated), matching the `sort -u` in fidelity.sh.
func contentWords(text string) map[string]struct{} {
	folded := ligatureFolder.Replace(text)
	folded = strings.ToLower(folded)
	out := make(map[string]struct{})
	for _, tok := range tokenSep.Split(folded, -1) {
		if !isContentToken(tok) {
			continue
		}
		out[tok] = struct{}{}
	}
	return out
}

// isContentToken reports whether a token survives the fidelity normalisation:
// at least two characters and not purely numeric.
func isContentToken(tok string) bool {
	if len(tok) < 2 {
		return false
	}
	for _, r := range tok {
		if r < '0' || r > '9' {
			return true // has a non-digit → keep
		}
	}
	return false // all digits → drop (page/equation number)
}

// recall computes the fraction of the reference's content words that gotex also
// produced: |ref ∩ got| / |ref|. It returns the ratio together with the two
// counts so a report can show the raw evidence. An empty reference yields a
// zero ratio and a zero count (the caller marks the paper unavailable rather
// than scoring it).
func recall(ref, got map[string]struct{}) (ratio float64, refCount, common int) {
	refCount = len(ref)
	if refCount == 0 {
		return 0, 0, 0
	}
	for w := range ref {
		if _, ok := got[w]; ok {
			common++
		}
	}
	return float64(common) / float64(refCount), refCount, common
}

// readmeSource is one entry of an arXiv 00README.json "sources" list.
type readmeSource struct {
	Usage    string `json:"usage"`
	Filename string `json:"filename"`
}

// readme is the subset of 00README.json this tool consumes.
type readme struct {
	Sources []readmeSource `json:"sources"`
}

// beginDocument matches the \begin{document} that only a top-level document has
// (a chapter/section \input fragment does not).
var beginDocument = regexp.MustCompile(`\\begin\s*\{document\}`)

// resolveToplevel finds the single .tex file to compile for a paper directory,
// following the same rule the corpus tooling uses:
//
//  1. if a 00README.json lists a source with "usage":"toplevel", that file wins;
//  2. otherwise the .tex file that contains \begin{document} is the top level.
//
// When several .tex files contain \begin{document} the lexicographically first
// is chosen for determinism. It returns the ABSOLUTE path to the chosen file.
func resolveToplevel(dir string) (string, error) {
	if name, ok := readmeToplevel(dir); ok {
		return filepath.Join(dir, name), nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".tex") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(full)
		if err != nil {
			continue
		}
		if beginDocument.Match(b) {
			candidates = append(candidates, full)
		}
	}
	if len(candidates) == 0 {
		return "", errNoToplevel
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

// errNoToplevel is returned when a paper directory has no resolvable top-level
// .tex (no README pointer and no \begin{document} anywhere).
var errNoToplevel = &noToplevelError{}

type noToplevelError struct{}

func (*noToplevelError) Error() string { return "no top-level .tex found" }

// readmeToplevel returns the "usage":"toplevel" filename from a paper's
// 00README.json, and whether one was found.
func readmeToplevel(dir string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(dir, "00README.json"))
	if err != nil {
		return "", false
	}
	var r readme
	if err := json.Unmarshal(b, &r); err != nil {
		return "", false
	}
	for _, s := range r.Sources {
		if s.Usage == "toplevel" && s.Filename != "" {
			return s.Filename, true
		}
	}
	return "", false
}

// documentClassRE captures the class name from \documentclass[opts]{name}. The
// optional [..] argument is skipped.
var documentClassRE = regexp.MustCompile(`\\documentclass\s*(?:\[[^\]]*\])?\s*\{([^}]+)\}`)

// documentClass extracts the LaTeX class of a document (e.g. "article",
// "revtex4-1"), or "?" when the source declares none in the given text.
func documentClass(tex string) string {
	m := documentClassRE.FindStringSubmatch(tex)
	if m == nil {
		return "?"
	}
	return strings.TrimSpace(m[1])
}
