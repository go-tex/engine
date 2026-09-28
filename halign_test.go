package engine

import "testing"

// A 2x2 \halign: column widths are the max cell width, every cell is repacked to
// its column width, and the rows stack into a vbox.
func TestHalignColumnWidths(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // each letter 5pt
	// col0: "A"(5) vs "CCC"(15) ⇒ 15pt ; col1: "BB"(10) vs "D"(5) ⇒ 10pt
	if _, err := e.Run(`\halign{#&#\cr A&BB\cr CCC&D\cr}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(e.mvl) == 0 {
		t.Fatal("no alignment contributed")
	}
	align, ok := e.mvl[len(e.mvl)-1].(*boxNode)
	if !ok || align.kind != vbox {
		t.Fatalf("expected a vbox alignment, got %+v", e.mvl[len(e.mvl)-1])
	}
	// two row hboxes, each width 15+10 = 25pt
	rows := 0
	for _, n := range align.list {
		row, ok := n.(*boxNode)
		if !ok || row.kind != hbox {
			continue
		}
		rows++
		if row.width != 25*unity {
			t.Errorf("row width %d sp want %d", row.width, 25*unity)
		}
		// first cell packed to column-0 width 15pt
		if c0, ok := row.list[0].(*boxNode); ok && c0.width != 15*unity {
			t.Errorf("col0 cell width %d sp want 15pt", c0.width)
		}
	}
	if rows != 2 {
		t.Errorf("expected 2 rows, got %d", rows)
	}
}

// Template text (before/after #) is included in every cell.
func TestHalignTemplateText(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	// template "x#" prepends an 'x' (5pt) to each col-0 entry: "xA" = 10pt
	e.Run(`\halign{x#\cr A\cr}`)
	align := e.mvl[len(e.mvl)-1].(*boxNode)
	row := align.list[0].(*boxNode)
	if row.width != 10*unity {
		t.Errorf("templated cell width %d sp want 10pt (x+A)", row.width)
	}
}

// \noalign puts vertical material between the rows of an alignment.
//
// tex.web §785: it may appear only where a row could begin — at the start of an
// alignment or just after a \cr — and its argument is vertical-mode material
// that joins the enclosing vertical list. TikZ stacks the lines of a node with
// align= exactly this way, one \halign per node whose \noalign{\vskip …} is the
// leading between the lines, so before this the second line of
// \node[align=center]{one \\ two} was not set at all.
func TestHalignNoalignContributesBetweenRows(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\halign{#\cr A\cr\noalign{\vskip 10pt}B\cr}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	align, ok := e.mvl[len(e.mvl)-1].(*boxNode)
	if !ok || align.kind != vbox {
		t.Fatalf("expected a vbox alignment, got %+v", e.mvl[len(e.mvl)-1])
	}
	var rows int
	var found bool
	for i, n := range align.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			rows++
			continue
		}
		g, ok := n.(glueNode)
		if !ok || g.spec.width != 10*unity {
			continue
		}
		// It belongs BETWEEN the rows: a row before it and a row after it.
		before, after := false, false
		for j := 0; j < i; j++ {
			if b, ok := align.list[j].(*boxNode); ok && b.kind == hbox {
				before = true
			}
		}
		for j := i + 1; j < len(align.list); j++ {
			if b, ok := align.list[j].(*boxNode); ok && b.kind == hbox {
				after = true
			}
		}
		if before && after {
			found = true
		}
	}
	if rows != 2 {
		t.Errorf("expected 2 rows, got %d", rows)
	}
	if !found {
		t.Errorf("no 10pt skip between the rows; \\noalign contributed nothing")
	}
}

// \noalign at the very start of an alignment is legal too, and it must not be
// mistaken for a row: an alignment opening with one still has exactly its own
// rows.
func TestHalignNoalignBeforeTheFirstRow(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\halign{#\cr\noalign{\vskip 3pt}A\cr B\cr}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	align, ok := e.mvl[len(e.mvl)-1].(*boxNode)
	if !ok || align.kind != vbox {
		t.Fatalf("expected a vbox alignment, got %+v", e.mvl[len(e.mvl)-1])
	}
	rows := 0
	for _, n := range align.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			rows++
		}
	}
	if rows != 2 {
		t.Errorf("expected 2 rows, got %d", rows)
	}
}
