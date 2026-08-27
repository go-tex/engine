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

// \multicolumn{n}{spec}{content} spans n columns: its block occupies exactly the
// space n ordinary cells would, so every row keeps the same width, and its content
// is aligned per its own spec.
func TestMulticolumn(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{tabular}{lll}\multicolumn{2}{c}{AB} & C \\ D & E & F\end{tabular}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	var rows []*boxNode
	for _, n := range tb.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			rows = append(rows, b)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].width != rows[1].width {
		t.Errorf("row widths differ (%d vs %d) — a multicolumn must align with normal rows", rows[0].width, rows[1].width)
	}
	if got := mvlText(e.mvl); got != "ABCDEF" {
		t.Errorf("text = %q, want ABCDEF", got)
	}
}

// Regression: a \multicolumn on a row that follows another row must be recognised —
// the newline after the previous row's \\ lands in the cell as a leading space.
func TestMulticolumnAfterRow(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{tabular}{lll}\na & b & c \\\\\n\\multicolumn{2}{c}{S} & d \\\\\n\\end{tabular}"
	if _, err := e.Run(src); err != nil {
		t.Fatalf("multicolumn after a normal row must not error, got: %v", err)
	}
}

// The \multicolumn argument parsers: span count and a single-column spec's
// alignment and | borders.
func TestMulticolumnParse(t *testing.T) {
	if n := toksToInt([]tok{{ch: '1', cat: catOther}, {ch: '2', cat: catOther}}); n != 12 {
		t.Errorf("toksToInt = %d, want 12", n)
	}
	spec := []tok{{ch: '|', cat: catOther}, {ch: 'c', cat: catOther}, {ch: '|', cat: catOther}}
	if a, lv, rv := parseColSpecToks(spec); a != 'c' || !lv || !rv {
		t.Errorf("parseColSpecToks(|c|) = %c,%v,%v, want c,true,true", a, lv, rv)
	}
	if a, lv, rv := parseColSpecToks([]tok{{ch: 'r', cat: catOther}}); a != 'r' || lv || rv {
		t.Errorf("parseColSpecToks(r) = %c,%v,%v, want r,false,false", a, lv, rv)
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

// A conditional at a tabular body's top level is evaluated as the body is scanned
// (as TeX's alignment scanner does), so only the taken branch's rows and text —
// including any \\ it carries — reach the row/cell splitter. Here \iffalse drops
// its row and \iftrue keeps "B"; the text after the box survives.
func TestTabularConditionalEvaluated(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	if _, err := e.Run(
		`\hbox{\begin{tabular}{l}` +
			`\iffalse A\\ \fi` +
			`\iftrue B\\ \fi` +
			`\end{tabular}}AFTER\par`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "BAFTER" {
		t.Errorf("conditional tabular text = %q want %q (only the taken branch should appear)", got, "BAFTER")
	}
}

// A conditional nested inside a cell's {…} group stays raw and is evaluated when
// the cell is typeset, so it must not be consumed by the body scanner. Both cells
// render their taken branch.
func TestTabularConditionalInsideBraces(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	if _, err := e.Run(
		`\begin{tabular}{ll}{\iftrue P\fi} & {\iffalse Q\fi}\\ ` +
			`{\iffalse R\fi} & {\iftrue S\fi}\end{tabular}ZZ\par`); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	for _, want := range []string{"P", "S", "ZZ"} {
		if !contains(got, want) {
			t.Errorf("tabular text %q missing %q", got, want)
		}
	}
	if contains(got, "Q") || contains(got, "R") {
		t.Errorf("tabular text %q kept a false branch", got)
	}
}

// A tabular's optional [pos] vertical-placement group (\begin{tabular}[t]{c}) must
// be consumed before the column spec. Regression: scanColSpec met the '[' where it
// expected '{', returned a zero-column spec, and collectTabularBody then dropped the
// whole body — silently erasing, among other things, the \maketitle author block
// (LaTeX's \@maketitle wraps \@author in \begin{tabular}[t]{c}…\end{tabular}). Every
// placement letter must render its cells.
func TestTabularOptionalPlacement(t *testing.T) {
	for _, pos := range []string{"[t]", "[b]", "[c]", "[ t ]"} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.hsize = 500 * unity
		if _, err := e.Run(
			`\begin{tabular}` + pos + `{cc}` +
				`CELLA & CELLB\\ ROWA & ROWB\end{tabular}AFTER\par`); err != nil {
			t.Fatalf("pos %q: %v", pos, err)
		}
		got := mvlText(e.mvl)
		for _, want := range []string{"CELLA", "CELLB", "ROWA", "ROWB", "AFTER"} {
			if !contains(got, want) {
				t.Errorf("tabular%s text %q missing %q (placement arg dropped the body)", pos, got, want)
			}
		}
	}
}

// tabularx accepts the same optional [pos] before its {width}: \begin{tabularx}[t]{W}{spec}.
func TestTabularxOptionalPlacement(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	if _, err := e.Run(
		`\begin{tabularx}[t]{200pt}{X}` +
			`CELLX\end{tabularx}AFTER\par`); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	for _, want := range []string{"CELLX", "AFTER"} {
		if !contains(got, want) {
			t.Errorf("tabularx[t] text %q missing %q", got, want)
		}
	}
}
