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
// Full-width frontmatter IS reproduced: a \twocolumn[...] span, and the revtex reprint
// frontmatter (title/authors/abstract, switched at \maketitle), are set across the whole
// page above the two columns; the material before the switch is a one-column region.
// Not yet reproduced: per-column footnote areas, last-page column balancing, and the real
// ltxgrid \twocolumngrid/\onecolumngrid engine for a bundled revtex .cls (the emulation
// path — the common case, since revtex is not embedded — is handled here).

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
		e.colRegions = []colRegion{{at: 0, cols: 2, colW: e.hsize}}
	}
}

// colRegion is one \onecolumn/\twocolumn span of the main vertical list: from index at
// (into e.mvl) until the next region's at, typeset in cols columns. span, when non-nil,
// is the \twocolumn[...] full-width material placed across the top of the region's first
// page (\@topnewpage), which shortens that page's columns. colW is the single-column
// measure this region's material was line-broken at — captured at creation because
// e.hsize is mutable state (a later \onecolumn restores it to full width), and reading
// it back at pagination time would use the wrong width for an earlier region.
type colRegion struct {
	at   int
	cols int
	span *boxNode
	colW int // per-column measure (full width for a one-column region, halved for two)
}

// fullWidth is the one-column measure: the width the frontmatter and any \onecolumn or
// \twocolumn[span] material is set at. Once two-column has entered, e.oneColHsize holds
// the full width even while e.hsize carries the halved column measure.
func (e *Engine) fullWidth() int {
	if e.oneColHsize > 0 {
		return e.oneColHsize
	}
	return e.hsize
}

// dblTextFloatSep is the gap between a \twocolumn[...] full-width span and the columns
// below it: the \dbltextfloatsep register when the class set it, else LaTeX's 20pt default.
func (e *Engine) dblTextFloatSep() int {
	if s := e.namedSkip("dbltextfloatsep"); s.width > 0 {
		return s.width
	}
	return 20 * unity
}

// enterTwoColumnMeasure sets e.hsize to the column width, remembering the full width
// (e.oneColHsize) for a later \onecolumn to restore. It is idempotent: the column
// measure is always derived from the stored full width, so calling it again after a
// \onecolumn restored e.hsize re-halves consistently rather than halving the halved.
func (e *Engine) enterTwoColumnMeasure() {
	if e.oneColHsize == 0 {
		e.oneColHsize = e.hsize
	}
	if cw := (e.oneColHsize - e.columnsep) / 2; cw > 0 {
		e.hsize = cw
	}
	e.twoColApplied = true
}

// typesetSpanFullWidth typesets \twocolumn[...] material at the FULL one-column width,
// regardless of whether the column measure has already been entered. It forces e.hsize
// to the full width and suppresses the deferred halving (applyTwoColumnMeasure) for the
// duration, so the span is line-broken across the whole page — \@topnewpage — not at a
// single column. The result box is capped at \textheight.
func (e *Engine) typesetSpanFullWidth(toks []tok) *boxNode {
	fullW := e.fullWidth()
	savedH, savedApplied := e.hsize, e.twoColApplied
	e.hsize = fullW
	e.twoColApplied = true // don't let applyTwoColumnMeasure halve mid-span
	span := e.typesetGroupToVbox(toks)
	e.hsize, e.twoColApplied = savedH, savedApplied
	span.width = fullW
	if v := e.effectiveVsize(); span.height+span.depth > v { // \@topnewpage caps at \textheight
		span.height = v - span.depth
	}
	return span
}

// switchToTwoColumn records the boundary at which the body switches to two columns:
// the material typeset so far (frontmatter) stays a full-width one-column region, and a
// new two-column region begins at the current position. span, when non-nil, is the
// \twocolumn[...] full-width block placed across the top of the region's first page.
// It captures the halved column measure in the region so pagination uses the right
// width even after a later \onecolumn restores e.hsize.
func (e *Engine) switchToTwoColumn(span *boxNode) {
	if len(e.colRegions) == 0 && len(e.mvl) > 0 {
		// The material so far (revtex frontmatter, a \maketitle block) is one-column
		// at the full width.
		e.colRegions = append(e.colRegions, colRegion{at: 0, cols: 1, colW: e.fullWidth()})
	}
	e.enterTwoColumnMeasure()
	e.twoColumn = true
	e.colRegions = append(e.colRegions, colRegion{at: len(e.mvl), cols: 2, span: span, colW: e.hsize})
}

// startTwoColumn implements the \twocolumn command: like real LaTeX it \clearpage's
// (here, a region boundary starts a fresh page) and switches to two columns. An optional
// [span] is typeset full-width and placed across the top of the region's first page
// (\@topnewpage), shortening that page's columns.
func (e *Engine) startTwoColumn() {
	spanToks, hasSpan := e.scanOptBracketToks()
	var span *boxNode
	if hasSpan && len(spanToks) > 0 {
		span = e.typesetSpanFullWidth(spanToks)
	}
	e.switchToTwoColumn(span)
}

// startOneColumn implements the \onecolumn command: \clearpage (region boundary) and
// switch back to a single full-width column, restoring the measure two-column saved.
func (e *Engine) startOneColumn() {
	if len(e.colRegions) == 0 && len(e.mvl) > 0 {
		e.colRegions = append(e.colRegions, colRegion{at: 0, cols: 2, colW: e.hsize})
	}
	if e.oneColHsize > 0 {
		e.hsize = e.oneColHsize
	}
	e.colRegions = append(e.colRegions, colRegion{at: len(e.mvl), cols: 1, colW: e.fullWidth()})
}

// pagesByRegion paginates each \onecolumn/\twocolumn region of the main vertical list in
// its own column mode and concatenates the pages (continuous numbering). Regions are
// page-aligned because both commands \clearpage.
func (e *Engine) pagesByRegion() []*boxNode {
	regs := e.colRegions
	if regs[0].at > 0 { // material before the first switch is one-column
		regs = append([]colRegion{{at: 0, cols: 1, colW: e.fullWidth()}}, regs...)
	}
	savedH := e.hsize
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
		// Restore the measure this region was set at, so its page furniture (header,
		// footnote rule) spans the right width — e.hsize itself carries whatever the
		// last column switch left it at, which is not this region's width.
		if r.colW > 0 {
			e.hsize = r.colW
		}
		if r.cols >= 2 {
			colW := r.colW
			if colW <= 0 {
				colW = (e.fullWidth() - e.columnsep) / 2
			}
			pages = append(pages, e.paginateTwoColList(slice, colW, r.span, len(pages))...)
		} else {
			pages = append(pages, e.paginateSingleList(slice, len(pages))...)
		}
	}
	e.hsize = savedH
	return pages
}

// paginateTwoColList slices one vertical list (already broken to the column measure)
// into \vsize-tall columns, two to a physical page, filling left then right, numbering
// from pageOffset+1. It reuses the single-column page breaker for each column and
// assemblePage for the page furniture. colW is the region's column measure (captured
// when the region was typeset), not e.hsize, which a later \onecolumn may have changed.
func (e *Engine) paginateTwoColList(list []node, colW int, span *boxNode, pageOffset int) []*boxNode {
	var pages []*boxNode
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
		// A \twocolumn[...] span sits full-width atop the region's FIRST page and
		// shortens that page's columns by its height + \dbltextfloatsep. Real LaTeX
		// sets \vsize\@colht for the page; here we temporarily reduce e.vsize so the
		// shared page breaker fills the columns to the reduced height.
		var pageSpan *boxNode
		savedVsize := e.vsize
		if len(pages) == 0 && span != nil {
			pageSpan = span
			if reduced := e.effectiveVsize() - (span.height + span.depth) - e.dblTextFloatSep(); reduced > 0 {
				e.vsize = reduced
			}
		}
		left, next := takeColumn(start)
		var right []node
		if next < len(list) {
			right, next = takeColumn(next)
		}
		e.vsize = savedVsize
		if len(left) > 0 || len(right) > 0 || pageSpan != nil {
			pages = append(pages, e.assembleTwoColumnPage(pageSpan, left, right, colW, fullW, pageOffset+len(pages)+1))
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
func (e *Engine) assembleTwoColumnPage(span *boxNode, left, right []node, colW, fullW, pageNum int) *boxNode {
	vsize := e.effectiveVsize()
	if span != nil { // the columns share the height left below the span
		if r := vsize - (span.height + span.depth) - e.dblTextFloatSep(); r > 0 {
			vsize = r
		}
	}
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

	// A \twocolumn[...] span sits full-width above the two columns, separated by
	// \dbltextfloatsep.
	content := []node{cols}
	if span != nil {
		content = []node{span, glueNode{spec: glueSpec{width: e.dblTextFloatSep()}}, cols}
	}

	saved := e.hsize
	e.hsize = fullW
	page := e.assemblePage(content, pageNum)
	e.hsize = saved
	return page
}
