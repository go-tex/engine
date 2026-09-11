// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A placeholder standing in for a figure must occupy the space the figure asked
// for — no more.
//
// frameNode grows its inner box by sep+rule on every side, so a placeholder built
// at the requested size took 2*(3+0.4) = 6.8pt MORE than the figure would, in both
// directions. Measured against tectonic on one corpus figure (1242.96x409.92pt at
// width=\textwidth=345pt): the reference sets it 113.8pt tall and this reserved
// 120.5. A width=\textwidth placeholder was also wider than the text block, which
// overfills the line it sits on.
func TestAFigurePlaceholderKeepsTheFiguresSize(t *testing.T) {
	e := &Engine{}
	const w, h = 200 * unity, 100 * unity
	e.placeholderImage(w, h, 0, 0, 0, 0, 0, "absent.pdf")
	var fr frameNode
	for _, n := range e.parList {
		if f, ok := n.(frameNode); ok {
			fr = f
		}
	}
	if fr.inner == nil {
		t.Fatal("no placeholder was placed")
	}
	if got := fr.width(); got != w {
		t.Errorf("outer width %d, want the requested %d (over by %.2fpt)",
			got, w, float64(got-w)/unity)
	}
	if got := fr.height() + fr.depth(); got != h {
		t.Errorf("outer height+depth %d, want the requested %d (over by %.2fpt)",
			got, h, float64(got-h)/unity)
	}
}
