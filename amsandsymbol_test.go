// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// amsmath's \And is an ampersand set as a RELATION between two thick spaces:
//
//	amsmath.sty:403  \def\And{\DOTSB\;\mathchar"3026 \;}
//
// "3026 is class 3 (relation), family 0, slot 0x26 = 38 = '&'. Three things the source
// gave that guessing would not:
//
//   - expanding the real definition only trades \And for an unknown \mathchar, so the
//     ampersand has to be written \& — which go-tex/math does know;
//   - a bare & is the maths layer's COLUMN SEPARATOR, so it cannot be emitted raw;
//   - the class is a RELATION, hence \mathrel, and the two \; are part of the symbol
//     rather than decoration. \DOTSB only marks a \dots boundary and carries no glyph.
//
// The test is an EQUIVALENCE against the definition written out, not a width: the
// width would pin today's font, while "\And is \;\mathrel{\&}\;" is what amsmath says.
//
// 10 equations on one corpus paper, which writes 1$\And$2.
func TestAndIsAnAmpersandRelationBetweenThickSpaces(t *testing.T) {
	got := mathGeom2(t, ``, `$1\And2$`)
	want := mathGeom2(t, ``, `$1\;\mathrel{\&}\;2$`)
	if got.width != want.width || got.height != want.height || got.depth != want.depth {
		t.Errorf(`$1\And2$ = %d/%d/%d, $1\;\mathrel{\&}\;2$ gives %d/%d/%d`,
			got.width, got.height, got.depth, want.width, want.height, want.depth)
	}
	// And the spaces are not optional: without them it is a different symbol. This is
	// what made \And depend on the maths-spacing fix — before it, the \; in the
	// substrate's own body were flattened away and \And came out 1478201 wide.
	bare := mathGeom2(t, ``, `$1\mathrel{\&}2$`)
	if got.width == bare.width {
		t.Errorf(`$1\And2$ is %d wide, the same as without its thick spaces — `+
			`the \; were lost`, got.width)
	}
}

// \symbol{N} is the character at code N, and the reference definition is used verbatim:
//
//	latex.ltx:10001  \DeclareRobustCommand\symbol[1]{\char#1\relax}
//
// Worth checking BEFORE writing a rewrite: \mathchar is unknown to go-tex/math and
// \char is not, so latex.ltx's own definition works as it stands. I had started
// designing a Go-side table mapping each code to a maths command, which would have
// been a second thing to keep in step with the kernel for no gain.
//
// 6 equations on one corpus paper, which writes \symbol{92} eleven times.
func TestSymbolIsTheCharacterAtThatCode(t *testing.T) {
	for _, c := range []struct{ got, want string }{
		// The corpus case: 92 is the backslash.
		{`$\symbol{92}$`, `$\char92\relax$`},
		{`$\symbol{92}$`, `$\backslash$`},
		{`$x\symbol{92}y$`, `$x\char92\relax y$`},
		// An ordinary letter, so the test is not only about one special character.
		{`$\symbol{65}$`, `$\char65\relax$`},
	} {
		got, want := mathGeom2(t, ``, c.got), mathGeom2(t, ``, c.want)
		if got.width != want.width || got.height != want.height || got.depth != want.depth {
			t.Errorf("%s = %d/%d/%d, %s gives %d/%d/%d", c.got, got.width, got.height,
				got.depth, c.want, want.width, want.height, want.depth)
		}
	}
}
