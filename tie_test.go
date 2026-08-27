package engine

import "testing"

// The active ~ is LaTeX's interword tie (\nobreakspace): an UNBREAKABLE interword
// space that appears everywhere in real documents — "Section~\ref", "Figure~2",
// "et~al.", "Theorem~1". It must typeset the SAME glue as an ordinary space (so
// the words do not jam), plus an infinite break penalty so no line break falls on
// it. The engine dispatched no meaning for the active char, so ~ produced nothing
// at all and "Section~1" set as "Section1".

// tieNodes runs `\setbox0=\hbox{<inner>}` and returns the box's node list.
func tieBoxList(t *testing.T, inner string) []node {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{` + inner + `}`); err != nil {
		t.Fatalf("run %q: %v", inner, err)
	}
	if e.box[0] == nil {
		t.Fatalf("box0 nil for %q", inner)
	}
	return e.box[0].list
}

func countGlueAndPenalty(list []node) (glues, glueW, penalties, penaltyVal int) {
	for _, n := range list {
		switch v := n.(type) {
		case glueNode:
			glues++
			glueW = int(v.spec.width)
		case penaltyNode:
			penalties++
			penaltyVal = v.penalty
		}
	}
	return
}

// A tie sets the same interword glue as a normal space: "Section~1" measures
// exactly as wide as "Section 1", and carries one space glue plus a nobreak
// penalty (so the previously-dropped space is present).
func TestActiveTieProducesInterwordGlue(t *testing.T) {
	spaceList := tieBoxList(t, `Section 1`)
	tieList := tieBoxList(t, `Section~1`)

	sg, sw, sp, _ := countGlueAndPenalty(spaceList)
	tg, tw, tp, tv := countGlueAndPenalty(tieList)

	if sg != 1 {
		t.Fatalf("normal space: got %d glues, want 1", sg)
	}
	if tg != 1 || tw != sw {
		t.Errorf("tie glue = %d nodes of width %d sp, want 1 of width %d (same as a normal space)", tg, tw, sw)
	}
	if sp != 0 {
		t.Errorf("normal space carried %d penalties, want 0", sp)
	}
	if tp != 1 || tv != 10000 {
		t.Errorf("tie penalties = %d with value %d, want 1 with value 10000 (\\nobreak)", tp, tv)
	}

	// "Section" = 7 letters × 5pt = 35pt, space = 3pt, "1" = one digit × 4pt.
	// Both boxes must measure identically: the tie is NOT a zero-width jam.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.Run(`\setbox0=\hbox{Section 1}\setbox1=\hbox{Section~1}`)
	if e.box[0].width != e.box[1].width {
		t.Errorf("tie box width %d sp != space box width %d sp", e.box[1].width, e.box[0].width)
	}
	if e.box[0].width != 42*unity {
		t.Errorf("box width %d sp, want 42pt (35 + 3 + 4)", e.box[0].width)
	}
}

// The tie's nobreak penalty forbids a line break between the two words: where a
// normal interword space lets the paragraph wrap into two lines, the tie keeps
// both words on one (overfull) line — the single legal breakpoint is suppressed.
func TestActiveTieDoesNotBreakLine(t *testing.T) {
	countLines := func(joiner string) int {
		e := New()
		e.SetFont(spMock{}) // letters 5pt, space 3pt; no patterns ⇒ no in-word breaks
		e.hsize = 40 * unity
		e.parindent = 0
		e.baselineskip = 10 * unity
		// Two 7-letter words (35pt each); joined natural width 35+3+35 = 73pt > 40pt.
		if _, err := e.Run(`aaaaaaa` + joiner + `bbbbbbb\par`); err != nil {
			t.Fatalf("run %q: %v", joiner, err)
		}
		n := 0
		for _, node := range e.mvl {
			if b, ok := node.(*boxNode); ok && b.kind == hbox {
				n++
			}
		}
		return n
	}
	if got := countLines(` `); got != 2 {
		t.Fatalf("normal space: %d lines, want 2 (the space is a legal breakpoint)", got)
	}
	if got := countLines(`~`); got != 1 {
		t.Errorf("tie ~: %d lines, want 1 (the nobreak penalty forbids the only break)", got)
	}
}

// ~ is protected (LaTeX declares \nobreakspace robust): it does not expand inside
// a moving argument, where an active ~ prints as the literal character.
func TestActiveTieIsProtectedInMessage(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	got, err := e.Run(`\message{[a~b]}`)
	if err != nil {
		t.Fatal(err)
	}
	if trimNL(got) != "[a~b]" {
		t.Errorf("\\message{[a~b]} = %q, want %q (protected ~ prints literally)", trimNL(got), "[a~b]")
	}
}

// A document may still redefine the active ~, now that it is a nameable target:
// \def~{...} and \let~=... take effect through the same active-char slot.
func TestActiveTieIsRedefinable(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	got, err := e.Run(`\def~{TIE}\message{[a~b]}`)
	if err != nil {
		t.Fatal(err)
	}
	if trimNL(got) != "[aTIEb]" {
		t.Errorf(`\def~{TIE} then \message = %q, want "[aTIEb]"`, trimNL(got))
	}
}

// A definition primitive that runs off the end of input names nothing: scanCSName
// sees no next token and returns "" without panicking (regression guard for the
// active-char branch split).
func TestScanCSNameAtEOF(t *testing.T) {
	e := New()
	if _, err := e.Run(`\let`); err != nil {
		t.Fatalf("\\let at EOF: %v", err)
	}
}
