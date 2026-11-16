// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ AN 11pt CLASS DOES NOT SET ITS BODY AT 11pt, AND THAT HELD IN TWO PLACES
// WHILE ONLY ONE WAS FIXED.
//
// size11.clo and scrsize11pt.clo:76 both state \normalsize as \@xipt, and
// \@xipt is 10.95 (latex.ltx:7858). #717 corrected classBaseSizes["11pt"] to
// 1095 and left setPtsize's KOMA DEFAULT at 1100 — the same number, the same
// reason, four lines apart, because the fix was applied to a table rather than
// to the quantity.
//
// Every size of a class's ladder is a permille of this base, so 1100 multiplied
// the whole ladder by 11/10.95 = 1.00457. Whole-point sizes hid it exactly
// (int(11*14.4/10.95 + 0.5) is 14, which is what the right anchor gives too),
// which is why it survived until sizes became fractional.
//
// This asserts the two agree, and asserts it on the QUANTITY rather than on
// either copy of it: a future third place to set a base size has to match here
// or this goes red.
func TestAnElevenPointClassIsTenPointNineFiveEverywhere(t *testing.T) {
	std, ok := classBaseSizes["11pt"]
	if !ok {
		t.Fatal(`classBaseSizes has no "11pt" entry: this test is measuring nothing`)
	}
	if std.permille != 1095 {
		t.Errorf(`classBaseSizes["11pt"].permille = %d, want 1095 — \@xipt is 10.95`, std.permille)
	}

	// ⛔ READ THROUGH setPtsize ITSELF, never through a copy of its rule. The
	// first draft of this test reimplemented that choice here and asserted
	// against its own copy — which would have stayed green with 1100 put back in
	// the code, since the judge and the subject would each have had their own
	// number. What is read below is the base point size the engine ACTUALLY ends
	// up with.
	for _, tc := range []struct {
		name string
		koma bool
		opts []string
		want float64
	}{
		{"koma default (no size option)", true, nil, 10.95},
		{"koma with an explicit 11pt", true, []string{"11pt"}, 10.95},
		{"koma with an explicit 10pt", true, []string{"10pt"}, 10.0},
		{"standard default", false, nil, 10.0},
		{"standard with 11pt", false, []string{"11pt"}, 10.95},
		{"standard with 12pt", false, []string{"12pt"}, 12.0},
	} {
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("%s: buildEngine: %v", tc.name, err)
		}
		if e.baseFontPt == 0 {
			t.Fatalf("%s: the engine has no base face, so setPtsize scales nothing "+
				"and this case measures NOTHING", tc.name)
		}
		e.komaClass = tc.koma
		e.setPtsize(tc.opts)
		if got := e.baseFontPt; !nearly(got, tc.want) {
			t.Errorf("%s: base face is %.4fpt, want %.4fpt", tc.name, got, tc.want)
		}
	}
}

// nearly compares two point sizes to a thousandth of a point, which is finer
// than any rounding in the size ladder and coarser than float noise.
func nearly(a, b float64) bool {
	d := a - b
	return d < 0.001 && d > -0.001
}
