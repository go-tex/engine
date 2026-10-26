// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ THE DEPTH OF THE LAST BOX ON A PAGE IS NOT CHARGED AGAINST \vsize.
//
// tex.web keeps page_total and page_depth apart (19226-19229). The badness that
// decides the break compares page_total alone (19594-19599); the depth of the last
// box enters page_total only when the NEXT box or glue arrives (19519, 19550). So a
// page whose material reaches EXACTLY \vsize is accepted, descender and all.
//
// The fixture is built so the two readings disagree by one box and nothing else.
// Three boxes 10pt tall and 2pt deep, with a zero-width glue breakpoint after each
// — which is what interline glue is — and \topskip 0 so the arithmetic is the
// boxes':
//
//	page_total  after box1 10, glue 12, box2 22, glue 24, box3 34, glue 36
//	height+depth            12,     12,     24,     24,     36,     36
//
// With \vsize 34pt the first reading fits all three boxes and the second stops
// after two. 34 is deliberately the EXACT total: equality is the case TeX accepts
// and the case a `>` written as `>=` would refuse, and it is the case the corpus
// kept hitting (go-tex/engine#622).
func TestPageDepthIsNotChargedAgainstVsize(t *testing.T) {
	list := func() []node {
		var l []node
		for i := 0; i < 3; i++ {
			l = append(l, &boxNode{height: ptToSP(10), depth: ptToSP(2)})
			l = append(l, glueNode{})
		}
		return l
	}

	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", ptToSP(5))
	e.vsize = ptToSP(34)
	got := e.findPageBreak(list(), 0)
	if got != 5 {
		t.Errorf("coupe à %d, want 5 (les trois boîtes): page_total vaut 34pt pile, "+
			"soit \\vsize — c'est l'égalité que TeX accepte (tex.web:19594-19599)", got)
	}

	// ⛔ The control, and it is the whole value of this test. \maxdepth=0pt is the
	// cap INITEX leaves behind, and under it every descender is charged at once —
	// which is what this engine did everywhere before. The same list must then break
	// one box earlier. Without this half, the assertion above would also pass for a
	// breaker that simply never charges a depth at all.
	e.setNamedDimen("maxdepth", 0)
	if earlier := e.findPageBreak(list(), 0); earlier != 3 {
		t.Errorf("avec \\maxdepth=0pt la coupe est à %d, want 3 (deux boîtes): "+
			"le plafond à zéro doit facturer chaque descendante immédiatement", earlier)
	}
}

// ⛔ The excess over \maxdepth IS charged (tex.web:19525-19529): a box deeper than
// the cap gives the page back only \maxdepth of its depth, not all of it.
//
// One box 10pt tall and 8pt deep against \maxdepth 5pt: page_total must become
// 10 + (8−5) = 13 and page_depth 5. A reading that forgot the excess would leave
// page_total at 10 and hand the page 3pt that the box occupies.
func TestDepthBeyondMaxdepthIsCharged(t *testing.T) {
	cases := []struct {
		depth, maxDepth, wantTotal, wantDepth int
	}{
		{ptToSP(8), ptToSP(5), ptToSP(13), ptToSP(5)}, // deeper than the cap
		{ptToSP(2), ptToSP(5), ptToSP(10), ptToSP(2)}, // within it
		{ptToSP(2), 0, ptToSP(12), 0},                 // no cap at all
		// A negative cap is legal (tex.web:18944-18946). Written as −ptToSP(3) and
		// not ptToSP(-3): the helper is int(pt*unity+0.5), which truncates toward
		// zero, so ptToSP(-3) is 1sp short of three points and the expected total
		// misses by that sp. The first draft of this row did exactly that.
		{ptToSP(2), -ptToSP(3), ptToSP(15), -ptToSP(3)},
	}
	for _, c := range cases {
		var p pageSoFar
		p.add(&boxNode{height: ptToSP(10), depth: c.depth}, c.maxDepth)
		if p.total != c.wantTotal || p.depth != c.wantDepth {
			t.Errorf("profondeur %d, \\maxdepth %d: page_total=%d page_depth=%d, "+
				"want %d et %d", c.depth, c.maxDepth, p.total, p.depth,
				c.wantTotal, c.wantDepth)
		}
	}
}

// Glue and kern charge the pending depth and then zero it (tex.web:19550), which is
// why the saving is the LAST box's depth and not every box's: between two lines
// there is interline glue, and it hands the previous descender to page_total.
func TestGlueChargesThePendingDepthAndZeroesIt(t *testing.T) {
	for _, n := range []node{glueNode{spec: glueSpec{width: ptToSP(3)}}, kernNode{width: ptToSP(3)}} {
		p := pageSoFar{total: ptToSP(10), depth: ptToSP(2)}
		p.add(n, ptToSP(5))
		if p.total != ptToSP(15) || p.depth != 0 {
			t.Errorf("%T: page_total=%d page_depth=%d, want %d et 0",
				n, p.total, p.depth, ptToSP(15))
		}
	}
	// A penalty moves neither value: TeX contributes nothing for it.
	p := pageSoFar{total: ptToSP(10), depth: ptToSP(2)}
	p.add(penaltyNode{penalty: 100}, ptToSP(5))
	if p.total != ptToSP(10) || p.depth != ptToSP(2) {
		t.Errorf("pénalité: page_total=%d page_depth=%d, want %d et %d",
			p.total, p.depth, ptToSP(10), ptToSP(2))
	}
}

// vContribution is still the natural size of the material — height AND depth —
// because that is what a \vbox of it measures. Only the page breaker separates the
// two. This pins the two readings together so a later edit cannot quietly make
// vContribution mean page_total.
func TestVContributionIsHeightPlusDepth(t *testing.T) {
	b := &boxNode{height: ptToSP(10), depth: ptToSP(2)}
	if got := vContribution(b); got != ptToSP(12) {
		t.Errorf("vContribution = %d, want %d", got, ptToSP(12))
	}
	h, d, kind := vHeightDepth(b)
	if h != ptToSP(10) || d != ptToSP(2) || kind != vBoxLike {
		t.Errorf("vHeightDepth = (%d, %d, %v), want (%d, %d, vBoxLike)",
			h, d, kind, ptToSP(10), ptToSP(2))
	}
}

// And the whole pager must show it: Pages() puts the three boxes on ONE page where
// before it needed two. findPageBreak is only half the path — assemblePage and
// trimTrailingGlue follow it, and a page the breaker accepted can still be split or
// dropped further down.
func TestPagesFitsTheExactlyFullPage(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	e.setNamedDimen("maxdepth", ptToSP(5))
	e.vsize = ptToSP(34)
	for i := 0; i < 3; i++ {
		e.mvl = append(e.mvl, &boxNode{height: ptToSP(10), depth: ptToSP(2)})
		e.mvl = append(e.mvl, glueNode{})
	}
	if got := len(e.Pages()); got != 1 {
		t.Errorf("%d page(s), want 1: les trois lignes tiennent sur une page de 34pt", got)
	}
}
