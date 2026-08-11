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

// doAlignEnv typesets an align/align*/gather/gather* environment. numbered is false
// for the starred forms; aligned is true for align (split at &), false for gather.
func (e *Engine) doAlignEnv(name string, numbered, aligned bool) {
	rows := e.collectAlignBody(name)
	e.endParagraph()

	// Render every cell to a math box and remember per-column widths.
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
			if aligned {
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

	quad := e.mathSize() * unity // 1em inter-pair gap
	blockWidth := 0
	if aligned {
		for j, w := range colw {
			blockWidth += w
			if j%2 == 1 && j != len(colw)-1 {
				blockWidth += quad // gap after each rl pair, except the last
			}
		}
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
		if aligned {
			row = append(row, kernNode{width: leftPad})
			for j, m := range br.cells {
				pad := colw[j] - m.width
				if j%2 == 0 { // even column: flush right
					if pad > 0 {
						row = append(row, kernNode{width: pad})
					}
					row = append(row, m)
				} else { // odd column: flush left
					row = append(row, m)
					if pad > 0 {
						row = append(row, kernNode{width: pad})
					}
					if j != len(br.cells)-1 {
						row = append(row, kernNode{width: quad})
					}
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
