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

// topLevelHas reports whether some top-level node of the main vertical list carries
// `want` in its text without `notWith`. It distinguishes "content escaped the
// environment box and became its own page-level material" (fix works) from "content
// was swallowed into the environment's box" (the truncation bug): when an env-body
// scanner runs to EOF, the trailing prose is trapped inside the table/box and every
// node that mentions `want` also mentions the in-env marker.
func topLevelHas(mvl []node, want, notWith string) bool {
	for _, n := range mvl {
		t := mvlText([]node{n})
		if strings.Contains(t, want) && !strings.Contains(t, notWith) {
			return true
		}
	}
	return false
}

// A user macro standing in for \end is read raw by the align/gather body scanner
// (collectAlignBody). Unless it is expanded there, the scanner never sees \end and
// runs to EOF: the trailing prose is fed to the math renderer (or dropped) instead
// of reaching the page. Same swallow class as the equation fix, now covered for
// align, gather and the other amsmath alignments that share collectAlignBody.
func TestMacroWrappedEndClosesAlign(t *testing.T) {
	for _, env := range []string{"align", "gather"} {
		src := `\documentclass{article}
\newcommand{\eqx}{\end{` + env + `}}
\begin{document}
BEFOREMARK
\begin{` + env + `} x = y \eqx
AFTERMARK
\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: compile: %v", env, err)
		}
		txt := mvlText(e.mvl)
		if !strings.Contains(txt, "BEFOREMARK") {
			t.Errorf("%s: text before the env is missing; got %q", env, txt)
		}
		// When the body scanner swallows to EOF the prose becomes math source (SVG),
		// so it never surfaces as page text; its presence proves the env closed.
		if !strings.Contains(txt, "AFTERMARK") {
			t.Fatalf("%s: macro-wrapped \\end failed to close the env; body after it was swallowed; got %q", env, txt)
		}
	}
}

// A user macro standing in for \end{tabular} is read raw by collectTabularBody.
// Unless expanded, the scanner runs to EOF and the rest of the document is trapped
// as extra table cells. The fix closes the table so trailing prose is page-level
// material again.
func TestMacroWrappedEndClosesTabular(t *testing.T) {
	const src = `\documentclass{article}
\newcommand{\etab}{\end{tabular}}
\begin{document}
\begin{tabular}{cc}
CELLMARK & beta \\ gamma & delta \etab

AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "CELLMARK") {
		t.Errorf("tabular cell missing; got %q", txt)
	}
	// Swallowed prose lands inside the table box (still rendered), so "contains
	// AFTERMARK" is not enough — assert it escaped the table as its own page material.
	if !topLevelHas(e.mvl, "AFTERMARK", "CELLMARK") {
		t.Fatalf("macro-wrapped \\end failed to close the tabular; trailing prose stayed inside the table; got %q", txt)
	}
}

// A user macro standing in for \end{minipage} is read raw by collectEnvBody. Unless
// expanded, the minipage never closes and the rest of the document is typeset inside
// its narrow box. The fix closes it so trailing prose is a full-width paragraph.
func TestMacroWrappedEndClosesMinipage(t *testing.T) {
	const src = `\documentclass{article}
\newcommand{\emp}{\end{minipage}}
\begin{document}
\begin{minipage}{3cm}
INSIDEMARK
\emp

AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "INSIDEMARK") {
		t.Errorf("minipage body missing; got %q", txt)
	}
	if !topLevelHas(e.mvl, "AFTERMARK", "INSIDEMARK") {
		t.Fatalf("macro-wrapped \\end failed to close the minipage; trailing prose stayed inside the box; got %q", txt)
	}
}

// A user macro standing in for the closing \] is read raw by collectMathUntilCS.
// Unless expanded, the display-math scanner runs to EOF and the rest of the document
// becomes math source instead of page text. The fix surfaces the real \].
func TestMacroWrappedCloseClosesDisplayMath(t *testing.T) {
	const src = `\documentclass{article}
\newcommand{\dc}{\]}
\begin{document}
BEFOREMARK
\[ x = y \dc
AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("text before the display math is missing; got %q", txt)
	}
	if !strings.Contains(txt, "AFTERMARK") {
		t.Fatalf("macro-wrapped \\] failed to close the display math; body after it was swallowed; got %q", txt)
	}
}

// Verbatim must NOT expand anything: a literal \end{equation} inside a verbatim block
// is ordinary text, and only a literal \end{verbatim} terminates it. This guards the
// deliberate decision to leave the raw-buffer verbatim/lstlisting scanners untouched.
func TestVerbatimKeepsLiteralEndVerbatim(t *testing.T) {
	const src = `\documentclass{article}
\newcommand{\enq}{\end{equation}}
\begin{document}
\begin{verbatim}
\end{equation} and \enq stay literal here
\end{verbatim}
AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	// The literal \end{equation} and \enq are rendered verbatim (their letters reach
	// the page), and the block terminated only at \end{verbatim} so AFTERMARK follows.
	if !strings.Contains(txt, "equation") || !strings.Contains(txt, "enq") {
		t.Errorf("verbatim did not keep its literal \\end text; got %q", txt)
	}
	if !strings.Contains(txt, "AFTERMARK") {
		t.Fatalf("verbatim block did not terminate at \\end{verbatim}; got %q", txt)
	}
}

// expandsToCloseCS (which expandsToEnd delegates to) must stay narrow for any close
// cs: only a parameterless macro whose body BEGINS with the exact close cs qualifies.
// A macro that merely mentions the close cs mid-body, or wraps a different cs, is not
// a shorthand and must not be expanded away by a body scanner.
func TestExpandsToCloseCSIsNarrow(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	// \dc -> \]  : a genuine shorthand for the \[ … \] closer.
	e.define("dc", &meaning{kind: mMacro, body: []tok{csTok("]")}}, false)
	// \midend -> x\end{eq} : \end appears mid-body, not at position 0 — not a shorthand.
	e.define("midend", &meaning{kind: mMacro, body: []tok{chTok('x', catLetter), csTok("end"), chTok('{', catBegin), chTok('e', catLetter), chTok('q', catLetter), chTok('}', catEnd)}}, false)

	if !e.expandsToCloseCS(csTok("dc"), "]") {
		t.Error("a macro whose body begins with the close cs should be recognised")
	}
	if e.expandsToCloseCS(csTok("dc"), "end") {
		t.Error("\\dc wraps \\], not \\end; must not match the \\end closer")
	}
	if e.expandsToEnd(csTok("midend")) {
		t.Error("a macro with \\end mid-body must not be treated as an \\end shorthand")
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
