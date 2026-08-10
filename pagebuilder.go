// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is a page builder: it splits the main vertical list into pages no
// taller than \vsize. TeX's real page builder chooses the break with least cost
// (\pagegoal vs the running \pagetotal, badness and penalties); this is the
// greedy form — fill until the next box would overflow, then break at that point,
// discarding the glue/penalty that sits at the break (as TeX discards glue after
// a page break). It is faithful in structure and produces correct multi-page
// output; cost-based optimisation and \output come next.

// vContribution is a node's contribution to the running page height (sp).
func vContribution(n node) int {
	switch c := n.(type) {
	case *boxNode:
		return c.height + c.depth
	case ruleNode:
		return c.height + c.depth
	case kernNode:
		return c.width
	case glueNode:
		return c.spec.width
	case charNode:
		return c.height + c.depth
	}
	return 0
}

// Pages splits the main vertical list into pages, each vpacked at natural height
// (≤ \vsize where the material allows a legal break). A single page taller than
// \vsize because one box exceeds it is kept whole (overfull) rather than lost.
func (e *Engine) Pages() []*boxNode {
	var pages []*boxNode
	var cur []node
	h := 0
	flush := func() {
		if len(cur) > 0 {
			pages = append(pages, vpackSP(cur, packNatural, 0))
			cur, h = nil, 0
		}
	}
	for _, n := range e.mvl {
		c := vContribution(n)
		_, isGlue := n.(glueNode)
		_, isPenalty := n.(penaltyNode)
		if h+c > e.vsize && len(cur) > 0 {
			flush()
			if isGlue || isPenalty {
				continue // discard the breakpoint glue/penalty at the top of a page
			}
		}
		cur = append(cur, n)
		h += c
	}
	flush()
	return pages
}

// RenderPages renders each page of the main vertical list to its own SVG string.
func (e *Engine) RenderPages(margin float64) []string {
	pages := e.Pages()
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = renderBoxSVG(p, margin, e.curFont)
	}
	return out
}
