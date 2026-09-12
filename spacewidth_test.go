// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"testing"
)

// The interword space is the font's own advance, UNROUNDED.
//
// Face.Advance rounds to a whole pixel (roundInt(units*scale)), and a space is a
// single glyph whose whole width IS that advance — so the rounding lands entirely
// on the interword space, and differently at every type size. Libertinus Serif
// declares 0.25em and the rounded path gave 2.000 at 9pt (-11%), 3.000 at 10pt
// (+20%), 3.000 at 11pt (+9%), 3.000 at 12pt (exact). With ~13 spaces to a line,
// +20% at 10pt moves every line break.
func TestInterwordSpaceIsNotRoundedToAWholePoint(t *testing.T) {
	const path = "/Users/Shared/gotex/measure/texmf/LibertinusSerif-Regular.otf"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skip("the corpus font tree is not on this machine: ", err)
	}
	for _, px := range []int{9, 10, 11, 12} {
		f, err := NewOpenTypeFont(b, px)
		if err != nil {
			t.Fatal(err)
		}
		w, st, sh := f.Space()
		want := float64(px) * 0.25 // what the face declares
		if diff := w - want; diff > 0.01 || diff < -0.01 {
			t.Errorf("%dpt: space %.3fpt, want %.3fpt (%+.1f%%)", px, w, want, diff/want*100)
		}
		// XeTeX's ratios for an OpenType font, confirmed against tectonic:
		// stretch = space/2, shrink = space/3.
		if d := st - w/2; d > 0.001 || d < -0.001 {
			t.Errorf("%dpt: stretch %.3f, want space/2 = %.3f", px, st, w/2)
		}
		if d := sh - w/3; d > 0.001 || d < -0.001 {
			t.Errorf("%dpt: shrink %.3f, want space/3 = %.3f", px, sh, w/3)
		}
	}
}
