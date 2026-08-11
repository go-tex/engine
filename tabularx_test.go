// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// firstRow returns the first data-row hbox of an assembled table vbox.
func firstRow(tb *boxNode) *boxNode {
	for _, n := range tb.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			return b
		}
	}
	return nil
}

// rowSlots returns the per-column slot boxes of a data row, in column order (the
// hbox children; the interleaved \tabcolsep kerns and | rule nodes are skipped).
func rowSlots(row *boxNode) []*boxNode {
	var out []*boxNode
	for _, n := range row.list {
		if b, ok := n.(*boxNode); ok {
			out = append(out, b)
		}
	}
	return out
}

// countLines counts the line boxes inside a p/X paragraph cell: the slot is an hbox
// packed to the column width whose sole child is the paragraph vbox.
func countLines(slot *boxNode) int {
	vb, ok := slot.list[0].(*boxNode)
	if !ok || vb.kind != vbox {
		return 0
	}
	n := 0
	for _, c := range vb.list {
		if _, ok := c.(*boxNode); ok {
			n++
		}
	}
	return n
}

// A single X column takes all the leftover width: with one l cell of known width and
// no vertical rules, X width == W − 2·ncol·\tabcolsep − lwidth, and the assembled
// table width is exactly W.
func TestTabularxSingleX(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{}) // each letter 5pt
	// l cell "ab" = 10pt; ncol=2, no rules.
	if _, err := e.Run(`\begin{tabularx}{300pt}{lX}ab & some long paragraph text here\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no tabularx vbox contributed")
	}
	row := firstRow(tb)
	slots := rowSlots(row)
	if len(slots) != 2 {
		t.Fatalf("row has %d slots, want 2", len(slots))
	}
	lw := 10 * unity
	wantX := 300*unity - 2*2*tabColSep - lw // 300 − 24 − 10 = 266pt
	if slots[0].width != lw {
		t.Errorf("l slot width = %d sp, want %d (10pt)", slots[0].width, lw)
	}
	if slots[1].width != wantX {
		t.Errorf("X slot width = %d sp, want %d (266pt)", slots[1].width, wantX)
	}
	if row.width != 300*unity {
		t.Errorf("table width = %d sp, want 300pt (%d)", row.width, 300*unity)
	}
}

// Two X columns split the leftover equally, and the table still fills W exactly.
func TestTabularxTwoXSplitEqually(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{tabularx}{300pt}{XX}foo & bar\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	row := firstRow(tb)
	slots := rowSlots(row)
	if len(slots) != 2 {
		t.Fatalf("row has %d slots, want 2", len(slots))
	}
	leftover := 300*unity - 2*2*tabColSep // 300 − 24 = 276pt
	wantX := leftover / 2                 // 138pt
	if slots[0].width != wantX || slots[1].width != wantX {
		t.Errorf("X widths = %d,%d sp, want %d,%d (138pt each)", slots[0].width, slots[1].width, wantX, wantX)
	}
	if row.width != 300*unity {
		t.Errorf("table width = %d sp, want 300pt", row.width)
	}
}

// Vertical rules are charged against the leftover: with {|l|X|X|} the two X columns
// share W minus the fixed column, all \tabcolsep and the four | rules.
func TestTabularxBorderedRuleAccounting(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{tabularx}{300pt}{|l|X|X|}x & aa & bb\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	row := firstRow(tb)
	slots := rowSlots(row)
	if len(slots) != 3 {
		t.Fatalf("row has %d slots, want 3", len(slots))
	}
	lw := 5 * unity // "x"
	// ncol=3, 4 vertical rules (gaps 0..3).
	leftover := 300*unity - 2*3*tabColSep - 4*defaultRule - lw
	wantX := leftover / 2
	if slots[1].width != wantX || slots[2].width != wantX {
		t.Errorf("X widths = %d,%d sp, want %d each", slots[1].width, slots[2].width, wantX)
	}
	// Table width within rounding of W (only integer-division truncation may differ).
	if diff := row.width - 300*unity; diff < -2 || diff > 0 {
		t.Errorf("table width = %d sp, want ≈300pt (diff %d)", row.width, diff)
	}
}

// Long text in a narrow X column wraps onto several lines at the computed width.
func TestTabularxWraps(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// W=80pt, l="x"=5pt ⇒ X width = 80 − 24 − 5 = 51pt. Each "aaaaa" is 25pt, so two
	// words (25+3+25=53pt) overflow 51pt and every word lands on its own line.
	if _, err := e.Run(`\begin{tabularx}{80pt}{lX}x & aaaaa aaaaa aaaaa aaaaa\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	row := firstRow(tb)
	slots := rowSlots(row)
	wantX := 80*unity - 2*2*tabColSep - 5*unity // 51pt
	if slots[1].width != wantX {
		t.Errorf("X width = %d sp, want %d (51pt)", slots[1].width, wantX)
	}
	if lines := countLines(slots[1]); lines < 2 {
		t.Errorf("X cell wrapped into %d lines, want ≥2", lines)
	}
}

// No X column: tabularx degrades to an ordinary tabular; the width argument is read
// and ignored, and the table is built without panicking.
func TestTabularxNoX(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{tabularx}{300pt}{ll}a & bb\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no vbox contributed for X-less tabularx")
	}
	row := firstRow(tb)
	slots := rowSlots(row)
	// col0 "a"=5pt, col1 "bb"=10pt (natural, not stretched to fill 300pt).
	if slots[0].width != 5*unity || slots[1].width != 10*unity {
		t.Errorf("slot widths = %d,%d sp, want 5pt,10pt (natural)", slots[0].width, slots[1].width)
	}
}

// leftover ≤ 0 (target too small for the fixed material) falls back to minXColWidth
// per X column rather than crashing or producing a non-positive width.
func TestTabularxLeftoverNonPositive(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// "abcdefgh" = 40pt ≫ 5pt target, so leftover is negative.
	if _, err := e.Run(`\begin{tabularx}{5pt}{lX}abcdefgh & yy\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	row := firstRow(tb)
	slots := rowSlots(row)
	if slots[1].width != minXColWidth {
		t.Errorf("X width = %d sp, want fallback minXColWidth %d", slots[1].width, minXColWidth)
	}
}

// An empty {} width group reads as a zero dimen (missing width argument): the leftover
// is negative, so the X column falls back to minXColWidth without panicking.
func TestTabularxMissingWidth(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{tabularx}{}{lX}a & b\end{tabularx}`); err != nil {
		t.Fatal(err)
	}
	tb := lastVbox(e)
	if tb == nil {
		t.Fatal("no vbox contributed for zero-width tabularx")
	}
	row := firstRow(tb)
	slots := rowSlots(row)
	if slots[1].width != minXColWidth {
		t.Errorf("X width = %d sp, want fallback minXColWidth %d", slots[1].width, minXColWidth)
	}
}

// \linewidth/\textwidth/\columnwidth all resolve to \hsize, so they are accepted as
// the tabularx target width.
func TestTabularxWidthLengths(t *testing.T) {
	for _, w := range []string{`\linewidth`, `\textwidth`, `\columnwidth`, `\hsize`} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		e.hsize = 250 * unity
		src := `\begin{tabularx}{` + w + `}{lX}z & tail\end{tabularx}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", w, err)
		}
		tb := lastVbox(e)
		row := firstRow(tb)
		if row == nil {
			t.Fatalf("%s: no row", w)
		}
		wantX := 250*unity - 2*2*tabColSep - 5*unity // z=5pt
		slots := rowSlots(row)
		if slots[1].width != wantX {
			t.Errorf("%s: X width = %d sp, want %d", w, slots[1].width, wantX)
		}
	}
}

// A \multicolumn inside tabularx advances the column cursor by its span while X
// widths are measured, and a \multirow cell is skipped for natural sizing; both keep
// working and every row stays the same width.
func TestTabularxWithSpanningCells(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{tabularx}{300pt}{lXX}` +
		`\multicolumn{2}{c}{AB} & C \\ ` +
		`\multirow{2}{*}{d} & e & f \\ ` +
		`g & h & i` +
		`\end{tabularx}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("spanning cells in tabularx must not error: %v", err)
	}
	tb := lastVbox(e)
	var rows []*boxNode
	for _, n := range tb.list {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			rows = append(rows, b)
		}
	}
	if len(rows) < 2 {
		t.Fatalf("rows = %d, want ≥2", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].width != rows[0].width {
			t.Errorf("row %d width %d differs from row0 %d", i, rows[i].width, rows[0].width)
		}
	}
}

// A tabularx mixing a declared p{} column, an \hline rule and a row with more cells
// than columns: the p{} width is charged against the leftover, the rule row is
// skipped while measuring, and the surplus cell is ignored without panicking.
func TestTabularxPColumnHlineAndOverlongRow(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{tabularx}{300pt}{p{20pt}X}\hline aa & bb & surplus \\ cc & dd\end{tabularx}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("p+hline+overlong tabularx must not error: %v", err)
	}
	tb := lastVbox(e)
	row := firstRow(tb)
	slots := rowSlots(row)
	if len(slots) < 2 {
		t.Fatalf("row has %d slots, want ≥2", len(slots))
	}
	// p column fixed at 20pt; X takes the rest: 300 − 24 − 20 = 256pt.
	if slots[0].width != 20*unity {
		t.Errorf("p slot width = %d sp, want 20pt", slots[0].width)
	}
	wantX := 300*unity - 2*2*tabColSep - 20*unity
	if slots[1].width != wantX {
		t.Errorf("X slot width = %d sp, want %d (256pt)", slots[1].width, wantX)
	}
}

// multicolumnSpan reads only the {n} count from a raw \multicolumn cell (with the
// leading inter-row space), defaulting to 1 for a missing/invalid count.
func TestMulticolumnSpan(t *testing.T) {
	mk := func(s string) []tok {
		var ts []tok
		ts = append(ts, tok{ch: ' ', cat: catSpace}) // leading newline-space
		ts = append(ts, csTok("multicolumn"))
		for _, r := range s {
			cat := catOther
			if r == '{' {
				cat = catBegin
			} else if r == '}' {
				cat = catEnd
			}
			ts = append(ts, tok{ch: r, cat: cat})
		}
		return ts
	}
	if n := multicolumnSpan(mk(`{3}{c}{X}`)); n != 3 {
		t.Errorf("span = %d, want 3", n)
	}
	if n := multicolumnSpan(mk(`{}{c}{X}`)); n != 1 {
		t.Errorf("empty count span = %d, want 1", n)
	}
}
