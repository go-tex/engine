// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements class-level two-column page layout: \documentclass[twocolumn],
// the \twocolumn / \onecolumn commands, and the two-column journal classes (revtex and
// friends) whose body is set in two columns per page.
//
// It differs from the multicols environment (multicols.go). multicols BALANCES a
// bounded block into N equal-height columns in one hbox; class two-column PAGINATES —
// each physical page carries two columns each a full \vsize tall, the galley flowing
// down the left column then the right, page after page, unbalanced except that the
// final page's columns simply end where the material runs out.
//
// Two pieces cooperate:
//
//   - The paragraph MEASURE. The body must be line-broken at the column width, not the
//     full text width. e.hsize is the measure every paragraph and display reads
//     (paragraph.go, equation.go, …), so halving it is all that is needed — but WHEN
//     matters: geometry (\usepackage[..]{geometry}, \geometry) settles e.hsize during
//     the preamble, and a class option is seen before that. applyTwoColumnMeasure
//     therefore defers the halving to the first paragraph typeset (after the preamble),
//     and a guard fires it once. Nested measures (minipage/parbox/multicols) save and
//     restore e.hsize around their own assignment, so they see the halved value as their
//     starting width, exactly as under real two-column.
//
//   - The OUTPUT routine. pagesTwoColumn slices the main vertical list into \vsize-tall
//     columns with the single-column page breaker (findPageBreak) and lays two per page.
//
// Not yet reproduced: full-width spanning of \twocolumn[...] material and \maketitle /
// abstract across both columns (they set at the column measure for now), per-column
// footnote areas, and last-page column balancing.

// applyTwoColumnMeasure halves the paragraph measure to the column width the first time
// a paragraph is set in two-column mode. Deferred to first use (not to class time) so it
// runs after all preamble geometry has settled e.hsize; the guard fires it once.
func (e *Engine) applyTwoColumnMeasure() {
	if !e.twoColumn || e.twoColApplied {
		return
	}
	if cw := (e.hsize - e.columnsep) / 2; cw > 0 {
		e.hsize = cw
	}
	e.twoColApplied = true
}

// pagesTwoColumn builds the pages of a two-column document: the main vertical list
// (already broken to the column measure) is sliced into \vsize-tall columns, two to a
// physical page, filling left then right. It reuses the single-column page breaker for
// each column and assemblePage for the page furniture.
func (e *Engine) pagesTwoColumn() []*boxNode {
	var pages []*boxNode
	list := e.mvl
	colW := e.hsize
	fullW := 2*colW + e.columnsep

	takeColumn := func(start int) ([]node, int) {
		end := e.findPageBreak(list, start)
		col := trimTrailingGlue(list[start:end])
		next := skipDiscardable(list, end)
		if next <= start {
			next = start + 1 // always make progress
		}
		return col, next
	}

	for start := 0; start < len(list); {
		left, next := takeColumn(start)
		var right []node
		if next < len(list) {
			right, next = takeColumn(next)
		}
		if len(left) > 0 || len(right) > 0 {
			pages = append(pages, e.assembleTwoColumnPage(left, right, colW, fullW, len(pages)+1))
		}
		if next >= len(list) {
			break
		}
		if len(pages) >= maxPages {
			if e.skippedCS == nil {
				e.skippedCS = map[string]int{}
			}
			e.skippedCS["gotex@pagelimit"]++
			break
		}
		start = next
	}
	return pages
}

// assembleTwoColumnPage packs the two column slices as top-anchored vboxes of the column
// measure, lays them side by side with a \columnsep gap (and a \columnseprule when set),
// and hands the full-width result to assemblePage for headers/footers/margins — with
// e.hsize temporarily the full width so that furniture spans the page.
func (e *Engine) assembleTwoColumnPage(left, right []node, colW, fullW, pageNum int) *boxNode {
	vsize := e.effectiveVsize()
	mk := func(slice []node) *boxNode {
		b := vpackSP(slice, packNatural, 0)
		b.width = colW
		return b
	}
	lb, rb := mk(left), mk(right)
	// Anchor both columns at the top: give them a common height (the taller, capped at
	// \vsize) and no depth so their tops align on the hbox baseline.
	total := lb.height + lb.depth
	if t := rb.height + rb.depth; t > total {
		total = t
	}
	if vsize > 0 && total > vsize {
		total = vsize
	}
	lb.height, lb.depth = total, 0
	rb.height, rb.depth = total, 0

	row := []node{lb}
	row = append(row, columnGap(e.columnsep, e.columnseprule, total)...)
	row = append(row, rb)
	cols := hpackSP(row, packNatural, 0)

	saved := e.hsize
	e.hsize = fullW
	page := e.assemblePage([]node{cols}, pageNum)
	e.hsize = saved
	return page
}
