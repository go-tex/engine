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
		// each row width = col0(15) + gap(12) + col1(10) = 37pt
		if row.width != 37*unity {
			t.Errorf("row width %d sp want 37pt", row.width)
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
	row0 := tb.list[0].(*boxNode)   // first row hbox
	cell := row0.list[0].(*boxNode) // its single cell
	if _, isGlue := cell.list[0].(glueNode); !isGlue {
		t.Errorf("right-aligned cell should lead with fil glue, got %T", cell.list[0])
	}
}
