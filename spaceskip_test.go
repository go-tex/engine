// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ \spaceskip REPLACES THE FONT'S SPACE; \xspaceskip IS TAKEN VERBATIM.
//
// tex.web:20358-20370 (app_space) and :20332-20338 (the ordinary space). Two
// rules that look alike and are not:
//
//   - \xspaceskip is used WHOLE and unscaled, and only when the space factor is
//     at least 2000;
//   - \spaceskip becomes the BASE and is then scaled by the factor exactly as
//     the font's space would have been, with \fontdimen7 added at ≥ 2000 (§1044
//     adds extra_space whatever the base is).
//
// Both were accept-and-ignore here (#826): declared as registers, read by
// nothing, so a document that set either was handed the font's own space in
// silence. That silence cost an hour today — eighteen line-breaking fixtures
// that set \spaceskip measured the mock font's 3pt plus 1.5 and said
// \looseness had no effect.
func TestSpaceSkipReplacesTheFontsSpace(t *testing.T) {
	// spMock's own space is 3pt plus 1.5pt minus 1pt (charnode_test.go), which is
	// what every row below must be distinguishable from.
	font := spMock{}

	for _, c := range []struct {
		name                string
		setup               string
		factor              int
		wantW, wantY, wantZ int
	}{
		{
			// The control: nothing set, the font's own space, unscaled.
			name: "font space when nothing is set", factor: 1000,
			wantW: 3 * unity, wantY: 3 * unity / 2, wantZ: unity,
		},
		{
			// ⛔ The row the defect was: the document's glue, not the font's.
			name: "spaceskip is the base", setup: `\spaceskip=10pt plus 4pt minus 2pt`,
			factor: 1000,
			wantW:  10 * unity, wantY: 4 * unity, wantZ: 2 * unity,
		},
		{
			// ⛔ ZERO IS UNSET, as :20328-20330 has it — the pointer test. A
			// \spaceskip of nothing falls back to the font rather than giving a
			// zero-width space.
			name: "a zero spaceskip is not a zero-width space", setup: `\spaceskip=0pt`,
			factor: 1000,
			wantW:  3 * unity, wantY: 3 * unity / 2, wantZ: unity,
		},
		{
			// ⛔ And a width of zero with stretch IS set: `0pt plus 2pt` is a space
			// that can only grow, and a "width > 0" test would throw it away.
			name: "stretch alone makes it set", setup: `\spaceskip=0pt plus 2pt`,
			factor: 1000,
			wantW:  0, wantY: 2 * unity, wantZ: 0,
		},
		{
			// \xspaceskip is ignored below 2000, however set.
			name:   "xspaceskip is ignored below 2000",
			setup:  `\xspaceskip=20pt plus 5pt \spaceskip=10pt plus 4pt minus 2pt`,
			factor: 1000,
			wantW:  10 * unity, wantY: 4 * unity, wantZ: 2 * unity,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(font)
			if c.setup != "" {
				if _, err := e.Run(c.setup); err != nil {
					t.Fatal(err)
				}
			}
			e.spaceFactor = c.factor
			g := e.spaceGlueForFactor(font)
			if g.width != c.wantW || g.stretch != c.wantY || g.shrink != c.wantZ {
				t.Errorf("glue = %d/%d/%d, want %d/%d/%d",
					g.width, g.stretch, g.shrink, c.wantW, c.wantY, c.wantZ)
			}
		})
	}
}

// ⛔ AT A FACTOR OF 2000 THE TWO RULES SEPARATE, and this is where a half
// implementation would have been worse than none: \xspaceskip is taken whole,
// while \spaceskip is a base that the factor still scales.
func TestAtTwoThousandTheTwoSkipsPartCompany(t *testing.T) {
	font := spMock{}

	// \xspaceskip set: taken verbatim. No \fontdimen7, no scaling — the stretch
	// stays 5pt where scaling by 2000/1000 would have doubled it to 10pt.
	e := New()
	e.SetFont(font)
	if _, err := e.Run(`\xspaceskip=20pt plus 5pt minus 1pt \spaceskip=10pt plus 4pt`); err != nil {
		t.Fatal(err)
	}
	e.spaceFactor = 2000
	if g := e.spaceGlueForFactor(font); g.width != 20*unity || g.stretch != 5*unity || g.shrink != unity {
		t.Errorf("with \\xspaceskip set at factor 2000: glue = %d/%d/%d, want %d/%d/%d "+
			"— tex.web:20361-20362 takes it whole and unscaled",
			g.width, g.stretch, g.shrink, 20*unity, 5*unity, unity)
	}

	// ⛔ THE CONTROL: the same factor with NO \xspaceskip takes \spaceskip as the
	// base and scales it. Without this row the test above would pass for an
	// implementation that always returned \xspaceskip, or never scaled anything.
	e2 := New()
	e2.SetFont(font)
	if _, err := e2.Run(`\spaceskip=10pt plus 4pt minus 2pt`); err != nil {
		t.Fatal(err)
	}
	e2.spaceFactor = 2000
	g := e2.spaceGlueForFactor(font)
	if g.stretch != 8*unity {
		t.Errorf("stretch = %d, want %d: the factor doubles \\spaceskip's stretch "+
			"as it would the font's (§1044)", g.stretch, 8*unity)
	}
	if g.shrink != unity {
		t.Errorf("shrink = %d, want %d: 1000/2000 halves it", g.shrink, unity)
	}
	if g.width != 10*unity+e2.extraSpaceOf(font) {
		t.Errorf("width = %d, want %d: \\fontdimen7 is added at 2000 whatever the "+
			"base is", g.width, 10*unity+e2.extraSpaceOf(font))
	}
}
