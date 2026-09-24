// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The star is the whole difference between \resizebox and \resizebox*, and it is
// one word of graphics.sty:539-541:
//
//	\protected\def\resizebox{%
//	  \leavevmode
//	  \@ifstar{\Gscale@@box\totalheight}{\Gscale@@box\height}}
//
// \resizebox sizes the box by its HEIGHT above the baseline; \resizebox* by its
// TOTAL height, depth included. Everything else is shared.
//
// Unstarred, the star was not consumed, so neither dimension parsed and the source
// reached the PAGE as text: one corpus paper printed "*45mm!" and "*100mm!" into
// its figures, each wrapping a tikzpicture that then went unscaled. spMock's 'x' is
// 5pt wide, 7pt tall and 2pt deep, so the two forms must disagree by 7 against 9.
func TestResizeboxStarSizesTheTotalHeight(t *testing.T) {
	plain := runTransform(t, `\resizebox{!}{14pt}{x}`)
	if got, want := plain.height(), 14*unity; got != want {
		t.Errorf(`\resizebox height = %d sp, want %d (the height alone, 7pt -> 14pt)`, got, want)
	}
	star := runTransform(t, `\resizebox*{!}{14pt}{x}`)
	if got, want := star.height()+star.depth(), 14*unity; got != want {
		t.Errorf(`\resizebox* total = %d sp, want %d (height+depth, 9pt -> 14pt)`, got, want)
	}
	// And the two must not be the same thing: a star silently ignored would make
	// this test pass on the first check and fail here by 2/7 of the box.
	if plain.height() == star.height() {
		t.Errorf("both forms scaled to height %d sp; the star changed nothing", plain.height())
	}
	// '!' on the width follows the vertical factor, so the widths differ too.
	if plain.width() == star.width() {
		t.Errorf("both forms scaled to width %d sp; the star changed nothing", plain.width())
	}
}

// The star must not leak into the page. This is the defect as a reader met it.
func TestResizeboxStarLeavesNoTextBehind(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\resizebox*{30pt}{!}{x}`); err != nil {
		t.Fatal(err)
	}
	if n := len(e.SkippedCommands()); n != 0 {
		t.Errorf("skipped %v, want none", e.SkippedCommands())
	}
	var chars []rune
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				chars = append(chars, c.ch)
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	for _, r := range chars {
		if r == '*' || r == '!' {
			t.Errorf("the character %q reached the page; the star's arguments leaked", r)
		}
	}
}
