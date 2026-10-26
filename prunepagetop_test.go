// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ prune_page_top DELETES EVERY glue, kern and penalty before the first box — not
// one of them, and not only the one the break landed on, which is all this engine did.
//
// tex.web:18879-18881 flushes each of them in a loop; 18888-18893 then puts a
// \splittopskip above the first box, reduced by that box's height. Both halves are
// asserted here, and each is mutated separately below.
func TestPrunePageTopDeletesEveryDiscardableAndAddsSplittopskip(t *testing.T) {
	e := New()
	list := []node{
		glueNode{spec: glueSpec{width: ptToSP(12)}},
		penaltyNode{penalty: 150},
		kernNode{width: ptToSP(3)},
		glueNode{spec: glueSpec{width: ptToSP(6)}},
		&boxNode{height: ptToSP(7), depth: ptToSP(2)},
		glueNode{spec: glueSpec{width: ptToSP(12)}},
		&boxNode{height: ptToSP(7), depth: ptToSP(2)},
	}
	got := e.prunePageTop(list)

	// Four discardables gone, a \splittopskip in their place: 7 − 4 + 1 = 4 nodes.
	if len(got) != 4 {
		t.Fatalf("%d nœud(s) après élagage, want 4: %#v", len(got), got)
	}
	g, ok := got[0].(glueNode)
	if !ok {
		t.Fatalf("premier nœud %T, want la glue de \\splittopskip", got[0])
	}
	// \splittopskip 10pt above a 7pt-tall box: 10 − 7 = 3pt (tex.web:18891).
	if g.spec.width != ptToSP(3) {
		t.Errorf("\\splittopskip = %d, want %d (10pt − 7pt de hauteur)", g.spec.width, ptToSP(3))
	}
	if _, isBox := got[1].(*boxNode); !isBox {
		t.Errorf("deuxième nœud %T, want la première boîte", got[1])
	}
	// ⛔ And NOTHING after that first box is touched: the glue between the two boxes
	// must still be there. An over-eager prune that swept the whole list would delete
	// the interline glue of every line of the column.
	if _, isGlue := got[2].(glueNode); !isGlue {
		t.Errorf("troisième nœud %T, want la glue entre les deux boîtes — "+
			"l'élagage ne doit PAS dépasser la première boîte", got[2])
	}
}

// ⛔ A whatsit and an insertion are KEPT and stepped over (tex.web:18877), never
// deleted. A `\special` dropped at a column break is a lost hyperlink or a lost
// destination, and a dropped footnote is a lost note — neither is vertical material.
func TestPrunePageTopKeepsWhatsitsAndInsertions(t *testing.T) {
	e := New()
	list := []node{
		glueNode{spec: glueSpec{width: ptToSP(12)}},
		specialNode{},
		footnoteNode{body: &boxNode{height: ptToSP(9)}},
		glueNode{spec: glueSpec{width: ptToSP(6)}},
		&boxNode{height: ptToSP(7)},
	}
	got := e.prunePageTop(list)
	if len(got) != 4 { // special, footnote, \splittopskip, box
		t.Fatalf("%d nœud(s), want 4: %#v", len(got), got)
	}
	if _, ok := got[0].(specialNode); !ok {
		t.Errorf("got[0] = %T, want le specialNode conservé", got[0])
	}
	if _, ok := got[1].(footnoteNode); !ok {
		t.Errorf("got[1] = %T, want le footnoteNode conservé", got[1])
	}
	if _, ok := got[2].(glueNode); !ok {
		t.Errorf("got[2] = %T, want la glue de \\splittopskip devant la boîte", got[2])
	}
}

// A list with no box or rule at all gets no glue: there is nothing for it to sit
// above. TeX returns link(temp_head) after the whole walk and inserts nothing.
func TestPrunePageTopAddsNoGlueWithoutABox(t *testing.T) {
	e := New()
	got := e.prunePageTop([]node{
		glueNode{spec: glueSpec{width: ptToSP(12)}},
		penaltyNode{penalty: 0},
		specialNode{},
	})
	if len(got) != 1 {
		t.Fatalf("%d nœud(s), want 1 (le seul specialNode): %#v", len(got), got)
	}
	if _, ok := got[0].(specialNode); !ok {
		t.Errorf("got[0] = %T, want le specialNode", got[0])
	}
}

// ⛔ \splittopskip is 10pt in a fresh engine, because ltplain.dtx:920-921 sets
// \topskip=10pt and \splittopskip=10pt on the next line, so every LaTeX document has
// both. The default belongs at the ALLOCATION (loadTeXParams), not in prunePageTop:
// a fallback there would be unreachable, and to fire at all it would have to read a
// register holding 0 as "absent", overriding a document's deliberate
// \splittopskip=0pt. That is the mistake topskip.go already made once.
func TestAFreshEnginesSplittopskipIsTenPoints(t *testing.T) {
	e := New()
	if got := e.namedSkip("splittopskip").width; got != 10*unity {
		t.Errorf("\\splittopskip = %d, want %d (ltplain.dtx:921)", got, 10*unity)
	}
	// And a document that sets it to zero gets zero, glue node and all.
	e.setNamedSkip("splittopskip", glueSpec{})
	got := e.prunePageTop([]node{
		glueNode{spec: glueSpec{width: ptToSP(12)}},
		&boxNode{height: ptToSP(7)},
	})
	g, ok := got[0].(glueNode)
	if !ok || g.spec.width != 0 {
		t.Errorf("got[0] = %#v, want une glue de 0pt — un 0pt délibéré doit rester 0pt", got[0])
	}
}

// ⛔⛔ THE SECOND HALF, ON ITS OWN: \splitmaxdepth, not \maxdepth, and the depth of a
// piece's last box is not charged against the target.
//
// vsplit passes split_max_depth to vert_break (tex.web:19059), and vert_break keeps
// cur_height and prev_dp apart exactly as the page builder keeps page_total and
// page_depth (tex.web:18959-18968, 19001). Same fixture shape as
// TestPageDepthIsNotChargedAgainstVsize, so the two halves are read the same way:
// three boxes 10pt tall and 2pt deep with zero-width glue between, target 34pt.
func TestSplitVListUsesSplitmaxdepthNotMaxdepth(t *testing.T) {
	list := func() []node {
		var l []node
		for i := 0; i < 3; i++ {
			l = append(l, &boxNode{height: ptToSP(10), depth: ptToSP(2)})
			l = append(l, glueNode{})
		}
		return l
	}

	e := New()
	e.setNamedDimen("splitmaxdepth", ptToSP(5))
	e.setNamedDimen("maxdepth", 0) // the page builder's cap must be IRRELEVANT here
	top, _ := e.splitVList(list(), ptToSP(34))
	if got := countBoxes(top); got != 3 {
		t.Errorf("%d boîte(s) dans la tranche, want 3: cur_height vaut 34pt pile, "+
			"et vert_break accepte l'égalité", got)
	}

	// ⛔ The control, and it is the half that proves the register is the right one:
	// with \splitmaxdepth at zero the same list splits one box earlier, while
	// \maxdepth is left at a generous 5pt and must change nothing.
	e.setNamedDimen("splitmaxdepth", 0)
	e.setNamedDimen("maxdepth", ptToSP(5))
	if got := countBoxes(mustFirst(e.splitVList(list(), ptToSP(34)))); got != 2 {
		t.Errorf("avec \\splitmaxdepth=0pt: %d boîte(s), want 2 — "+
			"et \\maxdepth=5pt ne doit pas sauver la troisième", got)
	}
}

func mustFirst(a, _ []node) []node { return a }
