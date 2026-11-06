// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A FAMILY SWITCH KEEPS THE SIZE IN FORCE.
//
// \rm, \bf, \it, \tt and \sf change the family; they do not change the size. Each is
// bound to a face built ONCE, at the document's base size (bindOptionalFont), so
// selecting one as bound silently undid any size switch before it.
//
// LaTeX's own \section states its style as {\normalfont\Large\bfseries} — size
// first, series LAST — so every section heading in every document came out at body
// size. Measured against tectonic on a two-word witness, in Libertinus:
//
//	{\Large …}            ours 13.95   reference 14.35
//	{\Large\bfseries …}   ours  9.96   reference 14.35   ← the defect
//	{\large …}            ours 11.96   reference 11.96
//
// and on \section{…} the heading went from 9.96 to 13.95 with its baseline landing
// at 167.70 against the reference's 167.73.
func TestAFamilySwitchKeepsTheSize(t *testing.T) {
	e := New()
	e.SetFont(scaleMock{px: 10})
	e.baseFontPx = 10
	e.bindFont("bf", scaleMock{px: 10}) // bound at the base size, as the CLI binds it
	e.classNormalsizePt = 10

	// \catcode64=11 by its number, not through a backtick: this is a Go raw string.
	if _, err := e.Run(`\catcode64=11 \gotex@fontsizeat{14.4}\bf`); err != nil {
		t.Fatal(err)
	}
	if got := e.curFont.sizePt(); got != 14 {
		t.Errorf("après \\Large puis \\bf la fonte est à %dpt, want 14: un changement "+
			"de FAMILLE ne doit pas défaire un changement de TAILLE — c'est l'ordre "+
			"que \\section emploie ({\\normalfont\\Large\\bfseries})", got)
	}
}

// ⛔ THE CONTROL, and it is what keeps the assertion above about the ORDER: the
// other way round must give the same answer. If only one order worked, the fix
// would be a coincidence of where the scaling happens.
func TestTheOtherOrderGivesTheSameSize(t *testing.T) {
	e := New()
	e.SetFont(scaleMock{px: 10})
	e.baseFontPx = 10
	e.bindFont("bf", scaleMock{px: 10})
	e.classNormalsizePt = 10
	if _, err := e.Run(`\catcode64=11 \bf\gotex@fontsizeat{14.4}`); err != nil {
		t.Fatal(err)
	}
	if got := e.curFont.sizePt(); got != 14 {
		t.Errorf("après \\bf puis \\Large la fonte est à %dpt, want 14", got)
	}
}

// ⛔ AND A FAMILY SWITCH AT THE BODY SIZE CHANGES NOTHING ABOUT THE SIZE. Without
// this half, a fix that always re-faced to e.baseFontPx would pass the two above
// and silently re-anchor a heading written inside a \small environment.
func TestAFamilySwitchAtBodySizeLeavesItAlone(t *testing.T) {
	e := New()
	e.SetFont(scaleMock{px: 10})
	e.baseFontPx = 10
	e.bindFont("bf", scaleMock{px: 10})
	e.classNormalsizePt = 10
	// 8pt body, then bold: the bold must be 8, not the 10 it was bound at.
	if _, err := e.Run(`\catcode64=11 \gotex@fontsizeat{8}\bf`); err != nil {
		t.Fatal(err)
	}
	if got := e.curFont.sizePt(); got != 8 {
		t.Errorf("après \\small puis \\bf la fonte est à %dpt, want 8: la taille "+
			"gardée est celle EN VIGUEUR, pas la taille de base", got)
	}
}
