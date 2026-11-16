// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// lineTextOf returns the characters set on each line box of a compiled document,
// which is the observable that shows a paragraph being BROKEN differently rather
// than merely placed differently.
//
// ⚠ The line box's WIDTH is the wrong observable and cost an earlier witness an
// afternoon: a positive indent keeps the box at the full measure with a rigid
// kern inside it, so every box is hsize wide either way.
func lineTextOf(t *testing.T, body string) []string {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}` + body + `\end{document}`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var out []string
	for _, n := range e.mvl {
		b, ok := n.(*boxNode)
		if !ok {
			continue
		}
		var s []rune
		for _, c := range b.list {
			if ch, ok := c.(charNode); ok {
				s = append(s, ch.ch)
			}
		}
		if len(s) > 0 {
			out = append(out, string(s))
		}
	}
	return out
}

const parShapeBody = `\hsize=120pt \parindent=0pt
Typography affects every line of every paper, and a measure that is wrong by a centimetre is wrong on every line it sets.\par`

// ⭐ A \parshape reaches the line BREAKER, so the words move. The shape below
// gives line 1 the full measure and every later line 60pt, which is half — the
// last pair repeating is what makes two pairs an answer for a five-line
// paragraph (tex.web:16680-16681).
func TestParShapeBreaksTheParagraphOnItsOwnWidths(t *testing.T) {
	flat := lineTextOf(t, parShapeBody)
	shaped := lineTextOf(t, `\parshape=2 0pt 120pt 0pt 60pt `+parShapeBody)

	t.Logf("uniform: %d line(s)", len(flat))
	for i, s := range flat {
		t.Logf("  %d %q", i+1, s)
	}
	t.Logf("parshape: %d line(s)", len(shaped))
	for i, s := range shaped {
		t.Logf("  %d %q", i+1, s)
	}

	if len(flat) == 0 {
		t.Fatal("the uniform control set no line: the witness is broken")
	}
	if len(flat) == len(shaped) && sameStrings(flat, shaped) {
		t.Error("the same words are on the same lines with and without \\parshape: " +
			"the shape is not reaching the line breaker (tex.web:16706, :16740-16748)")
	}
	// Halving the measure from line 2 on cannot produce FEWER lines.
	if len(shaped) <= len(flat) {
		t.Errorf("\\parshape halved the measure from line 2 and the paragraph came out in "+
			"%d lines against the uniform %d", len(shaped), len(flat))
	}
}

// ⛔ n<=0 TURNS THE SHAPE OFF, and the early return that does it is a PANIC
// GUARD against a document's own integer (tex.web:23276, `if n<=0 then
// p:=null`; here `make([]Shape, 0, n)` panics on a negative capacity).
//
// ⚠ BOTH CASES ARE HERE FOR DIFFERENT REASONS, which a mutation established
// rather than my reading: with the guard removed, `\parshape=0` is unharmed —
// the pair loop is bounded by n and runs zero times either way — while
// `\parshape=-1` PANICS. So `0` checks that the shape is off and the text
// intact, and `-1` checks that a hostile integer does not crash the engine.
// An earlier version of this comment claimed the guard stopped the scanner from
// eating the next two dimensions; it cannot, and saying so was wrong.
func TestParShapeZeroReadsNoPairsAndKeepsTheText(t *testing.T) {
	for _, n := range []string{"0", "-1"} {
		t.Run("parshape="+n, func(t *testing.T) {
			got := lineTextOf(t, `\parshape=`+n+` `+parShapeBody)
			want := lineTextOf(t, parShapeBody)
			if len(got) != len(want) || !sameStrings(got, want) {
				t.Errorf("\\parshape=%s changed the paragraph:\n got  %q\n want %q\n"+
					"n<=0 turns the shape off and scans nothing further", n, got, want)
			}
		})
	}
}

// ⭐ \parshape WINS OVER \hangindent (tex.web:16706), and the two are set
// together here so that reading them in the wrong order is visible: the
// \hangindent asks for 24pt off every line after the first, the \parshape asks
// for 60pt, and the result must be the \parshape's.
func TestParShapeTakesPrecedenceOverHangIndent(t *testing.T) {
	both := lineTextOf(t, `\hangindent=24pt \hangafter=1 \parshape=2 0pt 120pt 0pt 60pt `+parShapeBody)
	shapeOnly := lineTextOf(t, `\parshape=2 0pt 120pt 0pt 60pt `+parShapeBody)
	hangOnly := lineTextOf(t, `\hangindent=24pt \hangafter=1 `+parShapeBody)

	if len(both) != len(shapeOnly) || !sameStrings(both, shapeOnly) {
		t.Errorf("with both set the paragraph does not match the \\parshape alone:\n both  %q\n shape %q",
			both, shapeOnly)
	}
	// The control: the two parameters must actually differ, or the test above
	// passes whatever order the engine reads them in.
	if len(hangOnly) == len(shapeOnly) && sameStrings(hangOnly, shapeOnly) {
		t.Fatal("\\hangindent=24pt and \\parshape 60pt produce the same paragraph, so this " +
			"test cannot tell which one won")
	}
}

// A shape lasts one paragraph, like the other three (tex.web:20760).
func TestParShapeLastsOneParagraph(t *testing.T) {
	const second = `Typography affects every line of every paper, and a measure that is wrong by a centimetre is wrong on every line it sets.\par`
	after := lineTextOf(t, `\hsize=120pt \parindent=0pt \parshape=2 0pt 120pt 0pt 60pt `+
		`First paragraph, which asks for the shape.\par `+second)
	plain := lineTextOf(t, `\hsize=120pt \parindent=0pt `+
		`First paragraph, which asks for nothing.\par `+second)

	tail := func(ls []string, n int) []string {
		if len(ls) < n {
			return ls
		}
		return ls[len(ls)-n:]
	}
	a, b := tail(after, 3), tail(plain, 3)
	if !sameStrings(a, b) {
		t.Errorf("the second paragraph differs depending on whether the FIRST set a \\parshape:\n"+
			" after a shaped one %q\n after a plain one   %q\n"+
			"tex.web:20760 clears par_shape_ptr at every \\par", a, b)
	}
}
