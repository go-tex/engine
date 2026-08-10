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

// A p{width} column line-breaks its cell into a fixed-width vbox: the column width
// is the declared width (not the widest cell) and the content wraps onto several
// lines. Each word "aaaaa" is 25pt, wider than half of 30pt, so the three words
// stack into three lines.
func TestTabularParColumn(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{}) // each letter 5pt, space 3pt
	e.Run(`\begin{tabular}{p{30pt}}aaaaa aaaaa aaaaa\end{tabular}`)
	tb := lastVbox(e)
	row0 := tb.list[0].(*boxNode) // the single row: [tabcolsep, packedCell, tabcolsep]
	// width = 30pt column + 2·\tabcolsep = 30 + 12 = 42pt
	if row0.width != 42*unity {
		t.Errorf("p-column row width %d sp want 42pt", row0.width)
	}
	packed := row0.list[1].(*boxNode) // the cell packed to the column width (hbox)
	if packed.width != 30*unity {
		t.Errorf("p-column packed width %d sp want 30pt", packed.width)
	}
	cell, ok := packed.list[0].(*boxNode) // the paragraph vbox inside
	if !ok || cell.kind != vbox {
		t.Fatalf("p-cell should be a vbox, got %T", packed.list[0])
	}
	lines := 0
	for _, n := range cell.list {
		if _, ok := n.(*boxNode); ok {
			lines++
		}
	}
	if lines != 3 {
		t.Errorf("p-cell wrapped into %d lines, want 3", lines)
	}
}

// \cline{from-to} draws a partial rule: a left kern up to the start of column
// `from`, then a rule spanning columns from..to (each with its 2·\tabcolsep). With
// columns [20,30,40]pt and no vertical rules, \cline{2-3} kerns past column 1
// (2·6 + 20 = 32pt) and spans columns 2 and 3 (2·(2·6) + 30 + 40 = 94pt).
func TestClineGeometry(t *testing.T) {
	colw := []int{20 * unity, 30 * unity, 40 * unity}
	noRule := func(int) bool { return false }
	n := clineRule(2, 3, 3, colw, noRule)
	box, ok := n.(*boxNode)
	if !ok {
		t.Fatalf("clineRule returned %T, want *boxNode", n)
	}
	kern, ok := box.list[0].(kernNode)
	if !ok || kern.width != 32*unity {
		t.Errorf("cline left kern = %v, want 32pt", box.list[0])
	}
	rule, ok := box.list[1].(ruleNode)
	if !ok || rule.width != 94*unity {
		t.Errorf("cline rule width = %v, want 94pt", box.list[1])
	}
	// A malformed / out-of-range range renders nothing.
	if r := clineRule(0, 0, 3, colw, noRule); r != nil {
		t.Errorf("empty cline range should render nil, got %T", r)
	}
	if r := clineRule(3, 2, 3, colw, noRule); r != nil {
		t.Errorf("reversed cline range should render nil, got %T", r)
	}
}

// parseCline turns \cline's "from-to" text into 1-based column indices.
func TestParseCline(t *testing.T) {
	cases := []struct {
		in       string
		from, to int
	}{
		{"2-3", 2, 3},
		{"1-1", 1, 1},
		{" 2 - 4 ", 2, 4},
		{"nonsense", 0, 0},
		{"5", 0, 0},
	}
	for _, c := range cases {
		if f, to := parseCline(c.in); f != c.from || to != c.to {
			t.Errorf("parseCline(%q) = (%d,%d), want (%d,%d)", c.in, f, to, c.from, c.to)
		}
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
