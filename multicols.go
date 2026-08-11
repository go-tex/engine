// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the multicols environment:
//
//	\begin{multicols}{N}[preamble] … \end{multicols}
//
// It typesets the enclosed material in N balanced columns. The body is first set
// as one tall single-column vbox at the column measure, then that vertical list is
// sliced into N pieces of roughly equal height and the pieces are placed side by
// side in an hbox, separated by \columnsep (and, when \columnseprule>0, a vertical
// rule centred in the gap).
//
// Column measure. Each column is (\hsize − (N−1)·\columnsep) / N wide, so the N
// columns plus the N−1 inter-column gaps fill \hsize (integer truncation of the
// division may leave a few sp of slack at the right edge, which is harmless).
//
// Balancing (best-effort). Real LaTeX multicol balances iteratively, trying column
// heights until the shortest arrangement is found. This implementation balances in
// a single forward pass: for each column it targets (height-of-remaining-material /
// columns-still-to-fill) and takes the largest prefix of the remaining vertical
// list that fits at a legal breakpoint (splitVList). Because the target is recomputed
// against the material that is actually left, rounding does not accumulate and the
// last column is never starved. It is not guaranteed optimal, but for ordinary running
// text the columns come out close in height. A single item taller than the target is
// placed alone in its column and the pass continues (splitVList always keeps at least
// the first item of a slice), so nothing overflows catastrophically.
//
// Preamble. The optional [preamble] is spanning material typeset full-width and
// contributed above the columns (as in real multicol, where it is the single-column
// text that precedes the balanced block).
//
// Defaults. \columnsep = 10pt and \columnseprule = 0pt, matching LaTeX. Both are
// assignable primitives (\columnsep=<dimen>, \columnseprule=<dimen>).
//
// Degenerate N. N ≤ 1 is normal single-column typesetting: the preamble and body
// tokens are simply handed back to the main loop at the current \hsize.

// doMulticols implements \begin{multicols}{N}[preamble] … \end{multicols}.
func (e *Engine) doMulticols() {
	n := e.readBraceInt()
	preamble, _ := e.scanOptBracketToks()
	body := e.collectEnvBody("multicols")

	if n <= 1 {
		// Degenerate: typeset preamble then body as ordinary single-column text.
		e.push(append(append([]tok{}, preamble...), body...))
		return
	}

	// Spanning preamble: full-width single-column material above the columns.
	if len(preamble) > 0 {
		span := e.typesetGroupToVbox(append([]tok{csTok("noindent")}, preamble...))
		span.width = e.hsize
		e.contribute(span)
	}

	colWidth := (e.hsize - (n-1)*e.columnsep) / n

	// Typeset the whole body once, broken to the column measure.
	savedHsize := e.hsize
	e.hsize = colWidth
	whole := e.typesetGroupToVbox(append([]tok{csTok("noindent")}, body...))
	e.hsize = savedHsize

	// Slice the single-column vertical list into N balanced pieces and pack each
	// as a column vbox fixed to the column measure.
	pieces := balanceVList(whole.list, n)
	cols := make([]*boxNode, n)
	maxTotal := 0
	for i, p := range pieces {
		b := vpackSP(p, packNatural, 0)
		b.width = colWidth
		cols[i] = b
		if t := b.height + b.depth; t > maxTotal {
			maxTotal = t
		}
	}
	// Anchor every column at the top by giving them a common height (the tallest
	// column's total) and no depth, so their tops align on the hbox baseline; any
	// slack from balancing shows as trailing whitespace at a column's foot.
	for _, b := range cols {
		b.height, b.depth = maxTotal, 0
	}

	// Lay the columns side by side, separated by \columnsep (with a centred rule
	// of \columnseprule when that is positive).
	var row []node
	for i, b := range cols {
		if i > 0 {
			row = append(row, columnGap(e.columnsep, e.columnseprule, maxTotal)...)
		}
		row = append(row, b)
	}
	e.contribute(hpackSP(row, packNatural, 0))
}

// balanceVList slices a vertical list into n pieces of roughly equal height. For
// each piece it targets (remaining height / columns still to fill) and takes the
// largest prefix that fits at a legal break; the last column takes whatever is
// left. See the file comment for the best-effort caveat.
func balanceVList(list []node, n int) [][]node {
	pieces := make([][]node, 0, n)
	remaining := list
	for c := 0; c < n; c++ {
		left := n - c
		if left == 1 {
			pieces = append(pieces, remaining)
			remaining = nil
			continue
		}
		target := vlistHeight(remaining) / left
		piece, rest := splitVList(remaining, target)
		pieces = append(pieces, piece)
		remaining = rest
	}
	return pieces
}

// vlistHeight is the natural height of a vertical list (sum of contributions).
func vlistHeight(list []node) int {
	h := 0
	for _, n := range list {
		h += vContribution(n)
	}
	return h
}

// columnGap builds the material between two columns: a sep-wide gap, occupied by a
// centred vertical rule of thickness rule and height total when rule>0. The pieces
// always sum to exactly sep in width.
func columnGap(sep, rule, total int) []node {
	if rule <= 0 {
		return []node{kernNode{width: sep}}
	}
	left := (sep - rule) / 2
	right := sep - rule - left
	return []node{
		kernNode{width: left},
		ruleNode{width: rule, height: total, depth: 0},
		kernNode{width: right},
	}
}
