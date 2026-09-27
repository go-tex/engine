// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// mathGeomPkgs compiles one formula with the given packages requested and returns its
// box, so a switch's chosen branch can be compared against that branch written out.
func mathGeomPkgs(t *testing.T, pkgs, preamble, body string) mathNode {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{`+pkgs+`}`+preamble+
		`\begin{document}`+body+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	if len(e.mathDropped) != 0 {
		t.Fatalf("%s: the math layer refused it: %v", body, e.mathDropped)
	}
	m, ok := firstMath(e.mvl)
	if !ok {
		if m, ok = firstMath(e.parList); !ok {
			t.Fatalf("%s: no math box on the page", body)
		}
	}
	return m
}

// The branches are distinguished by their CONTENT and not by a face. A document macro
// wrapping \mathtt loses it before the maths layer sees the formula — classkernel.go
// defines \def\mathtt#1{#1} as a text-mode fallback and flattenMathBody expands it —
// which is a separate defect, and using a face here would have tested that instead.
const xsTensor = `\newcommand{\tensor}[2]{\IfEqCase*{#1}{{0}{#2}{1}{\boldsymbol{#2}}` +
	`{2}{\boldsymbol{#2}}}[QQQQQ]}`

// xstring's \IfEqCase is a string switch, and the corpus paper that uses it loses 33
// equations to it — the largest single item in the dropped-equation census. Its
// machinery cannot be expanded textually (\xs_testcase recursing over \_nil
// delimiters, xstring.tex:626-645), but its DECISION can, which is what the engine
// rewrites.
//
// The test compares each branch against that branch written out, rather than against
// a number: "the switch chose case 1" is only meaningful as "it produced what case 1
// says". A test that merely checked the equation was not dropped would pass on a
// switch that always chose the first case.
func TestIfEqCaseChoosesTheMatchingBranch(t *testing.T) {
	for _, c := range [][2]string{
		{`$\tensor{0}{B}$`, `$B$`},
		{`$\tensor{1}{A}$`, `$\boldsymbol{A}$`},
		{`$\tensor{2}{C}$`, `$\boldsymbol{C}$`},
		// No case matches, so the optional [else] branch is taken. Dropping it silently
		// would leave the symbol out of the page altogether.
		{`$\tensor{9}{D}$`, `$QQQQQ$`},
	} {
		got := mathGeomPkgs(t, "amsmath,xstring,bm", xsTensor, c[0])
		want := mathGeomPkgs(t, "amsmath,xstring,bm", xsTensor, c[1])
		if got.width != want.width || got.height != want.height || got.depth != want.depth {
			t.Errorf("%s = %d/%d/%d, %s gives %d/%d/%d", c[0], got.width, got.height, got.depth,
				c[1], want.width, want.height, want.depth)
		}
	}
}

// \IfEqCase falls back to NUMERIC equality when both sides are decimals, and
// \IfStrEqCase does not — xstring.tex:588-596 is \xs_IfStrEqFalse_ii versus _i, and
// folding the two together would make the second wrong.
//
// "1.0" against the case "1" is the witness: equal as numbers, different as strings.
func TestIfEqCaseComparesNumericallyAndIfStrEqCaseDoesNot(t *testing.T) {
	const eq = `\newcommand{\pick}[1]{\IfEqCase*{#1}{{1}{A}}[QQQQQ]}`
	const str = `\newcommand{\pick}[1]{\IfStrEqCase*{#1}{{1}{A}}[QQQQQ]}`
	one := mathGeomPkgs(t, "amsmath,xstring", eq, `$A$`)
	other := mathGeomPkgs(t, "amsmath,xstring", eq, `$QQQQQ$`)
	if one.width == other.width {
		t.Skip("the two branches render alike, so this test could not tell them apart")
	}
	if got := mathGeomPkgs(t, "amsmath,xstring", eq, `$\pick{1.0}$`); got.width != one.width {
		t.Errorf(`\IfEqCase{1.0} against case {1}: width %d, want the matched branch's %d `+
			`— the numeric fallback did not fire`, got.width, one.width)
	}
	if got := mathGeomPkgs(t, "amsmath,xstring", str, `$\pick{1.0}$`); got.width != other.width {
		t.Errorf(`\IfStrEqCase{1.0} against case {1}: width %d, want the else branch's %d `+
			`— it must compare as STRINGS only`, got.width, other.width)
	}
}

// Nothing is rewritten without \usepackage{xstring}, as for the physics and leftindex
// packages: the names could be a document's own.
func TestIfEqCaseIsGatedOnThePackage(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+
		`\begin{document}$\IfEqCase*{1}{{1}{x}}$\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.mathDropped) == 0 {
		t.Error(`\IfEqCase was rewritten without \usepackage{xstring}`)
	}
}

// A malformed switch is left for go-tex/math to report, not silently swallowed along
// with whatever followed it. An odd trailing group in the case list falls to the else
// branch rather than being paired with the next case's code.
func TestIfEqCaseRefusesToGuessAtAMalformedSwitch(t *testing.T) {
	const odd = `\newcommand{\pick}[1]{\IfEqCase*{#1}{{7}{A}{8}}[QQQQQ]}`
	other := mathGeomPkgs(t, "amsmath,xstring", odd, `$QQQQQ$`)
	if got := mathGeomPkgs(t, "amsmath,xstring", odd, `$\pick{8}$`); got.width != other.width {
		t.Errorf("a case with no code: width %d, want the else branch's %d", got.width, other.width)
	}
}
