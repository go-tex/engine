// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \put is defined for what its absence costs. Undefined, it leaves its own
// coordinates in the input, and "(-0.33\textwidth," reads as the start of an
// assignment to \textwidth whose missing number TeX takes as zero — which is what
// TeX itself would do, and what left \hsize at 0pt for the remaining 230 pages of
// a real paper, one word per line.
func TestPutDoesNotEatTheTextWidth(t *testing.T) {
	run := func(src string) *Engine {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src); err != nil {
			t.Fatal(err)
		}
		return e
	}
	// The class sets \textwidth itself, so the control is the same document without
	// the \put rather than the engine's default measure.
	before := run(`Text. more text.\par`).hsize
	e := run(`Text. \put(-0.33\textwidth, 0.5\textwidth){M} more text.\par`)
	if e.hsize != before {
		t.Errorf("\\hsize = %d sp after \\put, was %d: the coordinates were read as an assignment",
			e.hsize, before)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "M") || !strings.Contains(txt, "moretext.") {
		t.Errorf("the put object or the text after it is missing: %q", txt)
	}
}

// A coordinate with no unit is in \unitlength, one with a unit keeps its own
// (ltpictur's \@defaultunits). Both forms must leave the surrounding text alone.
func TestPutTakesBareCoordinatesInUnitlength(t *testing.T) {
	run := func(src string) *Engine {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src); err != nil {
			t.Fatal(err)
		}
		return e
	}
	before := run(`Text. after.\par`).hsize
	e := run(`\setlength{\unitlength}{2pt}Text. \put(10,20){M}\multiput(0,0)(5,5){3}{N} after.\par`)
	if e.hsize != before {
		t.Errorf("\\hsize = %d sp, was %d", e.hsize, before)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"M", "N", "after."} {
		if !strings.Contains(txt, want) {
			t.Errorf("output missing %q: %q", want, txt)
		}
	}
}
