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

// tabItem is one item of a tabular body: a full rule (\hline), a partial rule
// (\cline{from-to}, 1-based inclusive column range), or a data row.
type tabItem struct {
	hline bool
	cline bool
	cfrom int
	cto   int
	cells [][]tok
}

// builtCell is one built table cell: its aligned node list plus how many columns
// it spans. A \multicolumn cell has span>1, its own alignment, and | borders taken
// from its {spec} rather than the table's column spec.
type builtCell struct {
	nodes          []node
	span           int  // columns spanned (1 for an ordinary cell)
	multicol       bool // built from \multicolumn
	lvrule, rvrule bool // \multicolumn's own left/right | borders
}

// tabBuilt is a tabItem with its cells turned into aligned node lists.
type tabBuilt struct {
	hline bool
	cline bool
	cfrom int
	cto   int
	cells []builtCell
}

// doTabular typesets a tabular environment onto the current list.
func (e *Engine) doTabular() {
	aligns, pwidths, vrules := e.scanColSpec()
	items := e.collectTabularBody()
	ncol := len(aligns)

	var built []tabBuilt
	for _, it := range items {
		if it.hline {
			built = append(built, tabBuilt{hline: true})
			continue
		}
		if it.cline {
			built = append(built, tabBuilt{cline: true, cfrom: it.cfrom, cto: it.cto})
			continue
		}
		var cells []builtCell
		col := 0 // column this cell starts at (advanced by spans)
		for _, toks := range it.cells {
			if isMulticolumn(toks) {
				bc := e.buildMulticolumn(toks)
				cells = append(cells, bc)
				col += bc.span
				continue
			}
			a := byte('l')
			pw := 0
			if col < ncol {
				a = aligns[col]
				pw = pwidths[col]
			}
			cells = append(cells, builtCell{nodes: e.buildAlignedCell(toks, a, pw), span: 1})
			col++
		}
		built = append(built, tabBuilt{cells: cells})
	}
	e.place(assembleTabular(built, ncol, pwidths, vrules))
}

// isMulticolumn reports whether a raw cell begins with \multicolumn (past any
// leading space — the newline after a row's \\ lands in the next cell as one).
func isMulticolumn(toks []tok) bool {
	for _, t := range toks {
		if !t.cs_ && t.cat == catSpace {
			continue
		}
		return t.cs_ && t.cs == "multicolumn"
	}
	return false
}

// buildMulticolumn parses a \multicolumn{n}{spec}{content} cell (the tokens are
// raw, collected by collectTabularBody) into a spanning builtCell: the content is
// built aligned per its own spec, and the spec's leading/trailing | become the
// cell's borders.
func (e *Engine) buildMulticolumn(toks []tok) builtCell {
	i := 0
	for i < len(toks) && !toks[i].cs_ && toks[i].cat == catSpace {
		i++ // skip the leading space (the newline after the previous row's \\)
	}
	i++ // skip \multicolumn
	var nTok, specTok, contentTok []tok
	nTok, i = grabBraceGroup(toks, i)
	specTok, i = grabBraceGroup(toks, i)
	contentTok, _ = grabBraceGroup(toks, i)
	span := toksToInt(nTok)
	if span < 1 {
		span = 1
	}
	align, lv, rv := parseColSpecToks(specTok)
	return builtCell{
		nodes:    e.buildAlignedCell(contentTok, align, 0),
		span:     span,
		multicol: true,
		lvrule:   lv,
		rvrule:   rv,
	}
}

// grabBraceGroup reads a {…} group from a token slice starting at i (skipping
// leading spaces), returning the inner tokens and the index just past the '}'.
func grabBraceGroup(toks []tok, i int) ([]tok, int) {
	for i < len(toks) && !toks[i].cs_ && toks[i].cat == catSpace {
		i++
	}
	if i >= len(toks) || toks[i].cs_ || toks[i].cat != catBegin {
		return nil, i
	}
	i++ // past '{'
	depth := 0
	var inner []tok
	for i < len(toks) {
		t := toks[i]
		if !t.cs_ && t.cat == catBegin {
			depth++
		} else if !t.cs_ && t.cat == catEnd {
			if depth == 0 {
				i++
				break
			}
			depth--
		}
		inner = append(inner, t)
		i++
	}
	return inner, i
}

// toksToInt parses a run of digit character tokens into an integer.
func toksToInt(toks []tok) int {
	n := 0
	for _, t := range toks {
		if !t.cs_ && t.ch >= '0' && t.ch <= '9' {
			n = n*10 + int(t.ch-'0')
		}
	}
	return n
}

// parseColSpecToks reads a single-column \multicolumn spec ({|c|}, {r}, …):
// the l/c/r alignment (first found; default l) and whether a | precedes the
// alignment (left border) or follows it (right border).
func parseColSpecToks(toks []tok) (align byte, lvrule, rvrule bool) {
	align = 'l'
	seenAlign := false
	for _, t := range toks {
		if t.cs_ {
			continue
		}
		switch t.ch {
		case 'l', 'c', 'r':
			if !seenAlign {
				align = byte(t.ch)
				seenAlign = true
			}
		case '|':
			if seenAlign {
				rvrule = true
			} else {
				lvrule = true
			}
		}
	}
	return align, lvrule, rvrule
}

// scanColSpec reads the {…} column specification, returning the l/c/r/p alignments,
// the fixed width of each p{…} column (0 for l/c/r), and, for each gap 0..ncol,
// whether a vertical rule (|) sits there.
func (e *Engine) scanColSpec() ([]byte, []int, []bool) {
	e.skipOptSpace()
	var aligns []byte
	var pwidths []int
	ruleGaps := map[int]bool{}
	col := 0
	if t, ok := e.getXToken(); !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return aligns, pwidths, []bool{}
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
			pwidths = append(pwidths, 0)
			col++
		case 'p', 'm', 'b': // paragraph column: p{width} (m/b treated as p)
			aligns = append(aligns, 'p')
			pwidths = append(pwidths, e.readBraceDimen())
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
	return aligns, pwidths, vrules
}

// breakToVbox line-breaks a cell's horizontal list to a fixed width and returns a
// top-aligned vbox (LaTeX's p{width} column is a \parbox[t]): the reference point
// is the baseline of the first line, so the row baseline meets the cell's top line.
func (e *Engine) breakToVbox(hlist []node, width int) *boxNode {
	list := append([]node{}, hlist...)
	// \parfillskip (0pt plus 1fil) fills the last line; a forced break ends it.
	list = append(list,
		glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}},
		penaltyNode{penalty: -int(InfPenalty)})

	items := toItems(list)
	lineWidth := spToPt(width)
	lines, ok := KnuthPlass(items, lineWidth, 200, 10)
	if !ok || hasBadLine(lines) {
		if l2, ok2 := KnuthPlass(items, lineWidth, maxBadRatio, 10); ok2 {
			if !ok || len(l2) > len(lines) || !hasBadLine(l2) {
				lines, ok = l2, true
			}
		}
	}
	if !ok || len(lines) == 0 {
		lines = []Line{{Start: 0, End: len(list)}}
	}

	var vlist []node
	prevDepth := ignoreDepth
	for _, ln := range lines {
		seg := trimLeadingGlue(list[ln.Start:ln.End])
		if ln.End < len(list) {
			if _, isDisc := list[ln.End].(discNode); isDisc && e.curFont != nil {
				w, h, dd := e.curFont.charDimsSP('-')
				seg = append(append([]node{}, seg...), charNode{ch: '-', width: w, height: h, depth: dd})
			}
		}
		line := hpackSP(seg, packTo, width)
		if prevDepth > ignoreDepth {
			gap := e.baselineskip - prevDepth - line.height
			if gap < e.lineskip {
				gap = e.lineskip
			}
			vlist = append(vlist, glueNode{spec: glueSpec{width: gap}})
		}
		vlist = append(vlist, line)
		prevDepth = line.depth
	}
	b := vpackSP(vlist, packNatural, 0)
	b.width = width // the column's fixed width (vpack derives width from contents)
	// Convert to \parbox[t]: height = first line's height, depth = the rest.
	total := b.height + b.depth
	firstH := 0
	for _, n := range b.list {
		if lb, ok := n.(*boxNode); ok {
			firstH = lb.height
			break
		}
	}
	b.height = firstH
	b.depth = total - firstH
	return b
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
		case depth == 0 && t.cs_ && t.cs == "cline":
			endRow()
			from, to := parseCline(e.readBraceName())
			items = append(items, tabItem{cline: true, cfrom: from, cto: to})
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
func (e *Engine) buildAlignedCell(toks []tok, align byte, pwidth int) []node {
	content := e.buildCellHList(trimSpaceToks(toks))
	if align == 'p' { // paragraph column: line-break to the fixed width as a vbox
		return []node{e.breakToVbox(content, pwidth)}
	}
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
func assembleTabular(built []tabBuilt, ncol int, pwidths []int, vrules []bool) *boxNode {
	// Column widths come from single-column cells only (a \multicolumn spans several
	// and is sized to their total). A column cursor tracks the real column each cell
	// occupies, since a span makes the cell index diverge from the column index.
	colw := make([]int, ncol)
	for _, b := range built {
		if b.hline || b.cline {
			continue
		}
		col := 0
		for _, cell := range b.cells {
			if col >= ncol {
				break
			}
			if cell.span <= 1 {
				if w := hpackSP(cell.nodes, packNatural, 0).width; w > colw[col] {
					colw[col] = w
				}
				col++
			} else {
				col += cell.span
			}
		}
	}
	// p{…} columns have a fixed width, independent of their content.
	for j := 0; j < ncol && j < len(pwidths); j++ {
		if pwidths[j] > 0 {
			colw[j] = pwidths[j]
		}
	}
	hasRule := func(gap int) bool { return gap < len(vrules) && vrules[gap] }
	vrule := func() ruleNode { return ruleNode{width: defaultRule, heightRun: true, depthRun: true} }

	// First pass: build each data-row hbox and find the widest (the table width).
	rowBoxes := make([]*boxNode, len(built))
	maxW := 0
	for i, b := range built {
		if b.hline || b.cline {
			continue
		}
		rowBoxes[i] = buildTabRow(b.cells, ncol, colw, hasRule, vrule)
		if rowBoxes[i].width > maxW {
			maxW = rowBoxes[i].width
		}
	}

	// Second pass: emit rows, \hline rules (full width) and \cline partial rules.
	var vlist []node
	for i, b := range built {
		switch {
		case b.hline:
			vlist = append(vlist, ruleNode{width: maxW, height: defaultRule})
		case b.cline:
			if r := clineRule(b.cfrom, b.cto, ncol, colw, hasRule); r != nil {
				vlist = append(vlist, r)
			}
		default:
			vlist = append(vlist, rowBoxes[i])
		}
	}
	return vpackSP(vlist, packNatural, 0)
}

// buildTabRow lays one data row into an hbox using a column cursor so \multicolumn
// spans align with ordinary rows. Each cell emits its left | border (its own for a
// \multicolumn, the table's otherwise) then a \tabcolsep-padded content block sized
// to its column(s); a final spanning cell's right | overrides the table's trailing
// border. Short rows are padded with empty columns so every row is the same width.
func buildTabRow(cells []builtCell, ncol int, colw []int, hasRule func(int) bool, vrule func() ruleNode) *boxNode {
	var rn []node
	col := 0
	lastSpanToEnd, lastRvrule := false, false
	for _, cell := range cells {
		if col >= ncol {
			break
		}
		span := cell.span
		if span < 1 {
			span = 1
		}
		if col+span > ncol {
			span = ncol - col
		}
		if cell.multicol {
			if cell.lvrule {
				rn = append(rn, vrule())
			}
		} else if hasRule(col) {
			rn = append(rn, vrule())
		}
		innerW := 2 * (span - 1) * tabColSep
		for k := col; k < col+span; k++ {
			innerW += colw[k]
		}
		for g := col + 1; g < col+span; g++ {
			if hasRule(g) {
				innerW += defaultRule
			}
		}
		rn = append(rn, kernNode{width: tabColSep}, hpackSP(cell.nodes, packTo, innerW), kernNode{width: tabColSep})
		col += span
		lastSpanToEnd = cell.multicol && col == ncol
		lastRvrule = cell.rvrule
	}
	for col < ncol { // pad a short row so all rows share the table width
		if hasRule(col) {
			rn = append(rn, vrule())
		}
		rn = append(rn, kernNode{width: tabColSep}, hpackSP(nil, packTo, colw[col]), kernNode{width: tabColSep})
		col++
	}
	switch {
	case lastSpanToEnd:
		if lastRvrule {
			rn = append(rn, vrule())
		}
	case hasRule(ncol):
		rn = append(rn, vrule())
	}
	return hpackSP(rn, packNatural, 0)
}

// clineRule builds the partial horizontal rule for \cline{from-to} (1-based,
// inclusive): a left kern to the start of column `from` followed by a rule that
// spans columns from..to (their \tabcolsep padding and any interior | rules).
func clineRule(from, to, ncol int, colw []int, hasRule func(int) bool) node {
	if from < 1 || to < from || from > ncol {
		return nil
	}
	ci, cj := from-1, to-1
	if cj >= ncol {
		cj = ncol - 1
	}
	left := 0
	for j := 0; j <= ci; j++ {
		if hasRule(j) {
			left += defaultRule
		}
	}
	for j := 0; j < ci; j++ {
		left += 2*tabColSep + colw[j]
	}
	w := 0
	for j := ci; j <= cj; j++ {
		w += 2*tabColSep + colw[j]
	}
	for g := ci + 1; g <= cj; g++ {
		if hasRule(g) {
			w += defaultRule
		}
	}
	return hpackSP([]node{kernNode{width: left}, ruleNode{width: w, height: defaultRule}}, packNatural, 0)
}

// parseCline parses \cline's "from-to" argument (e.g. "2-3") into 1-based column
// indices. A malformed argument yields (0, 0), which clineRule renders as nothing.
func parseCline(s string) (int, int) {
	dash := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '-' {
			dash = i
			break
		}
	}
	if dash < 0 {
		return 0, 0
	}
	return atoiTrim(s[:dash]), atoiTrim(s[dash+1:])
}

// atoiTrim parses a run of ASCII digits (ignoring surrounding spaces) into an int.
func atoiTrim(s string) int {
	n, seen := 0, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
			seen = true
		} else if c == ' ' && !seen {
			continue
		} else {
			break
		}
	}
	if !seen {
		return 0
	}
	return n
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
