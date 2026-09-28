// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ An expandable primitive written DIRECTLY in a formula was dropped, while the same
// primitive carried in by a macro BODY has worked since #174: a substituted body is run
// through flattenMathBody, and a source the document wrote itself never was.
//
// A source arrives that way because an argument can be COLLECTED rather than expanded.
// aastex701.cls:2589 sets its affiliation marks with
// \textsuperscript{\expandafter\@affilcomma\@tempa\relax\relax}, and the superscript's
// argument becomes maths source verbatim. Measured on 999 arXiv papers: \expandafter 479
// equations over 17 papers, the largest census trigger the engine already defines (#466).
//
// ⛔ These tests assert what is DRAWN, not that nothing was dropped. A drop that stops
// being reported is not a drop that was fixed — #505 shipped a first figure of −152 that
// was really −83, because 69 equations left the census by rendering the wrong thing.
// Counting glyph paths against an equivalent formula is what tells the two apart.

func mathGlyphPaths(t *testing.T, preamble, math string) int {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+preamble+
		`\makeatother\begin{document}$`+math+`$\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", math, err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("%s: the maths layer refused the formula (%v)", math, e.mathDropped)
	}
	return strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path")
}

// \expandafter\joinc\tmpA must draw exactly what [\beta] draws — the same three glyphs —
// and not merely stop being reported.
func TestExpandafterWrittenInTheFormulaDrawsWhatItShould(t *testing.T) {
	const pre = `\def\tmpA{\beta}\def\joinc#1{[#1]}`
	got := mathGlyphPaths(t, pre, `\expandafter\joinc\tmpA`)
	want := mathGlyphPaths(t, pre, `[\beta]`)
	if got != want {
		t.Errorf("drew %d glyph path(s), the equivalent formula draws %d", got, want)
	}
}

// The corpus shape: the second token is a CHARACTER, whose one-step expansion is itself,
// so the macro reads it as its argument.
func TestExpandafterBeforeACharacterDrawsWhatItShould(t *testing.T) {
	const pre = `\newcommand\minusone[1]{10^{-1}#1}`
	got := mathGlyphPaths(t, pre, `\expandafter\minusone 2`)
	want := mathGlyphPaths(t, pre, `\minusone 2`)
	if got != want {
		t.Errorf("drew %d glyph path(s), the equivalent formula draws %d", got, want)
	}
}

// The fix is not about \expandafter: it runs the gullet on a top-level source for any
// expandable primitive the engine defines, so \number and \the are served by the same
// line. Each is checked against the literal it must produce.
//
// The counters are named without an @: \makeatother falls before the formula in this
// harness, so a \c@x written in the MATHS reads as \c followed by @x — which is how the
// first version of this test failed, blaming the engine for its own preamble.
func TestOtherExpandablePrimitivesAreServedTheSameWay(t *testing.T) {
	for _, c := range []struct{ name, pre, math, same string }{
		{"number", `\newcount\cntx \cntx=3 `, `y^{\number\cntx}`, `y^{3}`},
		{"the", `\newcount\cnty \cnty=7 `, `y_{\the\cnty}`, `y_{7}`},
		{"romannumeral", ``, `y^{\romannumeral 4}`, `y^{iv}`},
	} {
		got, want := mathGlyphPaths(t, c.pre, c.math), mathGlyphPaths(t, c.pre, c.same)
		if got != want {
			t.Errorf("%s: drew %d glyph path(s), %s draws %d", c.name, got, c.same, want)
		}
	}
}

// ⛔ The flattener runs only when the formula has ALREADY failed, so a formula that
// renders today cannot change. The witness is a source the maths layer handles itself and
// the gullet would mangle: \label is a macro here (classkernel.go), and expanding it hands
// the maths layer our own internals.
func TestAFormulaThatAlreadyRendersIsNotFlattened(t *testing.T) {
	if n := mathGlyphPaths(t, ``, `E = mc^2`); n == 0 {
		t.Error("a plain formula drew nothing")
	}
}

// ⛔ An \ifx whose branches are BOTH empty draws nothing, and nothing is what a wrong
// reading draws too. 2608.07200 is the corpus case: an author mark written as
//
//	^{\ifx\@fnmark\@empty\else\unskip\sep\@fnmark\let\sep=,\fi …}
//
// which this PR takes from "equation dropped" to "empty superscript" — 2 drops, 0 extra
// glyph paths. That is the shape the glyph channel cannot settle on its own, so the
// witness is a controlled one: the SAME conditional with a non-empty mark must draw it.
func TestATopLevelIfxTakesTheRightBranchAndNotJustAnEmptyOne(t *testing.T) {
	const math = `x^{\ifx\gotexmark\gotexnone\else\gotexmark\fi}`
	empty := mathGlyphPaths(t, `\def\gotexmark{}\def\gotexnone{}`, math)
	marked := mathGlyphPaths(t, `\def\gotexmark{2}\def\gotexnone{}`, math)
	// The DIFFERENCE is the claim: the page number is drawn either way, so an absolute
	// count measures the furniture as well as the formula.
	if marked-empty != 1 {
		t.Errorf("the mark added %d glyph path(s), want 1 (empty %d, marked %d)",
			marked-empty, empty, marked)
	}
}
