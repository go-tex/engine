// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \halign, TeX's horizontal alignment (tables). The
// preamble gives each column a template split at # into "before" and "after"
// token lists; rows are entries separated by & and ended by \cr. Each column's
// width is the maximum of its cells' natural widths, every cell is then repacked
// to that width, and the rows are stacked into a vbox. Simplifications for now:
// no \tabskip glue, no \omit/\span/\noalign, and a cell may not contain a bare
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
	var rows [][][]node
	for {
		row, ended := e.parseRow(templates)
		if row != nil {
			rows = append(rows, row)
		}
		if ended {
			break
		}
	}
	e.contribute(e.assembleAlignment(templates, rows))
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
func (e *Engine) assembleAlignment(cols []colTemplate, rows [][][]node) *boxNode {
	ncol := len(cols)
	colw := make([]int, ncol)
	for _, row := range rows {
		for j, cell := range row {
			if j < ncol {
				if w := hpackSP(cell, packNatural, 0).width; w > colw[j] {
					colw[j] = w
				}
			}
		}
	}
	var vlist []node
	for _, row := range rows {
		var rowNodes []node
		for j, cell := range row {
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
