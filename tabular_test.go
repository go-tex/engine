package engine

import "testing"

func lastVbox(e *Engine) *boxNode {
	for i := len(e.mvl) - 1; i >= 0; i-- {
		if b, ok := e.mvl[i].(*boxNode); ok && b.kind == vbox {
			return b
		}
	}
	return nil
}

// A 2x2 tabular computes column widths from the widest cell and stacks two rows.
func TestTabular(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{}) // each letter 5pt
	// col0: a(5) vs ccc(15) ⇒ 15 ; col1: bb(10) vs d(5) ⇒ 10
	if _, err := e.Run(`\begin{tabular}{ll}a & bb \\ ccc & d\end{tabular}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no tabular vbox contributed")
	}
	rows := 0
	for _, n := range tb.list {
		row, ok := n.(*boxNode)
		if !ok || row.kind != hbox {
			continue
		}
		rows++
		// width = col0(15)+col1(10) + 2·\tabcolsep per column (4×6pt) = 49pt
		if row.width != 49*unity {
			t.Errorf("row width %d sp want 49pt", row.width)
		}
	}
	if rows != 2 {
		t.Errorf("rows=%d want 2", rows)
	}
}

// Right alignment puts the fil before the content so the cell is flush right.
func TestTabularRightAlign(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.Run(`\begin{tabular}{r}a \\ ccc\end{tabular}`)
	tb := lastVbox(e)
	row0 := tb.list[0].(*boxNode)   // first row hbox: [tabcolsep, cell, tabcolsep]
	cell := row0.list[1].(*boxNode) // the cell (after the leading \tabcolsep kern)
	if _, isGlue := cell.list[0].(glueNode); !isGlue {
		t.Errorf("right-aligned cell should lead with fil glue, got %T", cell.list[0])
	}
}
