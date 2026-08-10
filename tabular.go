// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's tabular environment as a primitive: \tabular
// (reached via \begin{tabular}) reads the column specification {lcr…}, collects
// the body up to \end{tabular}, splits it into rows (by \\) and cells (by &),
// builds each cell aligned to its column (l/c/r via \hfil), computes each
// column's width as the widest cell, packs every cell to that width, and stacks
// the rows into a vbox. Vertical rules (|) and p/m/b columns are not yet handled.

const tabColSep = 6 * unity // \tabcolsep default (added on each side of a column)

// doTabular typesets a tabular environment onto the current list.
func (e *Engine) doTabular() {
	aligns := e.scanColSpec()
	rows := e.collectTabularBody()
	ncol := len(aligns)

	var cellRows [][][]node
	for _, row := range rows {
		var cells [][]node
		for j, toks := range row {
			a := byte('l')
			if j < ncol {
				a = aligns[j]
			}
			cells = append(cells, e.buildAlignedCell(toks, a))
		}
		cellRows = append(cellRows, cells)
	}
	e.place(assembleTabular(cellRows, ncol))
}

// scanColSpec reads the {…} column specification, keeping the l/c/r alignments.
func (e *Engine) scanColSpec() []byte {
	e.skipOptSpace()
	var spec []byte
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return spec
	}
	for {
		u, ok := e.getNext()
		if !ok || (u.cat == catEnd && !u.cs_) {
			break
		}
		if !u.cs_ {
			switch u.ch {
			case 'l', 'c', 'r':
				spec = append(spec, byte(u.ch))
			}
		}
	}
	return spec
}

// collectTabularBody reads raw tokens up to \end{tabular}, returning rows of
// cells (each a token slice). & separates cells, \\ separates rows.
func (e *Engine) collectTabularBody() [][][]tok {
	var rows [][][]tok
	var cur [][]tok
	var cell []tok
	depth := 0
	endCell := func() { cur = append(cur, cell); cell = nil }
	endRow := func() {
		endCell()
		if !allEmpty(cur) {
			rows = append(rows, cur)
		}
		cur = nil
	}
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		if depth == 0 && t.cs_ && t.cs == "end" {
			if name := e.readBraceName(); name == "tabular" {
				endRow()
				break
			}
			continue
		}
		if depth == 0 && t.cs_ && t.cs == `\` { // \\ row separator
			endRow()
			continue
		}
		if depth == 0 && !t.cs_ && t.cat == catAlign { // & cell separator
			endCell()
			continue
		}
		if !t.cs_ && t.cat == catBegin {
			depth++
		} else if !t.cs_ && t.cat == catEnd {
			depth--
		}
		cell = append(cell, t)
	}
	return rows
}

// readBraceName reads a {name} group and returns its text (used for \end{name}).
func (e *Engine) readBraceName() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return ""
	}
	var b []rune
	for {
		u, ok := e.getNext()
		if !ok || (u.cat == catEnd && !u.cs_) {
			break
		}
		if !u.cs_ {
			b = append(b, u.ch)
		}
	}
	return string(b)
}

// buildAlignedCell builds a cell's horizontal list and wraps it with \hfil glue
// so it aligns left, centre or right when packed to the column width.
func (e *Engine) buildAlignedCell(toks []tok, align byte) []node {
	content := e.buildCellHList(trimSpaceToks(toks))
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	switch align {
	case 'r':
		return append([]node{fil}, content...)
	case 'c':
		out := append([]node{fil}, content...)
		return append(out, fil)
	default: // 'l'
		return append(content, fil)
	}
}

// assembleTabular computes column widths, packs each cell to its column width
// with inter-column separation, and stacks the rows into a vbox.
func assembleTabular(cellRows [][][]node, ncol int) *boxNode {
	colw := make([]int, ncol)
	for _, row := range cellRows {
		for j, cell := range row {
			if j < ncol {
				if w := hpackSP(cell, packNatural, 0).width; w > colw[j] {
					colw[j] = w
				}
			}
		}
	}
	var vlist []node
	for _, row := range cellRows {
		var rowNodes []node
		for j, cell := range row {
			if j > 0 {
				rowNodes = append(rowNodes, kernNode{width: 2 * tabColSep})
			}
			width := 0
			if j < ncol {
				width = colw[j]
			}
			rowNodes = append(rowNodes, hpackSP(cell, packTo, width))
		}
		vlist = append(vlist, hpackSP(rowNodes, packNatural, 0))
	}
	return vpackSP(vlist, packNatural, 0)
}

// trimSpaceToks drops leading and trailing space tokens from a cell (LaTeX
// ignores whitespace adjacent to & and \\).
func trimSpaceToks(toks []tok) []tok {
	for len(toks) > 0 && !toks[0].cs_ && toks[0].cat == catSpace {
		toks = toks[1:]
	}
	for len(toks) > 0 && !toks[len(toks)-1].cs_ && toks[len(toks)-1].cat == catSpace {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// allEmpty reports whether every cell in a row is empty (a trailing \\ artefact).
func allEmpty(row [][]tok) bool {
	for _, c := range row {
		if len(c) > 0 {
			return false
		}
	}
	return true
}
