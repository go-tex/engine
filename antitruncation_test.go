// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests guard against a class of silent truncation: a construct that scans
// the input for a delimiter it never recognises and so swallows the rest of the
// document. Each was found on a real arXiv manuscript that rendered a handful of
// pages instead of the full paper. The assertion is always the same: text that
// sits AFTER the offending construct must still reach the page.

// A tabular column specification with array-package prefixes — @{}, >{..}, <{..} —
// has braces nested inside it. The spec scanner must match its own closing brace,
// not the first inner one, or the leftover preamble leaks into the body scanner
// and \end{tabular} is never found. Regression for arXiv 2608.10696 (elsarticle),
// which truncated at 2 pages of 48.
func TestColSpecBracesDoNotSwallowBody(t *testing.T) {
	const src = `\documentclass{article}\begin{document}
BEFOREMARK
\begin{tabular}{@{}>{\raggedright\arraybackslash}p{4cm}>{\raggedright\arraybackslash}p{4cm}@{}}
alpha & beta \\ gamma & delta \end{tabular}
AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("text before the tabular is missing; got %q", txt)
	}
	if !strings.Contains(txt, "AFTERMARK") {
		t.Fatalf("text after the tabular was swallowed; got %q", txt)
	}
	// The cell contents should also survive the corrected spec parse.
	if !strings.Contains(txt, "alpha") || !strings.Contains(txt, "delta") {
		t.Errorf("tabular cells missing; got %q", txt)
	}
}

// A user macro standing in for an environment's \end tag (\newcommand\enq{\end{equation}})
// is read raw by the math-body scanner. Unless it is expanded there, the scanner
// never sees \end and swallows the rest of the document. Regression for arXiv
// 2608.11183 (amsart, \beq/\enq), which truncated at 2 pages of 22.
func TestMacroWrappedEndClosesEquation(t *testing.T) {
	const src = `\documentclass{article}
\newcommand{\enq}{\end{equation}}
\begin{document}
BEFOREMARK
\begin{equation} x = y \enq
AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("text before the equation is missing; got %q", txt)
	}
	if !strings.Contains(txt, "AFTERMARK") {
		t.Fatalf("macro-wrapped \\end failed to close the equation; body after it was swallowed; got %q", txt)
	}
}

// expandsToEnd must stay narrow: an ordinary math macro (parameterless, but whose
// body does not begin with \end) is NOT expanded away by the math-body scanner, so
// the math source keeps it verbatim for the renderer.
func TestExpandsToEndIsNarrow(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.define("enq", &meaning{kind: mMacro, body: []tok{csTok("end"), chTok('{', catBegin), chTok('e', catLetter), chTok('q', catLetter), chTok('n', catLetter), chTok('}', catEnd)}}, false)
	e.define("Rset", &meaning{kind: mMacro, body: []tok{csTok("mathbb"), chTok('{', catBegin), chTok('R', catLetter), chTok('}', catEnd)}}, false)
	if !e.expandsToEnd(csTok("enq")) {
		t.Error("a macro whose body begins with \\end should be recognised")
	}
	if e.expandsToEnd(csTok("Rset")) {
		t.Error("an ordinary math macro must not be treated as an \\end shorthand")
	}
	if e.expandsToEnd(csTok("undefinedxyz")) {
		t.Error("an undefined cs is not an \\end shorthand")
	}
}

// \input names whose basename contains a dot (1.Introduction, 2.1.prelims) must
// still resolve to <name>.tex. The old heuristic treated the dot as an extension
// and never appended .tex, so every section was dropped. Regression for arXiv
// 2608.10848, which truncated at 1 page of 29.
func TestInputCandidatesDottedName(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"1.Introduction", []string{"1.Introduction", "1.Introduction.tex"}},
		{"Sections/2.1.prelims", []string{"Sections/2.1.prelims", "Sections/2.1.prelims.tex"}},
		{"chapter", []string{"chapter.tex", "chapter"}},
		{"figure.tex", []string{"figure.tex", "figure.tex.tex"}},
	}
	for _, c := range cases {
		got := inputCandidates(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("inputCandidates(%q) = %v, want %v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("inputCandidates(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// End-to-end: \input a dotted-name section file from a temp dir and confirm its
// prose reaches the page.
func TestInputDottedNameLoadsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "1.Introduction.tex"), []byte("SECTIONBODYMARK\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	const src = `\documentclass{article}\begin{document}
\input{1.Introduction}
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "SECTIONBODYMARK") {
		t.Fatalf("\\input of a dotted-name file did not load its body; got %q", txt)
	}
}
