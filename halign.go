// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \halign, TeX's horizontal alignment (tables). The
// preamble gives each column a template split at # into "before" and "after"
// token lists; rows are entries separated by & and ended by \cr. Each column's
// width is the maximum of its cells' natural widths, every cell is then repacked
// to that width, and the rows are stacked into a vbox. Simplifications for now:
// no \tabskip glue, no \omit/\span, and a cell may not contain a bare
// top-level {…} group (use \hbox{…} for grouped content) — matching the box
// builder's brace handling.

// colTemplate is one preamble column: text before and after the entry (#).
type colTemplate struct{ before, after []tok }

// doHalign parses and lays out an \halign{…}. The result vbox is contributed to
// the current vertical list.
func (e *Engine) doHalign() {
	e.endParagraph()
	e.skipOptSpace()
	if t, ok := e.getXToken(); !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return
	}
	templates := e.parsePreamble()
	var items []alignItem
	for {
		for {
			vmat, ok := e.scanNoalign()
			if !ok {
				break
			}
			items = append(items, alignItem{vmat: vmat})
		}
		row, ended := e.parseRow(templates)
		if row != nil {
			items = append(items, alignItem{row: row})
		}
		if ended {
			break
		}
	}
	e.contribute(e.assembleAlignment(templates, items))
}

// alignItem is one thing in an alignment's vertical list: either a row of cells
// or the material a \noalign contributed between two rows. Exactly one field is
// set.
type alignItem struct {
	row  [][]node
	vmat []node
}

// scanNoalign consumes a \noalign{…} if one is next, returning the vertical list
// its group builds.
//
// tex.web §785: \noalign may appear only where a row could begin — at the very
// start of an alignment or just after a \cr — and its argument is VERTICAL mode
// material that goes into the enclosing vertical list, between the rows. TikZ
// stacks the lines of a node with align= exactly this way, one \halign whose
// rows are the lines and whose \noalign{\vskip …} is the leading between them
// (tikz.code.tex, \tikz@align@end@check), so a node reading
//
//	\node [align=center] {one \\ two};
//
// used to leave \noalign undefined. That is not merely a lost skip: the group
// after it then reached parseRow as a bare top-level {…}, which a cell may not
// hold, and the rest of the document went with it.
func (e *Engine) scanNoalign() ([]node, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil, false
	}
	if !t.cs_ || t.cs != "noalign" {
		e.back(t)
		return nil, false
	}
	e.skipOptSpace()
	b, ok := e.getXToken()
	if !ok {
		return nil, true
	}
	if c, isChar := e.implicitChar(b); isChar {
		b = c // \bgroup opens the group just as { does
	}
	if b.cs_ || b.cat != catBegin {
		e.back(b) // \noalign without a group: nothing to contribute
		return nil, true
	}
	// TeX makes the material a group of its own (tex.web §785 opens one), so a
	// font or glue change inside a \noalign stops at it.
	e.beginGroupKind(boxGroup)
	list := e.buildVBoxList()
	e.endGroup()
	return list, true
}

// parsePreamble reads the template row (up to \cr), returning one entry per
// column split at the # placeholder.
func (e *Engine) parsePreamble() []colTemplate {
	var cols []colTemplate
	var cur colTemplate
	seenHash := false
	flush := func() {
		cols = append(cols, cur)
		cur = colTemplate{}
		seenHash = false
	}
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		switch {
		case t.cs_ && (t.cs == "cr" || t.cs == "crcr"):
			flush()
			return cols
		case !t.cs_ && t.cat == catAlign:
			flush()
		case !t.cs_ && t.cat == catParam:
			seenHash = true
		case seenHash:
			cur.after = append(cur.after, t)
		default:
			cur.before = append(cur.before, t)
		}
	}
	return cols
}

// parseRow reads one row's cells (separated by &, ended by \cr). ended is true
// when the alignment's closing brace or end of input was reached.
func (e *Engine) parseRow(cols []colTemplate) ([][]node, bool) {
	var cells [][]node
	var content []tok
	col, depth := 0, 0
	build := func() {
		var toks []tok
		if col < len(cols) {
			toks = append(toks, cols[col].before...)
		}
		toks = append(toks, content...)
		if col < len(cols) {
			toks = append(toks, cols[col].after...)
		}
		cells = append(cells, e.buildCellHList(toks))
		content = nil
		col++
	}
	for {
		t, ok := e.getXToken()
		if !ok {
			if len(content) > 0 || col > 0 {
				build()
			}
			return cells, true
		}
		switch {
		case depth == 0 && t.cs_ && (t.cs == "cr" || t.cs == "crcr"):
			build()
			return cells, false
		case depth == 0 && !t.cs_ && t.cat == catEnd: // alignment's closing }
			if len(content) > 0 || col > 0 {
				build()
			}
			return cells, true
		case depth == 0 && !t.cs_ && t.cat == catAlign:
			build()
		default:
			if !t.cs_ && t.cat == catBegin {
				depth++
			} else if !t.cs_ && t.cat == catEnd {
				depth--
			}
			content = append(content, t)
		}
	}
}

// buildCellHList builds a cell's horizontal list from its full token sequence
// (template before + entry + after) in isolation.
func (e *Engine) buildCellHList(toks []tok) []node {
	saved := e.noBase
	e.noBase = true
	seq := make([]tok, 0, len(toks)+1)
	seq = append(seq, toks...)
	seq = append(seq, chTok('}', catEnd)) // sentinel to terminate buildBoxList
	e.push(seq)
	// The sentinel closes a real group, as the cell's own braces would: TeX makes
	// every alignment entry a group (tex.web §791 — the u-part and v-part of a
	// template are inserted inside braces), so a font or colour switch in a cell
	// stops at the cell. Without the group the sentinel was a STRAY brace, which
	// the stomach reports the moment any \begingroup is open — and \begin{env} now
	// opens one.
	e.beginGroupKind(boxGroup)
	list := e.buildBoxList()
	e.endGroup()
	e.noBase = saved
	return list
}

// assembleAlignment computes column widths and stacks the repacked rows into a
// vbox.
func (e *Engine) assembleAlignment(cols []colTemplate, items []alignItem) *boxNode {
	ncol := len(cols)
	colw := make([]int, ncol)
	for _, it := range items {
		for j, cell := range it.row {
			if j < ncol {
				if w := hpackSP(cell, packNatural, 0).width; w > colw[j] {
					colw[j] = w
				}
			}
		}
	}
	var vlist []node
	// The rows are a vertical list, so they take interline glue like any other
	// (tex.web §679, interlineGlue in paragraph.go). Stacked at their natural
	// height they grew 6.63pt per row against the reference's 12.00pt — the
	// height of a line of text rather than \baselineskip.
	//
	// No paper in the 200-document arXiv reference corpus exercises this: all 282
	// of its \halign occurrences sit in .sty/.cls files, which this engine
	// substitutes rather than executes, and none in a document body. The fix rests
	// on the direct witness above, not on a corpus measurement, which is 0 pages,
	// 0 PDFs and 0 ink changed.
	prevDepth := ignoreDepth
	for _, it := range items {
		if it.row == nil {
			// \noalign material joins the vertical list as it stands. Glue and
			// penalties do not change prev_depth, and a box inside it already took
			// its own interline glue while buildVBoxList ran, so the running depth
			// is left alone here.
			vlist = append(vlist, it.vmat...)
			continue
		}
		var rowNodes []node
		for j, cell := range it.row {
			width := 0
			if j < ncol {
				width = colw[j]
			}
			rowNodes = append(rowNodes, hpackSP(cell, packTo, width))
		}
		b := hpackSP(rowNodes, packNatural, 0)
		if g, ok := e.interlineGlue(prevDepth, b.height); ok {
			vlist = append(vlist, g)
		}
		vlist = append(vlist, b)
		prevDepth = b.depth
	}
	return vpackSP(vlist, packNatural, 0)
}
