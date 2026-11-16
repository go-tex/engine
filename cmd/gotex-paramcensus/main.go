// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex-paramcensus answers one question about the fidelity corpus:
// WHICH MEASURED PAPERS SET A GIVEN TeX PARAMETER?
//
// It exists because the obvious way to ask — `grep -rl looseness <paper tree>`
// — is wrong in five separate ways, and each of them reads as a clean answer:
//
//  1. ⛔ IT ANSWERS ABOUT THE WRONG POPULATION. The paper tree holds thousands
//     of papers; the A/B samples the few hundred named in a list file. A count
//     over the tree says nothing about what a measurement would see.
//  2. ⛔ THE LIST NAMES EACH PAPER'S ENTRY FILE, NOT ITS DIRECTORY, and walking
//     a file visits that file alone. Measured on go-tex's own list: 154 entries
//     give 154 .tex files that way and 679 when each paper's directory is
//     walked, so better than three quarters of the corpus text — every
//     \input'ed chapter — is skipped by a scan that looks complete.
//  3. ⛔ A COMMENTED-OUT SETTING IS NOT A SETTING. TeX does not see it, so
//     neither does this: a `%` not preceded by an odd number of backslashes
//     ends the line. (An escaped \% is a percent sign, not a comment.)
//  4. ⛔ A BARE SUBSTRING MATCHES PROSE. A paper discussing "looseness" does
//     not set \looseness, so terms are matched as written, backslash included.
//  5. ⛔ AND A CONTROL WORD IS A PREFIX OF LONGER CONTROL WORDS. This one
//     survived the backslash rule above and was found by disbelieving an
//     answer: asked for `\font`, a substring scan reported 14 of 154 papers,
//     and the matching lines were `\fontsize{13}{16}`, `\fontsize{\sf@size}`,
//     `\Affilfont` — not one use of the primitive. TeX ends a control WORD at
//     the first non-letter, so a term that is one is matched to that boundary:
//     `\font` no longer answers for `\fontsize`, while `\par` still answers for
//     `\par` at an end of line and never for `\paragraph`. A control SYMBOL
//     (`\\`, `\'`, `\%`) is exactly one character and takes no boundary.
//     ⚠ The boundary uses the DEFAULT catcodes, where only A-Za-z are letters.
//     Inside a class or package under \makeatletter, `@` is a letter too, so
//     `\font` would still answer for a `\font@foo` written there. That case is
//     stated rather than guessed at, because the catcode regime of a line is
//     not decidable by a scanner that does not expand the document.
//
// ⭐ AND IT REFUSES RATHER THAN REPORTING A ZERO IT CANNOT STAND BEHIND. Every
// run must find its CONTROL term — by default \documentclass, which every LaTeX
// paper has — in at least one paper, and must have read at least one file.
// A scan that could not read reports zero exactly like a corpus that holds
// nothing, and the only thing separating them is the volume read, which is
// printed before the result.
//
// Usage:
//
//	gotex-paramcensus -list /path/clean-list.txt '\looseness' '\emergencystretch'
//	gotex-paramcensus -corpus /path/papers -show 40 '\hangindent'
//
// Exit status: 0 the census ran, 2 it refused (bad usage, nothing read, or the
// control term absent).
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("gotex-paramcensus", flag.ContinueOnError)
	fl.SetOutput(stderr)
	list := fl.String("list", "", "a file of paper paths, one per line (the list gotex-abdiff and gotex-refdiff take)")
	corpus := fl.String("corpus", "", "a directory of paper sub-directories, instead of -list")
	control := fl.String("control", `\documentclass`, "a term that MUST be found, so that a zero cannot be a scan that read nothing")
	show := fl.Int("show", 25, "how many matching lines to print (0 for none)")
	if err := fl.Parse(args); err != nil {
		return 2
	}
	terms := fl.Args()
	if (*list == "" && *corpus == "") || len(terms) == 0 {
		fmt.Fprintln(stderr, "gotex-paramcensus: one of -list or -corpus is required, with at least one term")
		fl.PrintDefaults()
		return 2
	}

	dirs, err := paperDirs(*list, *corpus)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-paramcensus: %v\n", err)
		return 2
	}
	if len(dirs) == 0 {
		fmt.Fprintln(stderr, "gotex-paramcensus: no paper named: refusing, since a census of nothing is not a census")
		return 2
	}

	c := census(dirs, append(terms, *control), *show)

	// ⭐ The population comes BEFORE the result, every time.
	fmt.Fprintf(stdout, "population: %d paper(s) of %d named, %d .tex file(s)\n", c.papers, len(dirs), c.files)
	if c.papers == 0 || c.files == 0 {
		fmt.Fprintln(stderr, "gotex-paramcensus: read no .tex file at all: refusing, since this found nothing because it looked at nothing")
		return 2
	}
	if n := len(c.hits[*control]); n == 0 {
		fmt.Fprintf(stderr, "gotex-paramcensus: the control term %q was found in NO paper: refusing, since a scan that cannot match what is certainly there cannot be believed about what is not\n", *control)
		return 2
	} else {
		fmt.Fprintf(stdout, "control:    %s in %d paper(s)\n", *control, n)
	}
	for _, t := range terms {
		ids := make([]string, 0, len(c.hits[t]))
		for id := range c.hits[t] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintf(stdout, "%s: %d of %d paper(s)", t, len(ids), c.papers)
		if len(ids) > 0 {
			fmt.Fprintf(stdout, "  %s", strings.Join(ids, " "))
		}
		fmt.Fprintln(stdout)
	}
	for _, l := range c.lines {
		fmt.Fprintln(stdout, "  "+l)
	}
	// -show 0 asks for no lines, so it does not get a tally of the ones withheld.
	if c.more > 0 && *show > 0 {
		fmt.Fprintf(stdout, "  … and %d more line(s)\n", c.more)
	}
	return 0
}

// paperDirs resolves the papers to scan to one DIRECTORY each. A list entry
// naming a file (clean-list.txt names each paper's main.tex) resolves to the
// directory holding it, which is the paper; see trap 2 in the package comment.
func paperDirs(list, corpus string) ([]string, error) {
	if list != "" {
		b, err := os.ReadFile(list)
		if err != nil {
			return nil, err
		}
		seen := map[string]bool{}
		var out []string
		for _, ln := range strings.Split(string(b), "\n") {
			ln = strings.TrimSpace(ln)
			if ln == "" {
				continue
			}
			d := ln
			if fi, err := os.Stat(ln); err == nil && !fi.IsDir() {
				d = filepath.Dir(ln)
			}
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
		return out, nil
	}
	ents, err := os.ReadDir(corpus)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, filepath.Join(corpus, e.Name()))
		}
	}
	return out, nil
}

type result struct {
	papers, files int
	hits          map[string]map[string]bool // term -> set of paper ids
	lines         []string
	more          int
}

func census(dirs, terms []string, show int) result {
	r := result{hits: map[string]map[string]bool{}}
	for _, t := range terms {
		r.hits[t] = map[string]bool{}
	}
	for _, d := range dirs {
		id, before := filepath.Base(d), r.files
		_ = filepath.WalkDir(d, func(p string, de fs.DirEntry, err error) error {
			if err != nil || de.IsDir() || !strings.HasSuffix(p, ".tex") {
				return nil
			}
			src, err := os.ReadFile(p)
			if err != nil {
				return nil
			}
			r.files++
			for _, ln := range strings.Split(string(src), "\n") {
				if k := commentAt(ln); k >= 0 {
					ln = ln[:k]
				}
				for _, t := range terms {
					if !containsTerm(ln, t) {
						continue
					}
					r.hits[t][id] = true
					if len(r.lines) < show {
						r.lines = append(r.lines, id+"/"+filepath.Base(p)+": "+strings.TrimSpace(ln))
					} else {
						r.more++
					}
				}
			}
			return nil
		})
		if r.files > before {
			r.papers++
		}
	}
	return r
}

// containsTerm reports whether ln holds term t, ENDING IT AT TeX's OWN
// CONTROL-WORD BOUNDARY when t is a control word.
//
// ⛔ Without this, a control word answers for every longer one that begins with
// it, and the result reads as a measurement: `\font` reported 14 of 154 corpus
// papers, every hit being `\fontsize` or `\Affilfont`. tex.web's scanner ends a
// control word at the first character that is not of category letter (§354-356
// in the mouth: it accumulates letters while cat=letter), which is what this
// reproduces.
//
// A control SYMBOL — a backslash and ONE non-letter, like `\\` or `\%` — ends at
// that character by definition, so it needs no boundary and gets none; neither
// does a plain word, where the caller asked for a substring and prose is their
// problem (trap 4).
func containsTerm(ln, t string) bool {
	if !isControlWord(t) {
		return strings.Contains(ln, t)
	}
	for i := 0; i <= len(ln)-len(t); {
		j := strings.Index(ln[i:], t)
		if j < 0 {
			return false
		}
		if end := i + j + len(t); end == len(ln) || !isTeXLetter(ln[end]) {
			return true
		}
		i += j + 1
	}
	return false
}

// isControlWord reports whether t is a backslash followed only by letters, which
// is the only shape that TeX terminates at a non-letter.
func isControlWord(t string) bool {
	if len(t) < 2 || t[0] != '\\' {
		return false
	}
	for i := 1; i < len(t); i++ {
		if !isTeXLetter(t[i]) {
			return false
		}
	}
	return true
}

// isTeXLetter is category code 11 under the DEFAULT regime: A-Za-z and nothing
// else. See trap 5 for why `@` is deliberately not here.
func isTeXLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// commentAt returns the index of the character that starts a TeX comment, or
// -1. A % preceded by an ODD number of backslashes is an escaped percent sign
// and not a comment.
func commentAt(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		back := 0
		for j := i - 1; j >= 0 && s[j] == '\\'; j-- {
			back++
		}
		if back%2 == 0 {
			return i
		}
	}
	return -1
}
