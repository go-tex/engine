package engine

import "testing"

// \topskip's natural width is reduced by the height of the page's first box, and
// FLOORED at zero (tex.web §1001). The floor is the half that matters: a first
// line taller than \topskip gets no glue at all, which is how a tall opening box
// sits flush against the top of the text block.
func TestTopskipGlueIsReducedByTheFirstBoxHeightAndFloored(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity, stretch: 3 * unity})
	for _, c := range []struct {
		name   string
		height int
		want   int
	}{
		{"shorter than topskip", ptToSP(2), ptToSP(8)},
		{"exactly topskip", 10 * unity, 0},
		{"taller than topskip", ptToSP(40), 0},
		{"zero height", 0, 10 * unity},
	} {
		g, at, ok := e.topskipGlueFor([]node{&boxNode{height: c.height}})
		if !ok {
			t.Fatalf("%s: aucune boîte trouvée", c.name)
		}
		if at != 0 {
			t.Errorf("%s: insérée à %d, want 0", c.name, at)
		}
		if g.spec.width != c.want {
			t.Errorf("%s: largeur %d, want %d", c.name, g.spec.width, c.want)
		}
		// ⛔ Only the natural width is reduced. The stretch and shrink are what the
		// register states, and a page whose first line is short must still be able
		// to stretch at the top as the reference does.
		if g.spec.stretch != 3*unity {
			t.Errorf("%s: étirement %d, want %d — seule la largeur se réduit",
				c.name, g.spec.stretch, 3*unity)
		}
	}
}

// ⛔ The HEIGHT alone, never height+depth. §1001 reduces the glue by height(p)
// because what is being positioned is the first BASELINE; charging the depth too
// would lift every page's first line by the descender of whatever sits on it.
func TestTopskipIgnoresTheFirstBoxDepth(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity})
	g, _, ok := e.topskipGlueFor([]node{&boxNode{height: ptToSP(2), depth: ptToSP(5)}})
	if !ok {
		t.Fatal("aucune boîte trouvée")
	}
	if g.spec.width != ptToSP(8) {
		t.Errorf("largeur %d, want %d (10−2, la profondeur ne compte pas)", g.spec.width, ptToSP(8))
	}
}

// Glue, kern and penalty before the first box are what TeX discards while
// page_contents is still empty (§1000), so they must not end the search — the
// glue goes before the BOX, not before them.
func TestTopskipSkipsDiscardablesBeforeTheFirstBox(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity})
	page := []node{
		glueNode{spec: glueSpec{width: ptToSP(7)}},
		penaltyNode{penalty: 50},
		kernNode{width: ptToSP(3)},
		&boxNode{height: ptToSP(2)},
	}
	g, at, ok := e.topskipGlueFor(page)
	if !ok {
		t.Fatal("aucune boîte trouvée")
	}
	if at != 3 {
		t.Errorf("insérée à %d, want 3 (devant la boîte, pas devant la glue)", at)
	}
	if g.spec.width != ptToSP(8) {
		t.Errorf("largeur %d, want %d", g.spec.width, ptToSP(8))
	}
	if got := e.withTopskip(page); len(got) != 5 {
		t.Fatalf("la page rendue a %d nœuds, want 5", len(got))
	}
}

// A page with no box at all is handed back unchanged: there is nothing for the
// glue to sit above, and inserting it would give a box-less page a height.
func TestTopskipLeavesABoxlessPageAlone(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity})
	page := []node{glueNode{spec: glueSpec{width: ptToSP(7)}}, penaltyNode{penalty: 0}}
	if _, _, ok := e.topskipGlueFor(page); ok {
		t.Error("une page sans boîte ne doit pas recevoir de \\topskip")
	}
	if got := e.withTopskip(page); len(got) != 2 {
		t.Errorf("la page a été modifiée: %d nœuds, want 2", len(got))
	}
}

// ⛔⛔ THE TWO SITES MUST AGREE. assemblePage puts the glue on the page and
// findPageBreak charges its height; if they computed it differently the breaker
// would choose a break for a page that then overflows by the difference. This is
// the "same guard at one of two sites" defect this repository keeps meeting, so
// it is asserted rather than trusted to a comment.
func TestTopskipHeightAndInsertionAgree(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity})
	list := []node{
		glueNode{spec: glueSpec{width: ptToSP(5)}},
		&boxNode{height: ptToSP(3), depth: ptToSP(1)},
		&boxNode{height: ptToSP(40)},
	}
	for start := 0; start < len(list); start++ {
		charged := e.topskipHeightAt(list, start)
		g, _, ok := e.topskipGlueFor(list[start:])
		inserted := 0
		if ok {
			inserted = g.spec.width
		}
		if charged != inserted {
			t.Errorf("start=%d: le briseur compte %d, l'assemblage pose %d",
				start, charged, inserted)
		}
	}
	// And past the end of the list it charges nothing rather than panicking.
	if got := e.topskipHeightAt(list, len(list)); got != 0 {
		t.Errorf("au-delà de la liste: %d, want 0", got)
	}
}

// A fresh engine's \topskip is 10pt, plain.tex's and size10.clo's value, because
// loadTeXParams gives the register that default when it allocates it.
//
// ⛔ This started life as a test of a FALLBACK inside topskipGlueFor, for the case
// of "no register at all". loadTeXParams always allocates one, so that branch was
// unreachable — an assertion dressed as a guard — and to fire at all it had to
// read a register holding 0 as absent, which would override a document's
// deliberate \topskip=0pt. The default moved to the allocation, where
// \parfillskip's already was, and the fallback is gone.
func TestAFreshEnginesTopskipIsTenPoints(t *testing.T) {
	e := New()
	spec, have := e.topskipSpec()
	if !have {
		t.Fatal("le registre \\topskip doit être alloué par loadTeXParams")
	}
	if spec.width != 10*unity {
		t.Errorf("\\topskip = %d, want %d (plain.tex et size10.clo)", spec.width, 10*unity)
	}
	g, _, ok := e.topskipGlueFor([]node{&boxNode{height: 0}})
	if !ok {
		t.Fatal("aucune boîte trouvée")
	}
	if g.spec.width != 10*unity {
		t.Errorf("glue au-dessus d'une boîte de hauteur nulle = %d, want %d", g.spec.width, 10*unity)
	}
}

// ⛔⛔ And the BREAKER must charge it, which the agreement test above does not
// prove: it compares two functions of this file and never goes through
// findPageBreak. Replacing that call with a plain 0 survived the whole suite, so
// the only thing asserted was that topskip.go agrees with itself.
//
// Here the page is built so that the boxes alone FIT and the boxes plus the glue
// do NOT. Boxes of 2 + 10 + 10 = 22pt with a legal breakpoint after each, a first
// box 2pt tall so the glue is 10−2 = 8pt, and \vsize 25pt: 22 ≤ 25 without the
// charge, 30 > 25 with it.
//
// ⚠ The first version of this test used \vsize 32pt, which both sides fit, so it
// was RED either way — and mutate then reported the mutation as "caught", because
// a test that already fails cannot distinguish anything. An arithmetic slip in a
// fixture reads as coverage.
func TestFindPageBreakChargesTheTopskip(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{width: 10 * unity})
	e.vsize = ptToSP(25)
	list := []node{
		&boxNode{height: ptToSP(2)},
		glueNode{spec: glueSpec{}},
		&boxNode{height: ptToSP(10)},
		glueNode{spec: glueSpec{}},
		&boxNode{height: ptToSP(10)},
		glueNode{spec: glueSpec{}},
	}
	got := e.findPageBreak(list, 0)
	if got >= len(list) {
		t.Fatalf("coupe à %d (la fin): la glue de \\topskip n'est pas comptée, "+
			"la page fera 8pt de plus que ce que le briseur a mesuré", got)
	}
	// The control: with \topskip at zero the same list breaks later, which is what
	// makes the assertion above about the CHARGE and not about this list's shape.
	e.setNamedSkip("topskip", glueSpec{})
	if later := e.findPageBreak(list, 0); later <= got {
		t.Errorf("sans \\topskip la coupe est à %d, elle devrait être plus tardive que %d",
			later, got)
	}
}

// ⛔ A \topskip deliberately set to 0pt must stay 0pt. namedSkip cannot tell a
// register holding zero from a name that is not a register, so reading the zero
// as "absent" gave such a document 10pt — and left a test no way to build a
// control. Found by the control in TestFindPageBreakChargesTheTopskip failing.
func TestAnExplicitZeroTopskipStaysZero(t *testing.T) {
	e := New()
	e.setNamedSkip("topskip", glueSpec{})
	spec, have := e.topskipSpec()
	if !have {
		t.Fatal("le registre existe, topskipSpec dit le contraire")
	}
	if spec.width != 0 {
		t.Errorf("largeur %d, want 0", spec.width)
	}
	g, _, ok := e.topskipGlueFor([]node{&boxNode{height: 0}})
	if !ok {
		t.Fatal("aucune boîte trouvée")
	}
	if g.spec.width != 0 {
		t.Errorf("\\topskip=0pt donne %d, want 0 — le repli a écrasé une valeur voulue", g.spec.width)
	}
}
