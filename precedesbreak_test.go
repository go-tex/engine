// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ THE BREAKPOINT TEST AND THE HEIGHT TEST MUST CLASSIFY A NODE THE SAME WAY.
//
// `precedes_break(p) == type(p) < math_node` (tex.web:3109), with `math_node = 9`
// (:3092), so it holds for hlist 0, vlist 1, rule 2, ins 3, mark 4, adjust 5,
// ligature 6, disc 7 and whatsit 8 (:3085). build_page applies it to the last node on
// the page before it will take a glue node as a breakpoint (:19496).
//
// vHeightDepth already decides which of this engine's nodes stand for TeX's box-like
// types — it charges their height and depth to page_total as TeX charges an hlist's.
// The predicate that answered the breakpoint question named four of those twelve. A
// page could therefore be handed an image's full height and then forbidden to break
// after it.
//
// This test walks every node vHeightDepth calls box-like and asserts the breakpoint
// test agrees. It is written as a LOOP OVER THE SOURCE OF TRUTH rather than as a list
// of names on purpose: a thirteenth box-like node added to vHeightDepth tomorrow is
// covered the day it is added, which a second hand-kept list is not.
func TestEveryBoxLikeNodePrecedesABreak(t *testing.T) {
	boxLike := []struct {
		name string
		n    node
	}{
		{"boxNode", &boxNode{height: unity}},
		{"ruleNode", ruleNode{height: unity}},
		{"charNode", charNode{height: unity}},
		{"mathNode", mathNode{height: unity}},
		{"imageNode", imageNode{height: unity}},
		{"footnoteNode", footnoteNode{body: &boxNode{height: unity}}},
	}
	for _, c := range boxLike {
		if _, _, kind := vHeightDepth(c.n); kind != vBoxLike {
			t.Fatalf("%s: le témoin lui-même est mal choisi — vHeightDepth ne le "+
				"dit pas vBoxLike", c.name)
		}
		if !precedesBreak(c.n) {
			t.Errorf("%s: vHeightDepth facture sa hauteur à page_total mais "+
				"precedesBreak refuse la coupe après lui — la page grandit sans "+
				"pouvoir s'arrêter (tex.web:3109, :19496)", c.name)
		}
	}
}

// ⛔ A whatsit is type 8 and 8 < math_node, so a break IS legal after it, and it is
// the one precedes_break case vHeightDepth cannot answer: a \special carries no
// height at all.
func TestAWhatsitPrecedesABreak(t *testing.T) {
	if !precedesBreak(specialNode{}) {
		t.Error("un whatsit est de type 8 et 8 < math_node=9 (tex.web:3085, :3109)")
	}
}

// ⛔ AND THE CONTROL: what must NOT precede a break. Glue, kerns and penalties are
// types 10, 11 and 12, all above math_node — a run of discardables is not a place to
// stop, which is the whole reason glue after a penalty is not a legal breakpoint.
// Without this half, `return true` would pass everything above.
func TestDiscardablesDoNotPrecedeABreak(t *testing.T) {
	for _, n := range []node{glueNode{}, kernNode{}, penaltyNode{}} {
		if precedesBreak(n) {
			t.Errorf("%T est un rebut (type 10-12, au-dessus de math_node): "+
				"aucune coupe ne se prend après lui", n)
		}
	}
}

// ⛔⛔ And the breakpoint the whole change is about, end to end: a glue node after an
// image is a legal page break. Before this, `legalPageBreak` said no — so a page
// holding a figure was charged the figure's height and then had nowhere to stop.
func TestGlueAfterAnImageIsALegalPageBreak(t *testing.T) {
	list := []node{
		&boxNode{height: ptToSP(10)},
		glueNode{},
		imageNode{height: ptToSP(20)},
		glueNode{},
	}
	if ok, _ := legalPageBreak(list, 3, 0); !ok {
		t.Error("la colle qui suit une image doit être une coupe légale")
	}
}
