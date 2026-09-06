package engine

import "testing"

// runTab runs a tabular snippet with the deterministic spMock font and returns the
// contributed tabular vbox, failing the test on any engine error.
func runTab(t *testing.T, src string) (*Engine, *boxNode) {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{}) // letters 5pt wide, 7pt tall, 2pt deep
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	tb := lastVbox(e)
	if tb == nil {
		t.Fatalf("run %q: no tabular vbox contributed", src)
	}
	return e, tb
}

// dataRows returns the hbox rows of a tabular vbox (skipping \hline/\cline rules).
func dataRows(tb *boxNode) []*boxNode {
	var rows []*boxNode
	for _, n := range tb.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			rows = append(rows, b)
		}
	}
	return rows
}

// smashedSlot returns the \multirow slot of a row: the zero-height, zero-depth
// hbox that still carries content (an empty covered cell has an empty list).
func smashedSlot(row *boxNode) *boxNode {
	for _, n := range row.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox && b.height == 0 && b.depth == 0 && len(b.list) > 0 {
			return b
		}
	}
	return nil
}

// mrboxOf returns the first nested box of a slot — the \multirow content box.
func mrboxOf(slot *boxNode) *boxNode {
	for _, n := range slot.list {
		if b, ok := n.(*boxNode); ok {
			return b
		}
	}
	return nil
}

// A 2-row \multirow{2}{*}{X} centres its content box over the two rows it spans:
// the box is smashed to zero height in its own row and shifted down so the free
// space above and below it is equal, and it still renders its glyphs.
func TestMultirowVerticalCentring(t *testing.T) {
	// col0: X (multirow), col1: a over b. Each letter box is 7pt tall and 2pt deep,
	// but a row is never shorter than \@arstrutbox — .7/.3 of \baselineskip
	// (latex.ltx:12102-12105) — so the row dimensions are read from the strut here
	// rather than written down: the arithmetic below is the same either way.
	e, tb := runTab(t, `\begin{tabular}{ll}\multirow{2}{*}{X} & a \\ & b\end{tabular}`)

	if got := mvlText(e.mvl); got != "Xab" {
		t.Errorf("text = %q, want Xab", got)
	}
	rows := dataRows(tb)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	// The multirow row keeps its ordinary height (the spanning box is smashed):
	// the strut's, since the strut is taller than the 7pt letters.
	strutH, strutD := strutDims(e)
	if rows[0].height != strutH || rows[0].depth != strutD {
		t.Errorf("multirow row = h%d d%d, want the strut h%d d%d (spanning box must not inflate it)",
			rows[0].height, rows[0].depth, strutH, strutD)
	}
	slot := smashedSlot(rows[0])
	if slot == nil {
		t.Fatal("no smashed \\multirow slot in the first row")
	}
	if slot.height != 0 || slot.depth != 0 {
		t.Errorf("slot = h%d d%d, want h0 d0", slot.height, slot.depth)
	}
	box := mrboxOf(slot)
	if box == nil {
		t.Fatal("no content box inside the \\multirow slot")
	}
	// The content box carries its real dimensions (7pt tall, 2pt deep).
	if box.height != 7*unity || box.depth != 2*unity {
		t.Errorf("content box = h%d d%d, want h7pt d2pt", box.height, box.depth)
	}
	// shift = -Hr + V/2 + (Hc-Dc)/2, positive = lowered.
	rowExtent := strutH + strutD
	wantShift := -strutH + rowExtent + (7*unity-2*unity)/2
	if box.shift != wantShift {
		t.Errorf("shift = %d sp, want %d sp (4.5pt)", box.shift, wantShift)
	}
	// Prove centring: the free space above the box top equals that below its bottom.
	Hr, V := rows[0].height, (rows[0].height+rows[0].depth)+(rows[1].height+rows[1].depth)
	blockTop, blockBot := -Hr, -Hr+V
	boxTop, boxBot := box.shift-box.height, box.shift+box.depth
	if (boxTop - blockTop) != (blockBot - boxBot) {
		t.Errorf("not centred: top margin %d != bottom margin %d", boxTop-blockTop, blockBot-boxBot)
	}
}

// An \hline between the two spanned rows adds its rule thickness to the block
// extent, lowering the centred box by half a rule versus the un-ruled case.
func TestMultirowSpansInteriorRule(t *testing.T) {
	e, tb := runTab(t, `\begin{tabular}{ll}\multirow{2}{*}{X} & a \\ \hline & b\end{tabular}`)
	strutH, strutD := strutDims(e)
	rowExtent := strutH + strutD
	box := mrboxOf(smashedSlot(dataRows(tb)[0]))
	if box == nil {
		t.Fatal("no \\multirow content box")
	}
	// V = row + 0.4(rule) + row ⇒ shift = -Hr + V/2 + 2.5.
	V := rowExtent + defaultRule + rowExtent
	wantShift := -strutH + V/2 + (7*unity-2*unity)/2
	if box.shift != wantShift {
		t.Errorf("shift with interior \\hline = %d sp, want %d sp", box.shift, wantShift)
	}
	// And it is exactly defaultRule/2 lower than the un-ruled table.
	if diff := wantShift - (-strutH + rowExtent + (7*unity-2*unity)/2); diff != defaultRule/2 {
		t.Errorf("interior rule lowered the box by %d sp, want %d (half a rule)", diff, defaultRule/2)
	}
}

// A fixed width packs the content to that width (and sizes its column to it),
// unlike `*` which uses the natural width.
func TestMultirowFixedWidth(t *testing.T) {
	_, tb := runTab(t, `\begin{tabular}{ll}\multirow{2}{40pt}{X} & a \\ & b\end{tabular}`)
	box := mrboxOf(smashedSlot(dataRows(tb)[0]))
	if box == nil {
		t.Fatal("no \\multirow content box")
	}
	if box.width != 40*unity {
		t.Errorf("fixed-width content box = %d sp wide, want 40pt", box.width)
	}
	if box.kind != vbox {
		t.Errorf("fixed-width content should be a line-broken vbox, got kind %d", box.kind)
	}
}

// The content is aligned inside its column per the column's own alignment: a right
// column leads the slot with fil glue, a centred column brackets it with two.
func TestMultirowAlignment(t *testing.T) {
	_, tbR := runTab(t, `\begin{tabular}{r}\multirow{2}{*}{X} \\ y\end{tabular}`)
	slotR := smashedSlot(dataRows(tbR)[0])
	if slotR == nil {
		t.Fatal("no slot (r)")
	}
	if _, ok := slotR.list[0].(glueNode); !ok {
		t.Errorf("right column: slot[0] = %T, want leading fil glue", slotR.list[0])
	}

	_, tbC := runTab(t, `\begin{tabular}{c}\multirow{2}{*}{X} \\ y\end{tabular}`)
	slotC := smashedSlot(dataRows(tbC)[0])
	if slotC == nil {
		t.Fatal("no slot (c)")
	}
	if len(slotC.list) != 3 {
		t.Errorf("centred column: slot has %d items, want 3 (fil, box, fil)", len(slotC.list))
	}
	if _, ok := slotC.list[0].(glueNode); !ok {
		t.Errorf("centred column: slot[0] = %T, want fil glue", slotC.list[0])
	}
	if _, ok := slotC.list[2].(glueNode); !ok {
		t.Errorf("centred column: slot[2] = %T, want trailing fil glue", slotC.list[2])
	}
}

// A \multirow on a row that follows another row must be recognised — the newline
// after the previous row's \\ lands in the cell as a leading space.
func TestMultirowAfterRow(t *testing.T) {
	e, _ := runTab(t, "\\begin{tabular}{ll}\na & b \\\\\n\\multirow{2}{*}{Y} & c \\\\\n & d\n\\end{tabular}")
	if got := mvlText(e.mvl); got != "abYcd" {
		t.Errorf("text = %q, want abYcd", got)
	}
}

// Degenerate spans must not panic and must behave sanely: n<=0 clamps to 1 (the box
// centres within its single row, shift 0), n=1 likewise, and an n larger than the
// remaining rows simply spans what exists.
func TestMultirowDegenerate(t *testing.T) {
	// n = 1: the box centres in its own row, which is the STRUT's row rather than
	// the content's, so the shift is what centring 7pt-over-2pt inside .7/.3 of
	// \baselineskip asks for — a tenth of a point, not zero.
	e1, tb1 := runTab(t, `\begin{tabular}{ll}\multirow{1}{*}{X} & a\end{tabular}`)
	sh, sd := strutDims(e1)
	wantOwnRow := -sh + (sh+sd)/2 + (7*unity-2*unity)/2
	if box := mrboxOf(smashedSlot(dataRows(tb1)[0])); box == nil {
		t.Fatal("n=1: no content box")
	} else if box.shift != wantOwnRow {
		t.Errorf("n=1 shift = %d, want %d (centred in its own strutted row)", box.shift, wantOwnRow)
	}

	// n = 0 clamps to 1: same as above, no panic.
	if _, tb0 := runTab(t, `\begin{tabular}{ll}\multirow{0}{*}{X} & a\end{tabular}`); mrboxOf(smashedSlot(dataRows(tb0)[0])).shift != wantOwnRow {
		t.Errorf("n=0 should clamp to 1 (shift %d)", wantOwnRow)
	}

	// n = -3 clamps to 1: no panic.
	runTab(t, `\begin{tabular}{ll}\multirow{-3}{*}{X} & a\end{tabular}`)

	// n = 5 in a 2-row table: spans only the two rows that exist, no panic.
	_, tb5 := runTab(t, `\begin{tabular}{ll}\multirow{5}{*}{X} & a \\ & b\end{tabular}`)
	rows := dataRows(tb5)
	box := mrboxOf(smashedSlot(rows[0]))
	V := (rows[0].height + rows[0].depth) + (rows[1].height + rows[1].depth) // only 2 rows
	if want := -rows[0].height + V/2 + (box.height-box.depth)/2; box.shift != want {
		t.Errorf("over-long span shift = %d, want %d (extent capped at the 2 real rows)", box.shift, want)
	}
}

// Missing braces must not panic: \multirow with no arguments parses to n=1, natural
// (empty) content, and renders nothing without erroring.
func TestMultirowMissingBraces(t *testing.T) {
	_, tb := runTab(t, `\begin{tabular}{l}\multirow\end{tabular}`)
	rows := dataRows(tb)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	// The (empty) content box still exists as a smashed slot with a zero-size box.
	slot := smashedSlot(rows[0])
	if slot == nil {
		t.Fatal("no slot for an argument-less \\multirow")
	}
	if box := mrboxOf(slot); box == nil || box.width != 0 {
		t.Errorf("empty \\multirow content box = %v, want a zero-width box", box)
	}
}

// isMultirow / spannedExtent unit checks over hand-built structures, covering the
// detection edge cases and the rule-thickness accounting directly.
func TestMultirowHelpers(t *testing.T) {
	// isMultirow skips leading space, matches only \multirow.
	if !isMultirow([]tok{{cat: catSpace}, {cs_: true, cs: "multirow"}}) {
		t.Error("isMultirow should match a space-prefixed \\multirow")
	}
	if isMultirow([]tok{{cs_: true, cs: "multicolumn"}}) {
		t.Error("isMultirow must not match \\multicolumn")
	}
	if isMultirow(nil) {
		t.Error("isMultirow(nil) should be false")
	}

	// spannedExtent: two 9pt rows with an interior rule between them = 18pt + rule.
	r := &boxNode{height: 7 * unity, depth: 2 * unity}
	built := []tabBuilt{
		{cells: []builtCell{{}}},
		{hline: true},
		{cells: []builtCell{{}}},
		{cells: []builtCell{{}}},
	}
	rowBoxes := []*boxNode{r, nil, r, r}
	if got, want := spannedExtent(built, rowBoxes, 0, 2), 18*unity+defaultRule; got != want {
		t.Errorf("spannedExtent(0,2) = %d, want %d", got, want)
	}
	// A trailing rule beyond the last spanned row is excluded (span the 1st row only).
	if got, want := spannedExtent(built, rowBoxes, 0, 1), 9*unity; got != want {
		t.Errorf("spannedExtent(0,1) = %d, want %d (no trailing rule)", got, want)
	}
	// An over-long span sums only the rows that exist.
	if got, want := spannedExtent(built, rowBoxes, 2, 9), 18*unity; got != want {
		t.Errorf("spannedExtent(2,9) = %d, want %d", got, want)
	}
}

// strutDims is \@arstrutbox's height and depth for the engine's current
// \baselineskip: .7 and .3 of it, scaled by \arraystretch (latex.ltx:12102-12105).
// Every table row is raised to at least these, so a test that states a row's
// geometry has to ask for them rather than assume the content's own size.
func strutDims(e *Engine) (int, int) {
	f := e.arrayStretch()
	return int(0.7*f*float64(e.baselineskip) + 0.5), int(0.3*f*float64(e.baselineskip) + 0.5)
}

// Every table row carries \@arstrutbox, so a row is never shorter than the line it
// would occupy in running text. LaTeX copies it into each row (latex.ltx:12249) and
// builds it from \strutbox scaled by \arraystretch (l.12102-12105): .7 and .3 of
// \baselineskip.
//
// Measured against tectonic before this: a row of short cells cost 7.20pt where the
// reference gives 11.96 — \baselineskip to the hundredth — and one tabular came out
// 5.55pt flat. The deficit halved when the cells were made 20pt tall, which is what
// a missing MINIMUM height looks like as opposed to missing glue.
func TestTabularRowsCarryTheArrayStrut(t *testing.T) {
	e, tb := runTab(t, `\begin{tabular}{ll}a & b\\ c & d\end{tabular}`)
	strutH, strutD := strutDims(e)
	if strutH <= 7*unity {
		t.Fatalf("the strut (%d) is not taller than the 7pt letters; the test proves nothing", strutH)
	}
	rows := dataRows(tb)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for i, r := range rows {
		if r.height != strutH || r.depth != strutD {
			t.Errorf("row %d = h%d d%d, want the strut h%d d%d", i, r.height, r.depth, strutH, strutD)
		}
	}
}

// \arraystretch scales the strut, which is how a document opens a table up
// (\renewcommand{\arraystretch}{1.5}). It is a macro holding a number, not a
// register, so it is read and parsed.
func TestArrayStretchScalesTheRowStrut(t *testing.T) {
	e, tb := runTab(t, `\renewcommand{\arraystretch}{1.5}\begin{tabular}{ll}a & b\end{tabular}`)
	if f := e.arrayStretch(); f != 1.5 {
		t.Fatalf("\\arraystretch reads %v, want 1.5", f)
	}
	want := int(0.7*1.5*float64(e.baselineskip) + 0.5)
	if got := dataRows(tb)[0].height; got != want {
		t.Errorf("row height with \\arraystretch 1.5 = %d, want %d", got, want)
	}
}
