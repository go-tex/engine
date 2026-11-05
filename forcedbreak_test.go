// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A FORCED BREAK DOES NOT GET TO CHOOSE ON A PAGE THAT IS ALREADY OVERFULL.
//
// tex.web:19553-19560 computes the badness BEFORE it looks at the penalty, and the
// first thing it tests is b:
//
//	if b<awful_bad then
//	  if pi<=eject_penalty then c:=pi
//	  else if b<inf_bad then c:=b+pi+insert_penalties
//	  else c:=deplorable
//	else c:=b;
//
// So when the page's excess already beats its shrink, a \newpage costs awful_bad
// like every other breakpoint on that page. It becomes the champion only if nothing
// better was recorded (:19563 — least_page_cost starts at awful_bad), and fire_up
// then ships best_page_break, the EARLIER champion, leaving the material between
// there and the \newpage for the next page, where the forced penalty is met again.
//
// Returning at the forced break before computing b put all of that overfull material
// on the page the \newpage ends.
//
//	index  0 A(50pt)  1 glue  2 B(60pt)  3 \penalty-10000
//
// \vsize 100pt, no shrink. Index 1 is a legal break on a 50pt page — infinitely bad,
// so it costs `deplorable`, but it is recorded. By index 3 the page holds 110pt
// against a goal of 100 with nothing to shrink: awful_bad. The break is index 1.
func TestAForcedBreakOnAnOverfullPageFiresAtTheChampion(t *testing.T) {
	list := []node{
		&boxNode{height: ptToSP(50)},
		glueNode{},
		&boxNode{height: ptToSP(60)},
		penaltyNode{penalty: -10000},
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list, 0); got != 1 {
		t.Errorf("coupe à %d, want 1: à l'indice 3 la page porte 110pt pour un but "+
			"de 100 sans retrait — awful_bad, et `else c:=b` (tex.web:19560) prive "+
			"la pénalité forcée de son choix", got)
	}
}

// ⛔ THE CONTROL, and it is what keeps the assertion above about awful_bad rather
// than about forced breaks in general: the same list with a page that still FITS at
// the forced break. There c := pi <= eject_penalty, the break wins, and it fires
// where it is written.
func TestAForcedBreakOnAPageThatFitsFiresWhereItIsWritten(t *testing.T) {
	list := []node{
		&boxNode{height: ptToSP(50)},
		glueNode{},
		&boxNode{height: ptToSP(30)}, // 80pt total: under the 100pt goal
		penaltyNode{penalty: -10000},
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list, 0); got != 3 {
		t.Errorf("témoin: coupe à %d, want 3 — la page tient, donc c:=pi et la "+
			"rupture forcée l'emporte (tex.web:19557, :19572)", got)
	}
}

// ⛔ AND WHEN THERE IS NO CHAMPION AT ALL, the forced break on an overfull page is
// still where the page ends: tex.web:19563 records it as champion because
// least_page_cost is still awful_bad, and :19572 fires there. Without this half, a
// fix could return -1 or run to the end of the list and nothing would catch it.
func TestAForcedBreakWithNoChampionFiresAtItself(t *testing.T) {
	list := []node{
		&boxNode{height: ptToSP(200)}, // one box, already over the goal
		penaltyNode{penalty: -10000},
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list, 0); got != 1 {
		t.Errorf("coupe à %d, want 1: aucun champion antérieur, la rupture forcée "+
			"est elle-même le champion", got)
	}
}
