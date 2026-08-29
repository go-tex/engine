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
	e.enterTwoColumnMeasure()
	// A class-option two-column document has no explicit \twocolumn, so seed the whole
	// document as one two-column region here (the commands add further regions).
	if len(e.colRegions) == 0 {
		e.colRegions = []colRegion{{at: 0, cols: 2}}
	}
}

// colRegion is one \onecolumn/\twocolumn span of the main vertical list: from index at
// (into e.mvl) until the next region's at, typeset in cols columns.
type colRegion struct {
	at   int
	cols int
}

// enterTwoColumnMeasure halves e.hsize to the column width, saving the full width for a
// later \onecolumn to restore. The guard fires it once; \twocolumn after \onecolumn
// re-enters through startTwoColumn, which halves the (restored) full width again.
func (e *Engine) enterTwoColumnMeasure() {
	e.oneColHsize = e.hsize
	if cw := (e.hsize - e.columnsep) / 2; cw > 0 {
		e.hsize = cw
	}
	e.twoColApplied = true
}

// startTwoColumn implements the \twocolumn command: like real LaTeX it \clearpage's
// (here, a region boundary starts a fresh page) and switches to two columns. The
// optional [span] full-width material (\@topnewpage) is not yet reproduced.
func (e *Engine) startTwoColumn() {
	e.scanOptBracketToks() // gobble the optional [span] for now
	if len(e.colRegions) == 0 && len(e.mvl) > 0 {
		e.colRegions = append(e.colRegions, colRegion{at: 0, cols: 1}) // material so far was one-column
	}
	if !e.twoColumn || !e.twoColApplied || e.hsize == e.oneColHsize {
		e.enterTwoColumnMeasure()
	}
	e.twoColumn = true
	e.colRegions = append(e.colRegions, colRegion{at: len(e.mvl), cols: 2})
}

// startOneColumn implements the \onecolumn command: \clearpage (region boundary) and
// switch back to a single full-width column, restoring the measure two-column saved.
func (e *Engine) startOneColumn() {
	if len(e.colRegions) == 0 && len(e.mvl) > 0 {
		e.colRegions = append(e.colRegions, colRegion{at: 0, cols: 2})
	}
	if e.oneColHsize > 0 {
		e.hsize = e.oneColHsize
	}
	e.colRegions = append(e.colRegions, colRegion{at: len(e.mvl), cols: 1})
}

// pagesByRegion paginates each \onecolumn/\twocolumn region of the main vertical list in
// its own column mode and concatenates the pages (continuous numbering). Regions are
// page-aligned because both commands \clearpage.
func (e *Engine) pagesByRegion() []*boxNode {
	regs := e.colRegions
	if regs[0].at > 0 { // material before the first switch is one-column
		regs = append([]colRegion{{at: 0, cols: 1}}, regs...)
	}
	var pages []*boxNode
	for i, r := range regs {
		end := len(e.mvl)
		if i+1 < len(regs) {
			end = regs[i+1].at
		}
		if r.at >= end {
			continue
		}
		slice := e.mvl[r.at:end]
		if r.cols >= 2 {
			pages = append(pages, e.paginateTwoColList(slice, len(pages))...)
		} else {
			pages = append(pages, e.paginateSingleList(slice, len(pages))...)
		}
	}
	return pages
}

// paginateTwoColList slices one vertical list (already broken to the column measure)
// into \vsize-tall columns, two to a physical page, filling left then right, numbering
// from pageOffset+1. It reuses the single-column page breaker for each column and
// assemblePage for the page furniture.
func (e *Engine) paginateTwoColList(list []node, pageOffset int) []*boxNode {
	var pages []*boxNode
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
			pages = append(pages, e.assembleTwoColumnPage(left, right, colW, fullW, pageOffset+len(pages)+1))
		}
		if next >= len(list) {
			break
		}
		if pageOffset+len(pages) >= maxPages {
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
