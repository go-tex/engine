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

// spanNode is a full-width band (a figure* / table* double-column float) sitting in the
// main vertical list. In two-column mode it interrupts the two-column flow and is placed
// across the top of a page, spanning BOTH columns — the same shape as a \twocolumn[span]
// or the revtex frontmatter span, but arising mid-flow rather than at a region boundary.
// The two-column pager (paginateTwoColList) lifts every spanNode out of the galley and
// places its box atop the page whose flow position it reaches; ordinary column material
// never contains one, so the packers treat it defensively as its inner box.
type spanNode struct {
	box *boxNode
	// floatPageOnly is set when the float's placement was [p] with no top bit: LaTeX may
	// then place it only on a float page, never at the top of a text page. The pager keeps
	// such a band out of a text-page top and lets it collect onto a band-only page.
	floatPageOnly bool
}

func (spanNode) isNode() {}

// placementBits returns the placement letters (h/t/b/p, and the ! override stripped) of a
// float's optional [placement] argument, lower-cased. An empty result means the default.
func placementBits(toks []tok) string {
	var b []byte
	for _, t := range toks {
		if t.cs_ {
			continue
		}
		switch t.ch {
		case 'h', 'H', 't', 'b', 'p':
			c := t.ch
			if c == 'H' {
				c = 'h'
			}
			b = append(b, byte(c))
		}
	}
	return string(b)
}

// floatPageOnly reports whether a placement string sends a double-column float to a float
// page only — [p] with no top ('t') or here ('h') bit. LaTeX supports only t and p for a
// double-column float (never a page bottom), so a b-only spec still defaults to top/page.
func floatPageOnly(place string) bool {
	if place == "" {
		return false
	}
	hasP, hasTopOrHere := false, false
	for i := 0; i < len(place); i++ {
		switch place[i] {
		case 'p':
			hasP = true
		case 't', 'h':
			hasTopOrHere = true
		}
	}
	return hasP && !hasTopOrHere
}

// pendingBand is a full-width band awaiting placement, tagged with the number of flowing
// (non-span) nodes that precede it: the band is placed across the top of the page whose
// flow position first reaches flowIndex.
type pendingBand struct {
	flowIndex     int
	box           *boxNode
	floatPageOnly bool
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

// setTextWidth assigns \textwidth. \textwidth is the width of the whole text block —
// the FULL one-column width, which in two-column mode spans both columns and the gutter,
// NOT the column measure e.hsize. Before the two-column measure has been entered
// (oneColHsize == 0) the two coincide and the standard classes size the text block by
// assigning \textwidth, so the assignment sets e.hsize (the paragraph measure) exactly as
// the old \let\textwidth\hsize did. Once two-column has split the measure into columns
// (oneColHsize holds the full width) an assignment sets that remembered full width and
// leaves the column measure alone.
func (e *Engine) setTextWidth(v int, global bool) {
	if e.oneColHsize > 0 {
		e.oneColHsize = v
		return
	}
	e.setEngineDimen(saveHsize, &e.hsize, v, global)
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

// doDblFloat implements \begin{figure*}/\begin{table*} — the \@dblfloat double-column
// float — via the internal hook \gotex@dblfloat{figure|table}. Under the two-column
// opt-in (twoColumnOptIn) the float body is typeset at the FULL text width and contributed
// as a spanNode, so the pager places it across the top of a page spanning both columns (a
// full-width band). Otherwise — one column, or a live standard-class two-column document
// with the band still gated — it degrades to the unstarred one-column float.
//
// captype is "figure" or "table"; the environment name is captype+"*".
func (e *Engine) doDblFloat(captype string) {
	// Consume the optional [placement] that follows \@dblfloat{type}. The one-column
	// \@float path reads it through \@ifnextchar, but a band collects its body verbatim,
	// so an unread [t]/[htbp] would land in the galley and typeset as a literal "[t]" atop
	// the figure. Standard LaTeX places a double-column float only at a page top or on a
	// float page (t/p — never a page bottom), so the bits are recorded but do not force a
	// bottom band; they gate deferral to a float page (see the pager).
	posToks, _ := e.scanOptBracketToks()
	place := placementBits(posToks)
	if !e.twoColumn || !twoColumnOptIn() {
		// Not spanning here: either one-column, or a LIVE standard-class two-column
		// document (article/report/book [twocolumn]) with the band still gated. Set the
		// unstarred float via \@float — \@dblfloat's historical behaviour — opening the
		// float group here; the environment's \end (\end@dblfloat = \end@float, or
		// \endfigure/\endtable) closes it.
		//
		// The full-width band is correct rendering (a figure*/table* DOES span both
		// columns), but it floats to a page top whose position does not yet track the
		// reference's float placement: on a live [twocolumn] article (2601.20606) it shifts
		// the layout enough to regress the position proxy (+2.2) even while rendering the
		// wide table more faithfully. So the band is part of the gated two-column bring-up
		// (GOTEX_TWOCOLUMN / revtex reprint), and the live standard two-column path keeps
		// the historical inline float until a fuller float-placement pass lands.
		e.push(append([]tok{csTok("@float")}, braceNameToks(captype)...))
		return
	}
	e.applyTwoColumnMeasure() // ensure the column measure and the seed region exist
	// Collect the float body (consuming its \end{figure*}/\end{table*} and closing the
	// environment group), then typeset \figure … \endfigure at the full text width so the
	// float's own centering, caption and counters run exactly as in one column, but across
	// the whole page. typesetSpanFullWidth caps it at \textheight and sets its width.
	body := e.collectEnvBody(captype + "*")
	toks := make([]tok, 0, len(body)+3)
	toks = append(toks, csTok(captype))
	// A figure*/table* body is set at the full text width. Its first paragraph — the
	// row of \includegraphics that a multi-panel figure opens with — carries \parindent,
	// and at the full width two side-by-side 0.48\textwidth panels plus that indent
	// overflow the line by a hair, so the second panel wraps and a 2x2 grid cascades into
	// three or four rows, inflating the band to most of a page. Real LaTeX packs these
	// compactly (a full-width figure*'s panels sit side by side); \@parboxrestore-style
	// resets zero the indent inside float boxes. Zero it here so the grid packs as it does
	// under TeXLive, keeping the band its true height.
	toks = append(toks, csTok("parindent"), chTok('0', catOther), chTok('p', catLetter), chTok('t', catLetter))
	toks = append(toks, body...)
	toks = append(toks, csTok("end"+captype))
	band := e.typesetSpanFullWidth(toks)
	if band == nil || band.height+band.depth == 0 {
		return // an empty float band reserves nothing
	}
	e.contribute(spanNode{box: band, floatPageOnly: floatPageOnly(place)})
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
//
// The list may carry full-width bands (spanNode — a figure*/table* double-column float,
// or the region's own \twocolumn[span]/frontmatter span passed in span). Those are lifted
// out of the flowing column material and placed across the top of the page whose flow
// position they reach, spanning both columns; the remaining material flows in two columns
// below and around them.
func (e *Engine) paginateTwoColList(list []node, colW int, span *boxNode, pageOffset int) []*boxNode {
	var flow []node
	var bands []pendingBand
	if span != nil { // the region span heads the first page
		bands = append(bands, pendingBand{flowIndex: 0, box: span})
	}
	for _, n := range list {
		if sn, ok := n.(spanNode); ok {
			if sn.box != nil {
				bands = append(bands, pendingBand{flowIndex: len(flow), box: sn.box, floatPageOnly: sn.floatPageOnly})
			}
			continue
		}
		flow = append(flow, n)
	}
	return e.paginateTwoColFlow(flow, bands, colW, pageOffset)
}

// paginateTwoColFlow paginates the flowing (span-free) column material into two-column
// pages, placing each pending band full-width across the top of the page whose flow
// position first reaches its flowIndex. Bands are ordered; a band met mid-page rides to
// the top of the next page (top-float placement), so pages stay full while every band
// lands on the page and spans both columns.
func (e *Engine) paginateTwoColFlow(list []node, bands []pendingBand, colW, pageOffset int) []*boxNode {
	var pages []*boxNode
	fullW := 2*colW + e.columnsep
	bi := 0
	var pending []*boxNode // bands deferred from an earlier page (did not fit its top)

	// takeCols fills the two columns from start with the height the bands (reserve) leave;
	// it returns the two slices and the flow index the page ends at. A band-heavy page
	// (reserve leaving under a tenth of the height) carries no column text.
	takeCols := func(start, reserve int) (left, right []node, next int) {
		next = start
		colVsize := e.effectiveVsize() - reserve
		if start >= len(list) || colVsize <= e.effectiveVsize()/10 {
			return nil, nil, start
		}
		savedVsize := e.vsize
		e.vsize = colVsize
		take := func(from int) ([]node, int) {
			end := e.findPageBreak(list, from)
			col := trimTrailingGlue(list[from:end])
			to := skipDiscardable(list, end)
			if to <= from {
				to = from + 1 // always make progress
			}
			return col, to
		}
		left, next = take(start)
		if next < len(list) {
			right, next = take(next)
		}
		e.vsize = savedVsize
		return left, right, next
	}
	for start := 0; ; {
		// Bands whose anchor lies at or before this page top were reached on an earlier
		// page (deferred because its top was full); they wait in pending.
		for bi < len(bands) && bands[bi].flowIndex <= start {
			pending = append(pending, bands[bi].box)
			bi++
		}
		if start >= len(list) && len(pending) == 0 && bi >= len(bands) {
			break // nothing left to place
		}

		fullVsize := e.effectiveVsize()
		// Place as many deferred bands atop this page as fit (always at least one), so a
		// run of figure*/table* floats spreads down successive pages rather than piling
		// onto one overfull page.
		var pageBands []*boxNode
		for len(pending) > 0 {
			need := pending[0].height + pending[0].depth + e.dblTextFloatSep()
			if len(pageBands) > 0 && e.bandsReserve(pageBands)+need > fullVsize {
				break
			}
			pageBands = append(pageBands, pending[0])
			pending = pending[1:]
		}

		// Tentatively fill the columns to learn this page's flow extent, then float up any
		// band whose anchor falls WITHIN this page (a [t] top float rides to the top of the
		// page its anchor sits on). Re-fill once with the added bands' reduced height.
		left, right, next := takeCols(start, e.bandsReserve(pageBands))
		grew := false
		for bi < len(bands) && bands[bi].flowIndex < next {
			if bands[bi].floatPageOnly {
				// [p]: never rides atop a text page — defer to a band-only (float) page.
				pending = append(pending, bands[bi].box)
				bi++
				continue
			}
			need := bands[bi].box.height + bands[bi].box.depth + e.dblTextFloatSep()
			if len(pageBands) > 0 && e.bandsReserve(pageBands)+need > fullVsize {
				break // no room atop this page; a later page's top takes it
			}
			pageBands = append(pageBands, bands[bi].box)
			bi++
			grew = true
		}
		if grew {
			left, right, next = takeCols(start, e.bandsReserve(pageBands))
		}

		if len(left) > 0 || len(right) > 0 || len(pageBands) > 0 {
			pages = append(pages, e.assembleTwoColumnPage(pageBands, left, right, colW, fullW, pageOffset+len(pages)+1))
		}
		if pageOffset+len(pages) >= maxPages {
			if e.skippedCS == nil {
				e.skippedCS = map[string]int{}
			}
			e.skippedCS["gotex@pagelimit"]++
			break
		}
		if next == start && len(pending) == 0 && bi >= len(bands) {
			// No column progress. That does NOT mean the document is finished: it
			// happens when the bands placed atop this page leave the columns less
			// than a tenth of the page (takeCols declines to fill a sliver), and the
			// text after them still has to go somewhere. Breaking here DISCARDED it —
			// on corpus paper 2403.13798 the loop stopped at node 1182 of 1702 and
			// 229 typeset line boxes never reached a page, the document ending
			// mid-section. Five of the 157 corpus papers lost content this way, one
			// of them a third of its text.
			if start >= len(list) {
				break // genuinely nothing left
			}
			if len(pageBands) > 0 {
				continue // the bands took this page; the next one has the full height
			}
			// Neither columns nor bands could take anything and material remains:
			// force one item through so the loop cannot spin. An overfull column is
			// visible and wrong in a way a reader can see; silent truncation is not.
			left, right, next = takeCols(start+1, 0)
			left = append([]node{list[start]}, left...)
			pages = append(pages, e.assembleTwoColumnPage(nil, left, right, colW, fullW, pageOffset+len(pages)+1))
			start = next
			continue
		}
		start = next
	}
	return pages
}

// bandsReserve is the vertical space a stack of full-width bands takes atop a page: each
// band's height + depth plus a \dbltextfloatsep gap to the material below it.
func (e *Engine) bandsReserve(bands []*boxNode) int {
	total := 0
	for _, b := range bands {
		total += b.height + b.depth + e.dblTextFloatSep()
	}
	return total
}

// assembleTwoColumnPage packs the two column slices as top-anchored vboxes of the column
// measure, lays them side by side with a \columnsep gap (and a \columnseprule when set),
// stacks any full-width bands above them (each separated by \dbltextfloatsep), and hands
// the full-width result to assemblePage for headers/footers/margins — with e.hsize
// temporarily the full width so that furniture spans the page.
func (e *Engine) assembleTwoColumnPage(bands []*boxNode, left, right []node, colW, fullW, pageNum int) *boxNode {
	vsize := e.effectiveVsize()
	if r := vsize - e.bandsReserve(bands); r > 0 { // the columns share the height left below the bands
		vsize = r
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

	// Full-width bands (a \twocolumn[...] span, or figure*/table* floats) sit stacked
	// above the two columns, each separated from the next by \dbltextfloatsep.
	var content []node
	for _, b := range bands {
		content = append(content, b, glueNode{spec: glueSpec{width: e.dblTextFloatSep()}})
	}
	content = append(content, cols)

	saved := e.hsize
	e.hsize = fullW
	page := e.assemblePage(content, pageNum)
	e.hsize = saved
	return page
}
