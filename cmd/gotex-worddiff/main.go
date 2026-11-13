// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// gotex-worddiff says which words one PDF has that the other lacks.
//
// ⭐ WHY IT EXISTS. gotex-refdiff scores a paper against the reference and
// reports a number. When that number moves it does not say WHAT moved: on #772
// two papers lost 0.1 and 0.2 points of word recall and the score could not say
// which words, so there was no way to tell a regression from an improvement the
// metric mis-counts.
//
// ⛔⛔ AND ITS FIRST ACT WAS TO CATCH ITS OWN AUTHOR. The scratch probe this
// file replaces read the PDFs with `pdfops text` and reported that an
// affiliation block had set as `CenterforTheoretical`, `QiluNormalUniversity`
// — words with no spaces. That was published on #772 and #716 before anyone
// checked it. This tool, reading the same two files through the extractor the
// SCORE uses, reported no such difference, and the engine's own line dump
// confirmed the spaces were present in both files all along. The finding was
// the two extractors disagreeing.
//
// So: ⛔ IT READS BOTH FILES THROUGH internal/pdftext, which gotex-refdiff is
// wired onto as well. A word-diff that reached for a different extractor than
// the score reports the extractors' disagreement in the engine's units, which
// is worse than reporting nothing — it is a confident wrong answer, and it
// cost two published corrections to learn that here.
//
// ⚠ IT COMPARES TWO OF OUR OWN PDFS AND NEEDS NO REFERENCE. That is the point:
// it explains a DELTA between two builds of this engine. Measuring against the
// oracle is gotex-refdiff's job and this does not duplicate it.
//
// Words are compared as a MULTISET, not a set: a word the page holds three
// times and the other twice is a difference, and collapsing duplicates would
// hide every repetition a line-breaking change causes.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/go-tex/engine/internal/pdftext"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// extractor is injected so the comparison is testable without poppler.
type extractor func(timeout time.Duration, pdf string) (string, error)

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotex-worddiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	timeout := fs.Duration("timeout", 60*time.Second, "per-PDF extraction timeout")
	quiet := fs.Bool("q", false, "print only the counts, not the words")
	fs.Usage = func() {
		fmt.Fprint(stderr, "usage: gotex-worddiff [-timeout d] [-q] <a.pdf> <b.pdf>\n\n"+
			"Reports which words a.pdf holds that b.pdf does not, and the reverse,\n"+
			"as a multiset difference. Exit 0 when they hold the same words, 1 when\n"+
			"they differ, 2 on a usage or extraction error.\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	return compare(pdftext.CaptureRunner("pdftotext"), *timeout, fs.Arg(0), fs.Arg(1), *quiet, stdout, stderr)
}

func compare(ex extractor, timeout time.Duration, aPath, bPath string, quiet bool, stdout, stderr io.Writer) int {
	a, err := wordsOf(ex, timeout, aPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-worddiff: %v\n", err)
		return 2
	}
	b, err := wordsOf(ex, timeout, bPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-worddiff: %v\n", err)
		return 2
	}
	onlyA, onlyB := surplus(a, b), surplus(b, a)
	report(stdout, aPath, onlyA, quiet)
	report(stdout, bPath, onlyB, quiet)
	if len(onlyA) == 0 && len(onlyB) == 0 {
		return 0
	}
	return 1
}

// wordsOf returns the multiset of words a PDF holds.
//
// ⛔ AN EXTRACTION THAT FAILS IS AN ERROR, NOT AN EMPTY PAGE. A tool that read
// nothing because poppler was missing would otherwise report every word of the
// other file as a difference — a maximal, confident, meaningless answer.
func wordsOf(ex extractor, timeout time.Duration, pdf string) (map[string]int, error) {
	if fi, err := os.Stat(pdf); err != nil {
		return nil, fmt.Errorf("%s: %w", pdf, err)
	} else if fi.Size() == 0 {
		return nil, fmt.Errorf("%s: empty file", pdf)
	}
	text, err := ex(timeout, pdf)
	if err != nil {
		return nil, fmt.Errorf("extracting %s: %w", pdf, err)
	}
	m := map[string]int{}
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		for _, f := range strings.Fields(sc.Text()) {
			if f = strings.Trim(f, ".,;:()[]{}"); f != "" {
				m[f]++
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", pdf, err)
	}
	return m, nil
}

// surplus returns the words a holds more often than b, each with its surplus
// count, sorted so two runs read the same.
func surplus(a, b map[string]int) []string {
	var out []string
	for w, n := range a {
		if d := n - b[w]; d > 0 {
			if d == 1 {
				out = append(out, w)
			} else {
				out = append(out, fmt.Sprintf("%s ×%d", w, d))
			}
		}
	}
	sort.Strings(out)
	return out
}

func report(w io.Writer, pdf string, words []string, quiet bool) {
	fmt.Fprintf(w, "only in %s (%d)\n", pdf, len(words))
	if quiet {
		return
	}
	for _, s := range words {
		fmt.Fprintf(w, "  %s\n", s)
	}
}
