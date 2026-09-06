// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "math"

// This file is the page builder: it splits the main vertical list into pages no
// taller than \vsize using TeX's cost-based rule — at each legal breakpoint the
// cost is the page badness plus the breakpoint penalty, the page fires at the
// least-cost break once it would overflow, a forced break (\penalty ≤ −10000)
// fires immediately, and \penalty ≥ 10000 forbids a break. Glue/penalty/kern at
// a break is discarded. \vsplit reuses the simpler height-limited splitVList.
// (Insertions, \topskip and a token \output routine are future work.)

// vContribution is a node's contribution to the running page height (sp).
func vContribution(n node) int {
	switch c := n.(type) {
	case *boxNode:
		return c.height + c.depth
	case spanNode: // a stray full-width band (normally lifted out by the two-column pager)
		if c.box != nil {
			return c.box.height + c.box.depth
		}
		return 0
	case ruleNode:
		return c.height + c.depth
	case kernNode:
		return c.width
	case glueNode:
		return c.spec.width
	case charNode:
		return c.height + c.depth
	case mathNode:
		return c.height + c.depth
	case imageNode:
		return c.height + c.depth
	case frameNode:
		return c.height() + c.depth()
	case decoNode:
		return c.height() + c.depth()
	case transformNode:
		return c.height() + c.depth()
	case linkNode:
		return c.height() + c.depth()
	case internalLinkNode:
		return c.height() + c.depth()
	case footnoteNode:
		// The note's own height, so the page breaks early enough to leave room for
		// the foot area even though the note is not painted inline (assemblePage
		// lifts it to the page bottom). The space the foot area itself costs —
		// \skip\footins — is charged ONCE per page, by findPageBreak, not here:
		// see footinsSkip.
		return c.body.height + c.body.depth
	}
	return 0
}

// Pages splits the main vertical list into pages using TeX's cost-based page
// builder (§1005): at each legal breakpoint it forms a cost = page badness +
// penalty, breaks at the least-cost point once the page would overflow, honours a
// forced break (\penalty ≤ −10000) immediately, and never breaks at \penalty ≥
// 10000. Each page is vpacked at natural height. (Insertions/\topskip are future.)
// maxPages bounds how many pages Pages() will emit. No real document approaches
// it (the largest in the arXiv corpus is ~150pp); it is a backstop against a
// class whose page-fitting machinery runs away — WileyNJD's \BX box-splitter can
// \vsplit a mis-sized box into hundreds of thousands of near-empty pages, which
// would otherwise write that many SVG files and exhaust memory/disk.
const maxPages = 5000

func (e *Engine) Pages() []*boxNode {
	// \onecolumn / \twocolumn split the document into page-aligned regions (both
	// commands \clearpage first — verified against the reference), each paginated in
	// its own column mode; see twocolumn.go.
	// A document that only ever asked for ONE column still records a region once the
	// column machinery is live, and the region pager knows nothing about floats — it
	// would drop every captured one. So regions route there only when one of them
	// actually has two columns.
	if len(e.colRegions) > 0 && (!floatsEnabled() || !e.mvlHasFloats() || e.hasTwoColumnRegion()) {
		return e.pagesByRegion()
	}
	// Under GOTEX_FLOATS, a document that captured any figure/table float paginates
	// through the float placer (top/bottom/float-page placement + deferral); otherwise the
	// plain single-column breaker. mvlHasFloats is false unless the placer is enabled, so
	// with the flag off this is the ordinary path and the output is unchanged.
	if floatsEnabled() && e.mvlHasFloats() {
		return e.pagesWithFloats()
	}
	return e.paginateSingleList(e.mvl, 0)
}

// paginateSingleList splits one vertical list into single-column pages using the
// cost-based breaker, numbering them from pageOffset+1.
func (e *Engine) paginateSingleList(list []node, pageOffset int) []*boxNode {
	var pages []*boxNode
	// Whatsits from slices that held no box. TeX keeps them on the contribution
	// list rather than shipping a page for them; see pageIsUnshippable.
	var carry []node
	for start := 0; start < len(list); {
		end := e.findPageBreak(list, start)
		page := trimTrailingGlue(list[start:end])
		switch {
		case pageIsUnshippable(page):
			for _, n := range page {
				if _, isSpecial := n.(specialNode); isSpecial {
					carry = append(carry, n)
				}
			}
		case len(page) > 0:
			full := make([]node, 0, len(carry)+len(page))
			full = append(append(full, carry...), page...)
			carry = nil
			pages = append(pages, e.assemblePage(full, pageOffset+len(pages)+1))
		}
		if end == len(list) {
			break
		}
		if pageOffset+len(pages) >= maxPages {
			if e.skippedCS == nil {
				e.skippedCS = map[string]int{}
			}
			e.skippedCS["gotex@pagelimit"]++
			break
		}
		// Discard the glue/penalty/kern at the break so the next page does not
		// restart inside that run. Advancing by the full run (not end+1) is what
		// keeps this loop linear: a long trailing discardable run would otherwise
		// be rescanned on every one-node step, making pagination O(n²).
		next := skipDiscardable(list, end)
		if next <= start { // safety: always make progress
			next = start + 1
		}
		start = next
	}
	return pages
}

// effectiveVsize is the page height used for breaking. It is \vsize, unless a
// class left \vsize at 0 (or negative): journal classes that compute their page
// height through a custom \output routine the engine does not run — aastex/ltxgrid,
// revtex — never set \vsize, so it stays 0. Breaking against 0 makes every legal
// breakpoint overfull, exploding a paper into hundreds of near-empty pages. Fall
// back to the plain-TeX default page height so such a document paginates normally.
func (e *Engine) effectiveVsize() int {
	if e.vsize > 0 {
		return e.vsize
	}
	return ptToSP(8.9 * 7227.0 / 100.0) // plain TeX \vsize = 8.9in
}

// pageIsUnshippable reports whether a page slice holds nothing that would make the
// page non-empty for TeX. tex.web §1000 sets page_contents to box_there only for an
// hlist, vlist or rule node; a whatsit is contributed WITHOUT doing so, and the glue,
// kerns and penalties that reach a page holding no box are discarded outright
// ("goto done1"). So \newpage — \par\penalty-10000 — before any box ships nothing.
//
// Here it shipped a blank sheet as soon as a class preamble held a \special: oupau.cls
// writes \special{papersize=210mm,297mm} (line 81), and every document that class set
// therefore opened on an empty page, its title pushed to page 2.
func pageIsUnshippable(page []node) bool {
	for _, n := range page {
		switch n.(type) {
		case glueNode, penaltyNode, kernNode, specialNode:
		default:
			return false
		}
	}
	return true
}

// findPageBreak returns the exclusive end index of the page beginning at start.
func (e *Engine) findPageBreak(list []node, start int) int {
	vsize := e.effectiveVsize()
	h := 0
	sawFootnote := false
	best := -1
	least := math.Inf(1)
	for i := start; i < len(list); i++ {
		if ok, pen := legalPageBreak(list, i, start); ok {
			if pen <= -10000 {
				return i // forced break
			}
			b := pageBadness(h, vsize)
			if math.IsInf(b, 1) { // already overfull at a legal break
				if best >= 0 {
					return best
				}
				return i
			}
			if c := b + float64(pen); c <= least {
				least, best = c, i
			}
		}
		h += vContribution(list[i])
		// \skip\footins is what the FIRST footnote of a page costs beyond its own
		// height, and it is charged once for the page (tex.web:19638: page_goal is
		// reduced by width(skip n) inside "Create a page insertion node", which runs
		// only when the first \insert n of a page is seen). Charging it per note —
		// a flat 16pt each, as this did — reserved room nobody used: measured
		// against tectonic on a page of four footnotes, our foot area ended 45pt
		// above the bottom of the text block where the reference's ends flush with
		// it, and the body gave up 88.9pt against the reference's 35.9.
		if _, isFn := list[i].(footnoteNode); isFn && !sawFootnote {
			sawFootnote = true
			h += e.footinsSkip()
		}
		if h > vsize && best >= 0 {
			return best
		}
	}
	return len(list)
}

// legalPageBreak reports whether index i is a legal page breakpoint and its
// penalty: a penalty node (< 10000), or glue that follows box-like material.
func legalPageBreak(list []node, i, start int) (bool, int) {
	switch n := list[i].(type) {
	case penaltyNode:
		if n.penalty >= 10000 {
			return false, 0
		}
		return true, n.penalty
	case glueNode:
		if i > start && isBoxLike(list[i-1]) {
			return true, 0
		}
	}
	return false, 0
}

func isBoxLike(n node) bool {
	switch n.(type) {
	case *boxNode, ruleNode, charNode, mathNode:
		return true
	}
	return false
}

// pageBadness measures how far a page of height h falls short of \vsize: 0 when
// exactly full, up to 10000 when empty, and +Inf when overfull.
func pageBadness(h, vsize int) float64 {
	if h > vsize {
		return math.Inf(1)
	}
	if vsize <= 0 {
		return 0
	}
	f := float64(vsize-h) / float64(vsize)
	return 10000 * f * f * f
}

// skipDiscardable advances past glue/penalty/kern at a page break (discarded).
func skipDiscardable(list []node, i int) int {
	for i < len(list) {
		switch list[i].(type) {
		case glueNode, penaltyNode, kernNode:
			i++
		default:
			return i
		}
	}
	return i
}

// trimTrailingGlue drops trailing glue/penalty/kern from a page's material.
func trimTrailingGlue(page []node) []node {
	for len(page) > 0 {
		switch page[len(page)-1].(type) {
		case glueNode, penaltyNode, kernNode:
			page = page[:len(page)-1]
		default:
			return page
		}
	}
	return page
}

// splitVList splits a vertical list at the first legal break that keeps the
// height within vsize, returning the taken part and the remainder (with a leading
// glue/kern discarded, as at a page break).
func splitVList(list []node, vsize int) ([]node, []node) {
	h := 0
	for i, n := range list {
		c := vContribution(n)
		_, isGlue := n.(glueNode)
		_, isKern := n.(kernNode)
		if h+c > vsize && h > 0 {
			if isGlue || isKern {
				return list[:i], list[i+1:]
			}
			return list[:i], list[i:]
		}
		h += c
	}
	return list, nil
}

// doVsplit handles \vsplit<n> to <dimen>: split box register n's vbox to the
// given height, returning the top part (packed to that height) and leaving the
// remainder in register n. Returns nil if the register is void or the syntax is
// malformed.
func (e *Engine) doVsplit() *boxNode {
	n := e.scanInt()
	if !e.scanKeyword("to") {
		return nil
	}
	h := e.scanDimen()
	src := e.getBox(n)
	if src == nil {
		return nil
	}
	top, rest := splitVList(src.list, h)
	if len(rest) == 0 {
		e.setBox(n, nil)
	} else {
		e.setBox(n, vpackSP(rest, packNatural, 0))
	}
	return vpackSP(top, packTo, h)
}

// RenderPages renders each page of the main vertical list to its own SVG string.
func (e *Engine) RenderPages(margin float64) []string {
	pages := e.Pages()
	vmargin := e.renderVMargin(margin)
	paperW, paperH, _ := e.paperSizePt()
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = renderBoxSVG(p, margin, vmargin, paperW, paperH, e.renderFont(), e.pageFill())
	}
	return out
}
