// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's tabular environment as a primitive: \tabular
// (reached via \begin{tabular}) reads the column specification {|lcr|…}, collects
// the body up to \end{tabular}, splits it into rows (by \\) and cells (by &) with
// \hline markers between them, builds each cell aligned to its column (l/c/r via
// \hfil), computes each column's width as the widest cell, and stacks the rows —
// interspersed with \hline full-width rules and | vertical column rules — into a
// vbox. Not yet handled: p/m/b (paragraph) columns and \cline.

const tabColSep = 6 * unity // \tabcolsep default (added on each side of a column)

// tabItem is one item of a tabular body: a horizontal rule (\hline) or a data row.
type tabItem struct {
	hline bool
	cells [][]tok
}

// tabBuilt is a tabItem with its cells turned into aligned node lists.
type tabBuilt struct {
	hline bool
	cells [][]node
}

// doTabular typesets a tabular environment onto the current list.
func (e *Engine) doTabular() {
	aligns, vrules := e.scanColSpec()
	items := e.collectTabularBody()
	ncol := len(aligns)

	var built []tabBuilt
	for _, it := range items {
		if it.hline {
			built = append(built, tabBuilt{hline: true})
			continue
		}
		var cells [][]node
		for j, toks := range it.cells {
			a := byte('l')
			if j < ncol {
				a = aligns[j]
			}
			cells = append(cells, e.buildAlignedCell(toks, a))
		}
		built = append(built, tabBuilt{cells: cells})
	}
	e.place(assembleTabular(built, ncol, vrules))
}

// scanColSpec reads the {…} column specification, returning the l/c/r alignments
// and, for each gap 0..ncol, whether a vertical rule (|) sits there.
func (e *Engine) scanColSpec() ([]byte, []bool) {
	e.skipOptSpace()
	var aligns []byte
	ruleGaps := map[int]bool{}
	col := 0
	if t, ok := e.getXToken(); !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return aligns, []bool{}
	}
	for {
		u, ok := e.getNext()
		if !ok || (u.cat == catEnd && !u.cs_) {
			break
		}
		if u.cs_ {
			continue
		}
		switch u.ch {
		case 'l', 'c', 'r':
			aligns = append(aligns, byte(u.ch))
			col++
		case '|':
			ruleGaps[col] = true
		}
	}
	vrules := make([]bool, len(aligns)+1)
	for g := range ruleGaps {
		if g >= 0 && g <= len(aligns) {
			vrules[g] = true
		}
	}
	return aligns, vrules
}

// collectTabularBody reads raw tokens up to \end{tabular}, returning the rows and
// \hline markers. & separates cells, \\ separates rows, \hline is a rule marker.
func (e *Engine) collectTabularBody() []tabItem {
	var items []tabItem
	var cur [][]tok
	var cell []tok
	depth := 0
	endCell := func() { cur = append(cur, cell); cell = nil }
	endRow := func() {
		endCell()
		if !allEmpty(cur) {
			items = append(items, tabItem{cells: cur})
		}
		cur = nil
	}
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		switch {
		case depth == 0 && t.cs_ && t.cs == "end":
			if name := e.readBraceName(); name == "tabular" {
				endRow()
				return items
			}
		case depth == 0 && t.cs_ && t.cs == "hline":
			endRow() // flush any pending row, then record the rule
			items = append(items, tabItem{hline: true})
		case depth == 0 && t.cs_ && t.cs == `\`: // \\ row separator
			endRow()
		case depth == 0 && !t.cs_ && t.cat == catAlign: // & cell separator
			endCell()
		default:
			if !t.cs_ && t.cat == catBegin {
				depth++
			} else if !t.cs_ && t.cat == catEnd {
				depth--
			}
			cell = append(cell, t)
		}
	}
	return items
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

// assembleTabular computes column widths, builds each data row (cells packed to
// their column width, inter-column separation, and | vertical rules), and stacks
// the rows with \hline full-width rules into a vbox.
func assembleTabular(built []tabBuilt, ncol int, vrules []bool) *boxNode {
	colw := make([]int, ncol)
	for _, b := range built {
		if b.hline {
			continue
		}
		for j, cell := range b.cells {
			if j < ncol {
				if w := hpackSP(cell, packNatural, 0).width; w > colw[j] {
					colw[j] = w
				}
			}
		}
	}
	hasRule := func(gap int) bool { return gap < len(vrules) && vrules[gap] }
	vrule := func() ruleNode { return ruleNode{width: defaultRule, heightRun: true, depthRun: true} }

	// First pass: build each data-row hbox and find the widest (the table width).
	rowBoxes := make([]*boxNode, len(built))
	maxW := 0
	for i, b := range built {
		if b.hline {
			continue
		}
		var rn []node
		for j := 0; j <= ncol; j++ {
			if hasRule(j) {
				rn = append(rn, vrule())
			}
			if j < ncol {
				// \tabcolsep on both sides of every column keeps the content off
				// the vertical rules and gives 2·\tabcolsep between columns.
				var cell []node
				if j < len(b.cells) {
					cell = b.cells[j]
				}
				rn = append(rn, kernNode{width: tabColSep})
				rn = append(rn, hpackSP(cell, packTo, colw[j]))
				rn = append(rn, kernNode{width: tabColSep})
			}
		}
		box := hpackSP(rn, packNatural, 0)
		rowBoxes[i] = box
		if box.width > maxW {
			maxW = box.width
		}
	}

	// Second pass: emit rows and \hline rules (spanning the full table width).
	var vlist []node
	for i, b := range built {
		if b.hline {
			vlist = append(vlist, ruleNode{width: maxW, height: defaultRule})
		} else {
			vlist = append(vlist, rowBoxes[i])
		}
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

// allEmpty reports whether a row has no real content — every cell is empty or
// only whitespace (e.g. the space between \\ and \hline, or a trailing \\).
func allEmpty(row [][]tok) bool {
	for _, c := range row {
		for _, t := range c {
			if t.cs_ || t.cat != catSpace {
				return false
			}
		}
	}
	return true
}
