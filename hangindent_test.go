// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ \hangindent WAS DECLARED, SETTABLE, AND DROPPED. The engine cached which
// \dimen register it landed on (engine.go:1205, because classkernel.go rebinds
// the name after texparams.go declares it) purely to REPORT the drop — the
// census at engine.go:1297 and droppedshape_test.go exist for that. What it
// could not do is act on it, because the line breaker took one measure for the
// whole paragraph.
//
// tex.web §847-850 recomputes line_width per line from hang_indent and
// hang_after, and go-typeset/linebreak v1.10.0 grew the Measure to express it.
//
// ⭐⭐ THE WITNESS IS THAT THE PARAGRAPH BREAKS DIFFERENTLY, not merely that it
// is placed differently. That is the whole reason a shape cannot be applied
// afterwards: four ems less room on every line but the first moves the words.
func TestHangIndentChangesTheBreaksAndNotOnlyThePlacement(t *testing.T) {
	const body = `\hsize=120pt \parindent=0pt
Typography affects every line of every paper, and a measure that is wrong by a centimetre is wrong on every line it sets.\par`

	// ⛔ THE BOX WIDTH IS THE WRONG OBSERVABLE, and the first version of this
	// test used it and reported "the shape is not reaching the breaker" when it
	// was. A positive \hangindent keeps the line box at the full measure with a
	// rigid kern INSIDE it — that is how the geometry is built here, since a line
	// box carries no shift_amount — so every box is hsize wide either way. What
	// moves is the TEXT PER LINE, which is what re-breaking means.
	lineText := func(t *testing.T, decl string) []string {
		t.Helper()
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		src := `\documentclass{article}\begin{document}` + decl + body + `\end{document}`
		if _, err := e.Run(src); err != nil {
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

	flat := lineText(t, ``)
	hung := lineText(t, `\hangindent=24pt \hangafter=1 `)

	t.Logf("uniform: %d line(s)", len(flat))
	for i, s := range flat {
		t.Logf("  %d %q", i+1, s)
	}
	t.Logf("hung:    %d line(s)", len(hung))
	for i, s := range hung {
		t.Logf("  %d %q", i+1, s)
	}

	if len(flat) == 0 {
		t.Fatal("the uniform control produced no line: the witness is broken")
	}
	if len(hung) < 2 {
		t.Fatalf("the hung paragraph came out as %d line(s); it needs at least two for \\hangafter=1 to mean anything", len(hung))
	}
	// ⭐ The signature of a shape reaching the BREAKER: the words move. Either a
	// different number of lines, or different text on them.
	if len(flat) == len(hung) && sameStrings(flat, hung) {
		t.Error("the same words are on the same lines with and without \\hangindent=24pt: " +
			"the shape is not reaching the line breaker (tex.web §847-850)")
	}
}

// ⛔ ZERO IS NOT A SHAPE. \hangindent=0pt asks for no hanging indent, and
// tex.web:16689 says hang_after is then irrelevant — so the paragraph must come
// out exactly as one that never mentioned either, or every document pays for a
// per-line measure it did not ask for.
func TestZeroHangIndentIsTheUniformParagraph(t *testing.T) {
	const body = `\hsize=120pt \parindent=0pt
Typography affects every line of every paper, and a measure that is wrong by a centimetre is wrong on every line.\par`

	widths := func(t *testing.T, decl string) []int {
		t.Helper()
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		if _, err := e.Run(`\documentclass{article}\begin{document}` + decl + body + `\end{document}`); err != nil {
			t.Fatalf("Run: %v", err)
		}
		var out []int
		for _, n := range e.mvl {
			if b, ok := n.(*boxNode); ok {
				out = append(out, b.width)
			}
		}
		return out
	}

	bare := widths(t, ``)
	zero := widths(t, `\hangindent=0pt \hangafter=3 `)
	if len(bare) != len(zero) || !sameInts(bare, zero) {
		t.Errorf("\\hangindent=0pt changed the paragraph: %v against %v", zero, bare)
	}
}

func sameInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
