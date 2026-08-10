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
