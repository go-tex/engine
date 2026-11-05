// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A PAGE IS FULL WHEN ITS EXCESS BEATS ITS SHRINK, NOT WHEN IT PASSES \vsize.
//
// build_page has no test on page_total against page_goal anywhere. It loops until
// the contribution list is empty (tex.web:19425-19432) and fires the page only when
// a BREAKPOINT costs awful_bad or carries a forced penalty (:19572). awful_bad is
// reached at :19598 — `page_total-page_goal>page_shrink` — so material that the
// page's own glue can tighten away stays on the page.
//
// findPageBreak carried a second, unauthorised firing condition: as soon as
// page_total crossed \vsize, at whatever node happened to do the crossing, it
// returned the champion so far. That discarded the half of badnessOf written for
// this moment, and the shrink it accumulates was computed and never read.
//
// The fixture is one box's worth of difference and nothing else:
//
//	index  0 A(50pt)  1 glue(0pt minus 20pt)  2 B(60pt)  3 glue  4 huge(1000pt)  5 glue
//
// \vsize 100pt, \topskip 0, \maxdepth 0. At index 3 the page holds 110pt: 10pt over
// the goal, against 20pt of shrink, so badness is 12 and the break is legal. At
// index 5 it holds 1110pt, 1010pt over against the same 20pt, which is awful_bad
// and fires.
func TestAPageKeepsWhatItsGlueCanShrinkAway(t *testing.T) {
	list := func(shrink int) []node {
		return []node{
			&boxNode{height: ptToSP(50)},
			glueNode{spec: glueSpec{shrink: shrink}},
			&boxNode{height: ptToSP(60)},
			glueNode{},
			&boxNode{height: ptToSP(1000)},
			glueNode{},
		}
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list(ptToSP(20)), 0); got != 3 {
		t.Errorf("coupe à %d, want 3: 110pt pour un but de 100pt, c'est 10pt "+
			"d'excès contre 20pt de retrait — badness 12, pas awful_bad "+
			"(tex.web:19598)", got)
	}

	// ⛔ THE CONTROL, and it is what makes the assertion above about SHRINK rather
	// than about breaking later in general: take the shrink away and the same list
	// stops at the same place it always did.
	if got := e.findPageBreak(list(0), 0); got != 1 {
		t.Errorf("témoin sans retrait: coupe à %d, want 1 — sans retrait, "+
			"110pt sur 100pt est awful_bad et la page part au champion", got)
	}
}

// The badness the fixture turns on, pinned so a change to badness() shows up as
// itself and not as a page that moved.
func TestTheShrinkFixtureBadness(t *testing.T) {
	if got := badness(ptToSP(10), ptToSP(20)); got != 12 {
		t.Errorf("badness(10pt,20pt) = %d, want 12", got)
	}
}
