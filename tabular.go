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

// minXColWidth is the fallback width of a tabularx X column when the target width
// leaves no room (leftover ≤ 0): a small positive width keeps the table renderable
// instead of collapsing to zero or a negative width.
const minXColWidth = 10 * unity // 10pt

// booktabs rule weights and spacing (all in sp). booktabs' \heavyrulewidth
// defaults to 0.08em (≈0.8pt at a 10pt base) and \lightrulewidth to 0.05em; we
// approximate the heavy rule (\toprule/\bottomrule) at 0.8pt = 2×defaultRule and
// the light rule (\midrule and \cmidrule) at 0.4pt = defaultRule, so the heavy
// rule is visibly twice the mid rule. bookRuleSep is the vertical breathing room
// booktabs adds around every rule (\abovetopsep/\aboverulesep/\belowrulesep are a
// few tenths of an ex each); a fixed 2pt glue above and below each rule
// reproduces booktabs' airier look versus \hline's tight rules. cmidTrim is the
// fixed kern shaved off a \cmidrule's trimmed side — booktabs uses \cmidrulekern
// = 0.5em, which we lightly honour with a fixed 3pt.
const (
	heavyRuleWidth = 2 * defaultRule // 0.8pt: \toprule, \bottomrule
	lightRuleWidth = defaultRule     // 0.4pt: \midrule, \cmidrule
	bookRuleSep    = 2 * unity       // 2pt vertical space above and below a rule
	cmidTrim       = 3 * unity       // 3pt (l)/(r)/(lr) trimming kern
)

// tabItem is one item of a tabular body: a full rule (\hline), a partial rule
// (\cline{from-to}, 1-based inclusive column range), or a data row.
type tabItem struct {
	hline bool
	cline bool
	cfrom int
	cto   int
	cells [][]tok
	// booktabs rules: brule is a full-width \toprule/\midrule/\bottomrule, bcmid
	// is a partial \cmidrule (reusing cfrom/cto as its 1-based column range).
	// weight is the rule thickness in sp; btrimL/btrimR carry \cmidrule's (l)/(r)
	// trimming.
	brule  bool
	bcmid  bool
	weight int
	btrimL bool
	btrimR bool
}

// builtCell is one built table cell: its aligned node list plus how many columns
// it spans. A \multicolumn cell has span>1, its own alignment, and | borders taken
// from its {spec} rather than the table's column spec. A \multirow cell has
// multirow set, spans mrows rows vertically (one column), and carries its content
// in mrbox — a single box whose .shift is computed once the spanned rows' heights
// are known, so the box is vertically centred over the block it covers.
type builtCell struct {
	nodes          []node
	span           int  // columns spanned (1 for an ordinary cell)
	multicol       bool // built from \multicolumn
	lvrule, rvrule bool // \multicolumn's own left/right | borders
	multirow       bool // built from \multirow
	mrows          int  // rows spanned (>=1)
	mralign        byte // column alignment the content is set to (l/c/r/p)
	mrbox          *boxNode
}

// tabBuilt is a tabItem with its cells turned into aligned node lists.
type tabBuilt struct {
	hline bool
	cline bool
	cfrom int
	cto   int
	cells []builtCell
	// booktabs rules (see tabItem): brule = full-width, bcmid = partial \cmidrule.
	brule  bool
	bcmid  bool
	weight int
	btrimL bool
	btrimR bool
}

// isRule reports whether this item is any horizontal rule (an \hline/\cline or a
// booktabs \toprule/\midrule/\bottomrule/\cmidrule) rather than a data row.
func (b tabBuilt) isRule() bool { return b.hline || b.cline || b.brule || b.bcmid }

// doTabular typesets a tabular environment onto the current list.
func (e *Engine) doTabular() {
	aligns, pwidths, vrules := e.scanColSpec()
	items := e.collectTabularBody("tabular")
	e.place(e.buildTabularBox(aligns, pwidths, vrules, items))
}

// doTabularx typesets a tabularx environment: \begin{tabularx}{W}{spec}. Unlike
// tabular it takes a leading {width} argument (a rigid <dimen>, e.g. \hsize or
// \linewidth); every X column in {spec} is a paragraph column whose width is
// computed by resolveXWidths so the assembled table fills W. Once the X columns
// have been rewritten into ordinary p{} columns the rest of the tabular machinery
// (rules, \\, &, \multicolumn, \multirow) is reused unchanged.
func (e *Engine) doTabularx() {
	width := e.readBraceDimen()
	aligns, pwidths, vrules := e.scanColSpec()
	items := e.collectTabularBody("tabularx")
	e.resolveXWidths(width, aligns, pwidths, vrules, items)
	e.place(e.buildTabularBox(aligns, pwidths, vrules, items))
}

// buildTabularBox turns the parsed column spec and collected body items into the
// assembled table vbox. Shared by tabular and tabularx (the latter after its X
// columns have been resolved to fixed-width p columns).
func (e *Engine) buildTabularBox(aligns []byte, pwidths []int, vrules []bool, items []tabItem) *boxNode {
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
		if it.brule {
			built = append(built, tabBuilt{brule: true, weight: it.weight})
			continue
		}
		if it.bcmid {
			built = append(built, tabBuilt{bcmid: true, cfrom: it.cfrom, cto: it.cto,
				weight: it.weight, btrimL: it.btrimL, btrimR: it.btrimR})
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
			if isMultirow(toks) {
				a := byte('l')
				if col < ncol {
					a = aligns[col]
				}
				bc := e.buildMultirow(toks, a)
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
	return assembleTabular(built, ncol, pwidths, vrules)
}

// resolveXWidths computes the width of each X (flexible paragraph) column so that a
// tabularx of target width W is filled exactly, then rewrites every X column into an
// ordinary p{} column of that width. The assembled table width is
//
//	2·ncol·\tabcolsep + Σ colw + (#vertical rules)·\defaultrule
//
// so the X columns share whatever is left of W once the fixed material is removed:
//
//	leftover  = W − 2·ncol·\tabcolsep − (#rules)·\defaultrule − Σ(non-X widths)
//	X width   = leftover / (number of X columns)   (integer, truncated)
//
// Σ(non-X widths) is the declared p{} widths plus the natural (widest single-column
// cell) width of every l/c/r column. A single X takes all the leftover, so the table
// width matches W exactly; several X columns split it equally (any 1–(n−1) sp lost to
// truncation leaves the total within rounding of W). If leftover ≤ 0 — the target is
// too small to hold the fixed columns and separators — each X falls back to
// minXColWidth so nothing collapses or panics. \multicolumn / \multirow cells are not
// measured into column widths here (assembleTabular sizes a \multicolumn to the
// columns it spans), matching that later sizing; this is best effort for the exotic
// case of a column whose only content is spanned. Column decorators >{…}/<{…} are not
// supported: an X column is treated plainly.
func (e *Engine) resolveXWidths(width int, aligns []byte, pwidths []int, vrules []bool, items []tabItem) {
	ncol := len(aligns)
	var xcols []int
	for j := 0; j < ncol; j++ {
		if aligns[j] == 'X' {
			xcols = append(xcols, j)
		}
	}
	if len(xcols) == 0 {
		return // no X column: an ordinary tabular with an (ignored) width argument
	}
	// Non-X column widths: declared p{} widths, then the widest single-column l/c/r
	// cell measured at its natural width.
	colw := make([]int, ncol)
	for j := 0; j < ncol; j++ {
		if aligns[j] != 'X' && pwidths[j] > 0 {
			colw[j] = pwidths[j]
		}
	}
	for _, it := range items {
		if it.hline || it.cline || it.brule || it.bcmid {
			continue
		}
		col := 0
		for _, toks := range it.cells {
			if col >= ncol {
				break
			}
			if isMulticolumn(toks) {
				col += multicolumnSpan(toks)
				continue
			}
			if isMultirow(toks) {
				col++ // multirow carries its own width; not measured as an l/c/r cell
				continue
			}
			switch aligns[col] {
			case 'l', 'c', 'r':
				w := hpackSP(e.buildCellHList(trimSpaceToks(toks)), packNatural, 0).width
				if w > colw[col] {
					colw[col] = w
				}
			}
			col++
		}
	}
	fixed := 0
	for j := 0; j < ncol; j++ {
		fixed += colw[j] // X columns contribute 0 here
	}
	nrules := 0
	for _, v := range vrules {
		if v {
			nrules++
		}
	}
	leftover := width - 2*ncol*tabColSep - nrules*defaultRule - fixed
	xw := leftover / len(xcols)
	if xw < minXColWidth {
		xw = minXColWidth
	}
	for _, j := range xcols {
		aligns[j] = 'p'
		pwidths[j] = xw
	}
}

// multicolumnSpan reads just the {n} span count of a raw \multicolumn cell (used to
// advance the column cursor while measuring X widths). A missing/invalid count is 1.
func multicolumnSpan(toks []tok) int {
	i := 0
	for i < len(toks) && !toks[i].cs_ && toks[i].cat == catSpace {
		i++ // skip the leading space (the newline after the previous row's \\)
	}
	i++ // skip \multicolumn
	nTok, _ := grabBraceGroup(toks, i)
	span := toksToInt(nTok)
	if span < 1 {
		span = 1
	}
	return span
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

// isMultirow reports whether a raw cell begins with \multirow (past any leading
// space — the newline after a row's \\ lands in the next cell as one).
func isMultirow(toks []tok) bool {
	for _, t := range toks {
		if !t.cs_ && t.cat == catSpace {
			continue
		}
		return t.cs_ && t.cs == "multirow"
	}
	return false
}

// buildMultirow parses a \multirow{n}{width}{content} cell into a spanning
// builtCell. n is the number of rows the cell covers (>=1); width is a dimension
// or `*` (natural width, parsed as 0); content is set to that width (line-broken
// like a p{} column when a fixed width is given, natural otherwise). The cell holds
// one column (span 1); assembleTabular later shifts mrbox so it is vertically
// centred over the n rows. align is the alignment the content is placed with inside
// its column (the table's column alignment, matching an ordinary cell).
func (e *Engine) buildMultirow(toks []tok, align byte) builtCell {
	i := 0
	for i < len(toks) && !toks[i].cs_ && toks[i].cat == catSpace {
		i++ // skip the leading space (the newline after the previous row's \\)
	}
	i++ // skip \multirow
	var nTok, wTok, cTok []tok
	nTok, i = grabBraceGroup(toks, i)
	wTok, i = grabBraceGroup(toks, i)
	cTok, _ = grabBraceGroup(toks, i)
	n := toksToInt(nTok)
	if n < 1 {
		n = 1
	}
	// parseDimenStr yields 0 for `*` (and for an empty/malformed width): a
	// non-positive width means "natural", a positive one packs to that width.
	width := parseDimenStr(e.toksToString(wTok))
	content := e.buildCellHList(trimSpaceToks(cTok))
	var box *boxNode
	if width > 0 {
		box = e.breakToVbox(content, width)
	} else {
		box = hpackSP(content, packNatural, 0)
	}
	return builtCell{span: 1, multirow: true, mrows: n, mralign: align, mrbox: box}
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
	// depth tracks {..} groups nested in the spec — the array-package prefixes
	// @{sep}, >{decl}, <{decl}, !{sep} each brace a group whose contents are not
	// column letters. Without this, the first inner } (e.g. from @{}) would be
	// mistaken for the spec's closing brace, leaking the rest of the preamble into
	// the body scanner (which then never finds \end{tabular} and swallows the
	// document). Only a } at depth 0 closes the spec.
	depth := 0
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if u.cs_ {
			continue
		}
		switch {
		case u.cat == catBegin:
			depth++
			continue
		case u.cat == catEnd:
			if depth == 0 {
				goto done // the spec's own closing brace
			}
			depth--
			continue
		}
		if depth > 0 {
			continue // inside a @{}/>{}/<{}/!{} group: not column letters
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
		case 'X': // tabularx flexible paragraph column; width resolved later
			aligns = append(aligns, 'X')
			pwidths = append(pwidths, 0)
			col++
		case '|':
			ruleGaps[col] = true
		}
	}
done:
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

// collectTabularBody reads raw tokens up to \end{env} (env is "tabular" or
// "tabularx"), returning the rows and \hline markers. & separates cells, \\ separates
// rows, \hline is a rule marker.
func (e *Engine) collectTabularBody(env string) []tabItem {
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
		case depth == 0 && t.cs_ && t.cs != "end" && e.expandsToEnd(t):
			// A user macro standing in for \end{...} (e.g. \newcommand\etab{\end{tabular}}):
			// read raw here it hides the \end, so the body scanner would run to EOF and
			// swallow the rest of the document. Expand it in place so the real \end token
			// surfaces next iteration. Narrow: only a parameterless macro whose body
			// begins with \end (see expandsToEnd), so verbatim cell content is untouched.
			e.expandMacro(e.meaningOf(t))
		case depth == 0 && t.cs_ && t.cs == "end":
			if name := e.readBraceName(); name == env {
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
		case depth == 0 && t.cs_ && (t.cs == "toprule" || t.cs == "midrule" || t.cs == "bottomrule"):
			endRow()
			w := heavyRuleWidth
			if t.cs == "midrule" {
				w = lightRuleWidth
			}
			if d, ok := e.readOptBracketDimen(); ok && d > 0 {
				w = d // \toprule[<dimen>] weight override
			}
			items = append(items, tabItem{brule: true, weight: w})
		case depth == 0 && t.cs_ && t.cs == "cmidrule":
			endRow()
			w := lightRuleWidth
			if d, ok := e.readOptBracketDimen(); ok && d > 0 {
				w = d // \cmidrule[<dimen>] weight override
			}
			tl, tr := e.readCmidTrim() // optional (l)/(r)/(lr) trimming
			from, to := parseCline(e.readBraceName())
			items = append(items, tabItem{bcmid: true, cfrom: from, cto: to, weight: w, btrimL: tl, btrimR: tr})
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
	depth := 0
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if !u.cs_ && u.cat == catEnd {
			if depth == 0 {
				break // the matching close brace of the whole group
			}
			// A nested }: part of the argument (e.g. \tag{L~\ref{key}}). Balance
			// it so a group that closes an inner { does not leak its outer } into
			// the caller's token stream — an unconsumed } drives an align/tabular
			// cell scanner's brace depth negative, after which \end is no longer
			// recognised and the rest of the document is swallowed.
			depth--
			b = append(b, u.ch)
			continue
		}
		if !u.cs_ && u.cat == catBegin {
			depth++
		}
		if !u.cs_ {
			b = append(b, u.ch)
		}
	}
	return string(b)
}

// readOptDelimited reads an optional argument delimited by the given open/close
// characters (e.g. '['..']' or '('..')'), skipping leading spaces. It returns the
// inner text and true when the group is present; otherwise it pushes every token
// it inspected back onto the input (leaving the stream untouched) and returns
// false, so a rule with no optional argument is followed cleanly by the next row.
func (e *Engine) readOptDelimited(open, close rune) (string, bool) {
	var skipped []tok
	for {
		t, ok := e.getNext()
		if !ok {
			for i := len(skipped) - 1; i >= 0; i-- {
				e.back(skipped[i])
			}
			return "", false
		}
		if !t.cs_ && t.cat == catSpace {
			skipped = append(skipped, t)
			continue
		}
		if !t.cs_ && t.ch == open {
			var b []rune
			for {
				u, ok := e.getNext()
				if !ok || (!u.cs_ && u.ch == close) {
					break
				}
				if !u.cs_ {
					b = append(b, u.ch)
				}
			}
			return string(b), true
		}
		e.back(t) // not the opener: restore this token then the skipped spaces
		for i := len(skipped) - 1; i >= 0; i-- {
			e.back(skipped[i])
		}
		return "", false
	}
}

// readOptBracketDimen reads an optional [<dimen>] weight override (booktabs allows
// \toprule[<wd>] etc.) into scaled points, returning (0,false) when absent.
func (e *Engine) readOptBracketDimen() (int, bool) {
	s, ok := e.readOptDelimited('[', ']')
	if !ok {
		return 0, false
	}
	return parseDimenStr(s), true
}

// readCmidTrim reads \cmidrule's optional (l)/(r)/(lr) trimming specifier,
// returning which side(s) to shave. An absent specifier trims neither side.
func (e *Engine) readCmidTrim() (trimL, trimR bool) {
	s, ok := e.readOptDelimited('(', ')')
	if !ok {
		return false, false
	}
	for _, r := range s {
		switch r {
		case 'l':
			trimL = true
		case 'r':
			trimR = true
		}
	}
	return trimL, trimR
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
		if b.isRule() {
			continue
		}
		col := 0
		for _, cell := range b.cells {
			if col >= ncol {
				break
			}
			if cell.span <= 1 {
				w := 0
				if cell.multirow {
					w = cell.mrbox.width // sized to its content or the given width
				} else {
					w = hpackSP(cell.nodes, packNatural, 0).width
				}
				if w > colw[col] {
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
	// A \multirow slot is smashed to zero height here (so it never inflates its own
	// row); its content box is shifted vertically in the pass below.
	rowBoxes := make([]*boxNode, len(built))
	maxW := 0
	for i, b := range built {
		if b.isRule() {
			continue
		}
		rowBoxes[i] = buildTabRow(b.cells, ncol, colw, hasRule, vrule)
		if rowBoxes[i].width > maxW {
			maxW = rowBoxes[i].width
		}
	}

	// Vertical-centring pass: now that every row's height/depth is known, shift each
	// \multirow content box so it is centred over the block of rows it spans. The
	// block runs from the top of the multirow's own row to the bottom of its last
	// spanned row, counting interior \hline/\cline rules (defaultRule each).
	for i, b := range built {
		if b.isRule() {
			continue
		}
		for j := range b.cells {
			c := b.cells[j]
			if !c.multirow || c.mrbox == nil {
				continue
			}
			extent := spannedExtent(built, rowBoxes, i, c.mrows)
			// Content centre (relative to this row's baseline, +down) must equal the
			// block centre: shift + (Dc-Hc)/2 = -Hr + extent/2.
			hr := rowBoxes[i].height
			c.mrbox.shift = -hr + extent/2 + (c.mrbox.height-c.mrbox.depth)/2
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
		case b.brule: // booktabs \toprule/\midrule/\bottomrule: full-width, weighted, airy
			vlist = append(vlist,
				glueNode{spec: glueSpec{width: bookRuleSep}},
				ruleNode{width: maxW, height: b.weight},
				glueNode{spec: glueSpec{width: bookRuleSep}})
		case b.bcmid: // booktabs \cmidrule: partial rule with the same breathing room
			if r := partialColRule(b.cfrom, b.cto, ncol, colw, hasRule, b.weight, b.btrimL, b.btrimR); r != nil {
				vlist = append(vlist,
					glueNode{spec: glueSpec{width: bookRuleSep}}, r,
					glueNode{spec: glueSpec{width: bookRuleSep}})
			}
		default:
			vlist = append(vlist, rowBoxes[i])
		}
	}
	return vpackSP(vlist, packNatural, 0)
}

// spannedExtent returns the total vertical extent (sp) of the block a \multirow
// covers: the sum of the heights+depths of the n data rows starting at built index
// `start`, plus the thickness of every \hline/\cline rule sitting between them. It
// stops at the n-th data row (or the end of the table), so a rule after the block
// is excluded; an over-long span (n past the last row) simply sums what exists.
func spannedExtent(built []tabBuilt, rowBoxes []*boxNode, start, n int) int {
	total, seen := 0, 0
	for k := start; k < len(built) && seen < n; k++ {
		b := built[k]
		if b.isRule() {
			if seen > 0 {
				// booktabs rules add their own weight plus the breathing glue
				// above and below; \hline/\cline are a single defaultRule.
				if b.brule || b.bcmid {
					total += b.weight + 2*bookRuleSep
				} else {
					total += defaultRule
				}
			}
			continue
		}
		total += rowBoxes[k].height + rowBoxes[k].depth
		seen++
	}
	return total
}

// multirowSlot builds the column slot for a \multirow cell: the content box is
// aligned within the column (l/c/r via \hfil, matching an ordinary cell) and the
// slot is then smashed to zero height and depth so it contributes nothing to its
// own row. The content box's .shift (set by assembleTabular) carries it down into
// the rows below, where the covered cells are left blank. Simplification vs. the
// real multirow package: an \hline between spanned rows still draws its full width
// across the cell (plain multirow's default behaviour; \cline is used upstream to
// carve gaps — not emulated here).
func multirowSlot(cell builtCell, innerW int) *boxNode {
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	var aligned []node
	switch cell.mralign {
	case 'r':
		aligned = []node{fil, cell.mrbox}
	case 'c':
		aligned = []node{fil, cell.mrbox, fil}
	default: // 'l', 'p' and anything else: flush left
		aligned = []node{cell.mrbox, fil}
	}
	slot := hpackSP(aligned, packTo, innerW)
	slot.height, slot.depth = 0, 0 // smash: the row's height ignores the spanning box
	return slot
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
		var slot *boxNode
		if cell.multirow {
			slot = multirowSlot(cell, innerW)
		} else {
			slot = hpackSP(cell.nodes, packTo, innerW)
		}
		rn = append(rn, kernNode{width: tabColSep}, slot, kernNode{width: tabColSep})
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
	return partialColRule(from, to, ncol, colw, hasRule, defaultRule, false, false)
}

// partialColRule builds a partial horizontal rule spanning columns from..to
// (1-based, inclusive) at the given thickness: a left kern to the start of column
// `from` followed by a rule covering those columns' \tabcolsep padding and any
// interior | rules. It backs both \cline (thickness defaultRule, no trimming) and
// booktabs' \cmidrule (\lightrulewidth, optional trimming). trimL/trimR shave a
// fixed cmidTrim off the named side(s) — booktabs' (l)/(r)/(lr) trimming, lightly
// honoured with a fixed kern so abutting \cmidrules do not touch. An empty or
// out-of-range range renders nothing (nil).
func partialColRule(from, to, ncol int, colw []int, hasRule func(int) bool, thick int, trimL, trimR bool) node {
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
	if trimL { // shave the left side and push the rule inward by the trim kern
		left += cmidTrim
		w -= cmidTrim
	}
	if trimR {
		w -= cmidTrim
	}
	if w < 0 {
		w = 0
	}
	return hpackSP([]node{kernNode{width: left}, ruleNode{width: w, height: thick}}, packNatural, 0)
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
