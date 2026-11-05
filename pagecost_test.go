// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ THE BREAKPOINT COST IS A THREE-WAY CHOICE, NOT A SUM (tex.web:19556-19560):
//
//	if pi<=eject_penalty then c:=pi
//	else if b<inf_bad then c:=b+pi+insert_penalties
//	else c:=deplorable
//
// The middle line is the one this engine had, and it was applied unconditionally. So
// a \penalty-300 sitting on a page that is INFINITELY bad — badness 10000, a page
// nowhere near full — scored 9700, which beat an honest later break at 10000. TeX
// refuses that trade: once b reaches inf_bad every breakpoint on the page costs the
// same `deplorable` 100000 and the penalties stop mattering, because no penalty is
// worth shipping a nearly-empty page.
//
// The fixture is the trade itself and nothing else: two breakpoints, both on an
// infinitely bad page, the EARLIER one carrying the negative penalty.
//
//	index  0 A(10pt)  1 \penalty-300  2 glue  3 B(10pt)  4 glue  5 huge(1000pt)
//
// \vsize 100pt with no stretch anywhere, so badness is inf_bad at both: 10000 at
// index 1 and 10000 at index 4. Index 2 is not a legal break — glue after a penalty
// never is (tex.web:19496, precedes_break).
func TestANegativePenaltyDoesNotBuyAnInfinitelyBadPage(t *testing.T) {
	list := func(pen int) []node {
		return []node{
			&boxNode{height: ptToSP(10)},
			penaltyNode{penalty: pen},
			glueNode{},
			&boxNode{height: ptToSP(10)},
			glueNode{},
			&boxNode{height: ptToSP(1000)},
		}
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list(-300), 0); got != 4 {
		t.Errorf("coupe à %d, want 4: sur une page infiniment mauvaise, "+
			"\\penalty-300 coûte `deplorable` comme toute autre rupture "+
			"(tex.web:19559) — la somme 10000-300 le faisait gagner à l'indice 1", got)
	}

	// ⛔ THE CONTROL. Without the penalty the answer is already 4, so the assertion
	// above only means something next to this one: what the fix changes is the
	// ARBITRAGE, not the breaker's taste for later breaks.
	if got := e.findPageBreak(list(0), 0); got != 4 {
		t.Errorf("témoin sans pénalité: coupe à %d, want 4", got)
	}
}

// ⭐ AND THE OTHER HALF: while the page is merely bad, the penalty IS summed in, and
// a negative one does move the break. A fix that simply stopped adding penalties
// would pass the test above and fail this one.
//
//	index  0 glue(0pt plus 90pt)  1 A(10pt)  2 \penalty-300  3 glue
//	       4 B(10pt)  5 glue  6 huge(1000pt)
//
// The leading glue is not a breakpoint (it is the first node), and it puts 90pt of
// stretch on the page so both candidates are finite:
//
//	index 2: shortfall 90pt over 90pt → badness 100   → cost 100-300 = -200
//	index 5: shortfall 80pt over 90pt → badness  70   → cost 70
func TestAPenaltyStillCountsWhileThePageIsMerelyBad(t *testing.T) {
	list := func(pen int) []node {
		return []node{
			glueNode{spec: glueSpec{stretch: ptToSP(90)}},
			&boxNode{height: ptToSP(10)},
			penaltyNode{penalty: pen},
			glueNode{},
			&boxNode{height: ptToSP(10)},
			glueNode{},
			&boxNode{height: ptToSP(1000)},
		}
	}
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", 0)
	e.vsize = ptToSP(100)

	if got := e.findPageBreak(list(-300), 0); got != 2 {
		t.Errorf("coupe à %d, want 2: badness 100 moins 300 bat les 70 de l'indice 5", got)
	}
	if got := e.findPageBreak(list(0), 0); got != 5 {
		t.Errorf("témoin sans pénalité: coupe à %d, want 5 (70 bat 100)", got)
	}
}

// The two badness readings the fixture above depends on, pinned so that a change to
// badness() shows up here as itself rather than as a page that moved.
func TestTheFixtureBadnessValues(t *testing.T) {
	if got := badness(ptToSP(90), ptToSP(90)); got != 100 {
		t.Errorf("badness(90pt,90pt) = %d, want 100", got)
	}
	if got := badness(ptToSP(80), ptToSP(90)); got != 70 {
		t.Errorf("badness(80pt,90pt) = %d, want 70", got)
	}
	if got := badness(ptToSP(90), 0); got != infBad {
		t.Errorf("badness(90pt,0) = %d, want inf_bad %d", got, infBad)
	}
}
