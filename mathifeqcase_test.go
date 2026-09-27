// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// xstring's \IfEqCase{<test>}{{<case>}{<code>}…}[<else>] (xstring.tex:620-645), resolved on
// the maths SOURCE STRING because a TeX transcription cannot be reached from a formula:
// renderMathResolvingMacros expands parameterless macros against text and can match neither
// xstring's delimited scan (##3\_nil) nor a conditional. The same wall as engine#454, one
// step further along — there the answer was "strip it", here it is "pick a branch".
//
// Corpus paper 2308.09839 ships porousmedia-macros.sty with
//
//	\newcommand{\tensor}[2]{\IfEqCase*{#1}{{0}{#2}{1}{\boldsymbol{#2}}…}[Did not match…]}
//
// used throughout its formulas: one undefined command cost 33 EQUATIONS, the largest single
// entry in the dropped-equation channel. Judged against tectonic 0.17.0, which renders
// "ordre 0: A, ordre 1: B, hors liste: NOMATCH" where we rendered "ordre 0: , ordre 1: ,
// hors liste:" — three formulas dropped whole.
func TestIfEqCaseInMathsPicksItsBranch(t *testing.T) {
	const src = `\documentclass{article}\usepackage{xstring}\usepackage{amsmath}` +
		`\newcommand{\tensor}[2]{\IfEqCase*{#1}{{0}{#2}{1}{\boldsymbol{#2}}}[NOMATCH]}` +
		`\begin{document}` +
		`zero $\tensor{0}{AAA}$ one $\tensor{1}{BBB}$ none $\tensor{9}{CCC}$` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("equation dropped: %v", e.mathDropped)
	}
}

// The case picker on its own, including the two shapes xstring itself treats as errors.
func TestPickEqCase(t *testing.T) {
	const cases = `{0}{ZERO}{1}{\boldsymbol{ONE}}{two}{TWO}`
	for _, c := range []struct{ test, want string }{
		{"0", "ZERO"},
		{"1", `\boldsymbol{ONE}`},
		{"two", "TWO"},
		{"9", "FALLBACK"}, // no case matches
		{"", "FALLBACK"},  // an empty test matches nothing
	} {
		if got := pickEqCase(c.test, cases, "FALLBACK"); got != c.want {
			t.Errorf("pickEqCase(%q) = %q, want %q", c.test, got, c.want)
		}
	}
	// An ODD number of groups is xstring's own error case: the trailing group is dropped
	// rather than guessed at, and the fallback answers.
	if got := pickEqCase("x", `{a}{A}{x}`, "FB"); got != "FB" {
		t.Errorf("an odd case list gave %q, want the fallback", got)
	}
	// Nested braces in the code must not end it early.
	if got := pickEqCase("k", `{k}{\frac{a}{b}}`, "FB"); got != `\frac{a}{b}` {
		t.Errorf("nested braces gave %q", got)
	}
}
