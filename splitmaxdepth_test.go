// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ \splitmaxdepth AND \boxmaxdepth ARE \maxdimen, NOT ZERO. ltplain.dtx sets both
// on consecutive lines and LaTeX inherits them:
//
//	\splitmaxdepth=\maxdimen        ltplain.dtx:872
//	\boxmaxdepth=\maxdimen          ltplain.dtx:873
//
// Measured against tectonic from inside both engines on the same document:
// \splittopskip 10.0pt and \maxdepth 5.0pt agree, and these two read 0.0pt here
// against 16383.99998pt there.
func TestSplitAndBoxMaxDepthDefaultToMaxdimen(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\message{[\the\splitmaxdepth|\the\boxmaxdepth]}`); err != nil {
		t.Fatal(err)
	}
	const want = "[16383.99998pt|16383.99998pt]"
	if got := trimNL(e.out.String()); got != want {
		t.Errorf("got %q want %q (ltplain.dtx:872-873)", got, want)
	}
}

// ⛔⛔ AND IT IS THE #622 DEFECT, UNFIXED IN THE \vsplit PATH.
//
// \splitmaxdepth is the cap vert_break uses for prev_dp — tex.web:19059 passes it as
// vert_break's d argument — exactly as \maxdepth caps page_depth for the page
// builder. At zero, every descender is charged to the running height the moment the
// next box arrives, so a column that fills EXACTLY gives up its last line. That is
// what #622 fixed for the page builder, and nobody carried it to \vsplit; multicols
// slices through the same function.
//
// The fixture is the one from TestPageDepthIsNotChargedAgainstVsize, pointed at
// splitVList: three boxes 10pt tall and 2pt deep with a zero glue after each, split
// to 34pt — which is 10 + 12 + 12, the exact total.
func TestSplitKeepsTheLineThatFillsTheColumnExactly(t *testing.T) {
	list := func() []node {
		var l []node
		for i := 0; i < 3; i++ {
			l = append(l, &boxNode{height: ptToSP(10), depth: ptToSP(2)})
			l = append(l, glueNode{})
		}
		return l
	}
	e := New()
	e.setNamedDimen("splitmaxdepth", maxDimenSP)
	top, _ := e.splitVList(list(), ptToSP(34))
	if len(top) != 5 {
		t.Errorf("la colonne garde %d nœuds, want 5 (les trois boîtes): 34pt pile, "+
			"et la profondeur de la dernière n'est pas facturée (tex.web:18967, 19059)",
			len(top))
	}

	// ⛔ THE CONTROL, and it is the whole value of this test: with the cap at zero —
	// which is what this engine had — the same column stops one box earlier. Without
	// this half the assertion above would also pass for a splitter that never charges
	// a depth at all.
	e.setNamedDimen("splitmaxdepth", 0)
	if top, _ := e.splitVList(list(), ptToSP(34)); len(top) != 4 {
		t.Errorf("avec \\splitmaxdepth=0 la colonne garde %d nœuds, want 4 (deux "+
			"boîtes): le plafond à zéro facture chaque descendante immédiatement",
			len(top))
	}
}
