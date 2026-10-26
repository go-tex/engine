package engine

import "testing"

// \vsplit takes the top part of a vbox up to the given height and leaves the rest.
func TestVsplit(t *testing.T) {
	e := New()
	// box1 = five 10pt rules stacked (no interline glue inside a vbox) = 50pt
	e.Run(`\setbox1=\vbox{\hrule height10pt\hrule height10pt\hrule height10pt\hrule height10pt\hrule height10pt}`)
	if b := e.box[1]; b == nil || b.height != 50*unity {
		t.Fatalf("box1 setup: %+v", e.box[1])
	}
	// split off the top 25pt: two rules fit (20pt), the third would overflow
	e.Run(`\setbox0=\vsplit1 to 25pt`)
	top := e.box[0]
	if top == nil || top.height != 25*unity {
		t.Errorf("top height %v want 25pt (packed to)", top)
	}
	// two rules taken ⇒ remainder is three rules = 30pt
	rest := e.box[1]
	if rest == nil || rest.height != 30*unity {
		t.Errorf("remainder height %v want 30pt", rest)
	}
	// the top box holds exactly two rules
	nrules := 0
	for _, n := range top.list {
		if _, ok := n.(ruleNode); ok {
			nrules++
		}
	}
	if nrules != 2 {
		t.Errorf("top has %d rules want 2", nrules)
	}
}

// ⛔⛔ \vsplit PRUNES ITS REMAINDER, AND THIS GOES THROUGH \vsplit TO SAY SO.
//
// tex.web:19062 calls prune_page_top on the remainder, after vert_break has chosen
// the break. prunepagetop_test.go covers the function; it does not cover the CALL,
// and taking the call out survived the whole suite — the "same guard at one of two
// sites" defect this repository keeps meeting. TestVsplit above could not see it
// either: its list alternates rule and glue, so the remainder always begins with a
// rule, the \splittopskip above a 10pt rule is 10−10 = 0pt, and the height is
// unchanged whether the pruning happens or not.
//
// So this fixture is built to make both halves visible:
//
//   - the remainder BEGINS WITH A PENALTY, which needs two discardables in a row
//     (\vskip then \penalty): the break falls on the glue and discards it, leaving
//     the penalty at the head. Only pruning removes that.
//   - \splittopskip is 30pt, so the glue above the remainder's 10pt rule is 20pt and
//     not 0: the remainder's HEIGHT moves from 10pt to 30pt, which no comment can
//     explain away.
func TestVsplitPrunesTheRemainder(t *testing.T) {
	e := New()
	e.Run(`\splittopskip=30pt`)
	e.Run(`\setbox1=\vbox{\hrule height10pt \vskip20pt \penalty100 \hrule height10pt}`)
	if b := e.box[1]; b == nil || b.height != 40*unity {
		t.Fatalf("box1 setup: %+v (want 40pt: 10 + 20 + 10)", e.box[1])
	}
	e.Run(`\setbox0=\vsplit1 to 25pt`)

	rest := e.box[1]
	if rest == nil {
		t.Fatal("le reste est vide")
	}
	// 30pt − 10pt of rule = 20pt of \splittopskip, then the 10pt rule.
	if rest.height != 30*unity {
		t.Errorf("hauteur du reste %d, want %d (20pt de \\splittopskip + 10pt de filet) — "+
			"10pt signifie que le reste n'a PAS été élagué", rest.height, 30*unity)
	}
	for _, n := range rest.list {
		if _, isPen := n.(penaltyNode); isPen {
			t.Errorf("le reste porte encore une pénalité: prune_page_top supprime " +
				"tout glue, kern et pénalité avant la première boîte (tex.web:18879-18881)")
		}
	}
	if len(rest.list) == 0 {
		t.Fatal("le reste n'a aucun nœud")
	}
	g, ok := rest.list[0].(glueNode)
	if !ok {
		t.Fatalf("le reste commence par %T, want la glue de \\splittopskip", rest.list[0])
	}
	if g.spec.width != 20*unity {
		t.Errorf("\\splittopskip = %d, want %d (30pt − 10pt de hauteur de filet)",
			g.spec.width, 20*unity)
	}
}
