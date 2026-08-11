// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the amsmath multi-line display environments align / align*
// and gather / gather*. Unlike a single equation, these stack several math lines,
// number each line independently (unless starred or \nonumber), and — for align —
// line the rows up on their & alignment points.
//
// go-tex/math renders one formula string at a time and only aligns & inside
// matrix-like environments (centred columns), which is not align's right/left
// column model. So alignment is done here at the box level: each & -separated cell
// is rendered as its own inline math box, then the cells are placed into per-column
// boxes (even columns flush right, odd columns flush left, as in amsmath's rl
// pairs) sharing one set of column widths so every row lines up. gather has a single
// centred column.

import "strings"

// alignRow is one line of an align/gather body: its & -separated cells (already
// reconstructed to math source strings), whether \nonumber suppressed its number,
// and any \label keys attached to it.
type alignRow struct {
	cells    []string
	nonumber bool
	labels   []string
}

// alignColumn maps a 0-based cell column to its horizontal alignment within the
// column: 'r' flush right, 'l' flush left, 'c' centred. align uses right/left
// alternating pairs; eqnarray uses right/centre/left triples.
type alignColumn func(j int) byte

// alignPairs is the amsmath align column rule: even columns flush right, odd flush
// left (the & sits between an rl pair).
func alignPairs(j int) byte {
	if j%2 == 0 {
		return 'r'
	}
	return 'l'
}

// eqnarrayCols is the classic eqnarray {rcl} rule: right, centre, left, repeating.
func eqnarrayCols(j int) byte {
	switch j % 3 {
	case 0:
		return 'r'
	case 1:
		return 'c'
	default:
		return 'l'
	}
}

// doAlignEnv typesets an align/eqnarray/gather family environment. numbered is false
// for the starred forms. colAlign is nil for gather (a single centred column);
// otherwise it gives each &-separated column's alignment, and rows are lined up on a
// shared set of column widths so the alignment points coincide across rows.
func (e *Engine) doAlignEnv(name string, numbered bool, colAlign alignColumn) {
	rows := e.collectAlignBody(name)
	e.endParagraph()

	type builtRow struct {
		cells    []mathNode
		nonumber bool
		labels   []string
	}
	var built []builtRow
	var colw []int
	for _, r := range rows {
		br := builtRow{nonumber: r.nonumber, labels: r.labels}
		for j, c := range r.cells {
			var m mathNode
			if strings.TrimSpace(c) != "" {
				m = e.makeMath(c, true)
			}
			br.cells = append(br.cells, m)
			if colAlign != nil {
				if j >= len(colw) {
					colw = append(colw, 0)
				}
				if m.width > colw[j] {
					colw[j] = m.width
				}
			}
		}
		built = append(built, br)
	}

	blockWidth := 0
	for _, w := range colw {
		blockWidth += w
	}
	leftPad := (e.hsize - blockWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	for _, br := range built {
		number := ""
		if numbered && !br.nonumber {
			number = e.stepEquationCounter()
			for _, k := range br.labels {
				e.setLabel(k, number)
			}
		} else {
			// Stepped or not, a \label on an unnumbered row records the current label.
			for _, k := range br.labels {
				e.setLabel(k, e.currentLabel())
			}
		}
		var row []node
		if colAlign != nil {
			row = append(row, kernNode{width: leftPad})
			for j, m := range br.cells {
				pad := colw[j] - m.width
				if pad < 0 {
					pad = 0
				}
				switch colAlign(j) {
				case 'r':
					row = append(row, kernNode{width: pad}, m)
				case 'l':
					row = append(row, m, kernNode{width: pad})
				default: // 'c'
					row = append(row, kernNode{width: pad / 2}, m, kernNode{width: pad - pad/2})
				}
			}
		} else { // gather: single centred cell
			row = append(row, e.hfil())
			if len(br.cells) > 0 {
				row = append(row, br.cells[0])
			}
		}
		row = append(row, e.hfil())
		if number != "" {
			row = append(row, e.textToHbox("("+number+")"))
		}
		e.contribute(hpackSP(row, packTo, e.hsize))
	}
}

// doMultline typesets a multline/multline* environment: one long equation split
// across lines with no alignment points. The first line is flush left, the last
// flush right, intermediate lines centred; a single number (on the last line) is
// placed when numbered.
func (e *Engine) doMultline(name string, numbered bool) {
	rows := e.collectAlignBody(name)
	e.endParagraph()

	// Flatten to one cell per row (multline has no &); render each as display math.
	var lines []mathNode
	var labels []string
	for _, r := range rows {
		src := ""
		if len(r.cells) > 0 {
			src = r.cells[0]
		}
		var m mathNode
		if strings.TrimSpace(src) != "" {
			m = e.makeMath(src, true)
		}
		lines = append(lines, m)
		labels = append(labels, r.labels...)
	}

	number := ""
	if numbered {
		number = e.stepEquationCounter()
	}
	for _, k := range labels {
		if number != "" {
			e.setLabel(k, number)
		} else {
			e.setLabel(k, e.currentLabel())
		}
	}

	last := len(lines) - 1
	for i, m := range lines {
		var row []node
		switch {
		case i == 0: // first line flush left
			row = append(row, m, e.hfil())
		case i == last: // last line flush right (+ number)
			row = append(row, e.hfil(), m)
		default: // centred
			row = append(row, e.hfil(), m, e.hfil())
		}
		if i == last && number != "" {
			row = append(row, e.hfil(), e.textToHbox("("+number+")"))
		}
		e.contribute(hpackSP(row, packTo, e.hsize))
	}
}

// hfil returns an infinitely stretchable glue (first-order fil), used to centre or
// right-flush align/gather material.
func (e *Engine) hfil() glueNode {
	return glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
}

// stepEquationCounter advances \c@equation and returns the new \theequation string.
func (e *Engine) stepEquationCounter() string {
	if m := e.eq["c@equation"]; m != nil && m.kind == mCountRef {
		e.setCount(m.code, e.count[m.code]+1, true)
	}
	return e.toksToString(e.expandList([]tok{csTok("theequation")}))
}

// currentLabel returns the current \@currentlabel expanded to a string.
func (e *Engine) currentLabel() string {
	return e.toksToString(e.expandList([]tok{csTok("@currentlabel")}))
}

// setLabel records key → val in the cross-reference table (so \ref/\eqref resolve).
func (e *Engine) setLabel(key, val string) {
	if key == "" {
		return
	}
	if e.labels == nil {
		e.labels = map[string]string{}
	}
	e.labels[key] = val
}

// collectAlignBody reads the raw tokens of an align/gather body up to \end{name},
// splitting rows at \\ and cells at & (both only at brace depth 0), reconstructing
// each cell as a math source string. \nonumber and \label are pulled out of the
// cell (executed as row metadata, not sent to the math renderer).
func (e *Engine) collectAlignBody(name string) []alignRow {
	var rows []alignRow
	var row alignRow
	var cell strings.Builder
	depth := 0
	endCell := func() {
		row.cells = append(row.cells, cell.String())
		cell.Reset()
	}
	endRow := func() {
		endCell()
		if !rowEmpty(row) {
			rows = append(rows, row)
		}
		row = alignRow{}
	}
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		switch {
		case depth == 0 && t.cs_ && t.cs == "end":
			if e.readBraceName() == name {
				endRow()
				return rows
			}
		case depth == 0 && t.cs_ && t.cs == `\`: // \\ row separator
			endRow()
		case depth == 0 && !t.cs_ && t.cat == catAlign: // & cell separator
			endCell()
		case depth == 0 && t.cs_ && t.cs == "nonumber":
			row.nonumber = true
		case depth == 0 && t.cs_ && t.cs == "label":
			row.labels = append(row.labels, e.readBraceName())
		default:
			if !t.cs_ && t.cat == catBegin {
				depth++
			} else if !t.cs_ && t.cat == catEnd {
				depth--
			}
			if t.cs_ {
				cell.WriteByte('\\')
				cell.WriteString(t.cs)
				cell.WriteByte(' ')
			} else {
				cell.WriteRune(t.ch)
			}
		}
	}
	return rows
}

// rowEmpty reports whether a row has no non-blank cell (so a trailing \\ before
// \end doesn't produce a spurious empty line).
func rowEmpty(r alignRow) bool {
	if len(r.labels) > 0 {
		return false
	}
	for _, c := range r.cells {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
