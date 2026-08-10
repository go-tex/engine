// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file is the paragraph builder: TeX's transition from horizontal to
// vertical mode. Text accumulated at top level forms a horizontal list; \par (or
// end of input) breaks it into lines of \hsize with Knuth–Plass, packs each line
// to \hsize, and appends the line boxes to the main vertical list separated by
// interline (baselineskip) glue. It reuses the KnuthPlass optimiser over an Item
// projection of the sp node list.

// ignoreDepth is TeX's -1000pt sentinel for \prevdepth: no interline glue is
// inserted above a box when the previous depth is this value (top of page/after a
// rule).
var ignoreDepth = ptToSP(-1000)

// endParagraph finishes the current paragraph: it appends \parfillskip and a
// forced break, runs the line breaker, and contributes the packed lines to the
// main vertical list. A no-op when no paragraph is in progress.
func (e *Engine) endParagraph() {
	if !e.inPar {
		return
	}
	e.inPar = false
	list := e.parList
	e.parList = nil

	// \parfillskip (0pt plus 1fil) fills the last line; a forced break ends it.
	list = append(list,
		glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}},
		penaltyNode{penalty: -int(InfPenalty)})

	lines, ok := KnuthPlass(toItems(list), spToPt(e.hsize), 200, 10)
	if !ok || len(lines) == 0 {
		// Fall back to a single overfull line so nothing is silently lost.
		lines = []Line{{Start: 0, End: len(list)}}
	}
	for _, ln := range lines {
		seg := trimLeadingGlue(list[ln.Start:ln.End])
		e.appendToPage(hpackSP(seg, packTo, e.hsize))
	}
}

// appendToPage adds a box to the main vertical list, inserting interline glue so
// the baseline sits \baselineskip below the previous one (at least \lineskip).
func (e *Engine) appendToPage(b *boxNode) {
	if e.prevDepth > ignoreDepth {
		gap := e.baselineskip - e.prevDepth - b.height
		if gap < e.lineskip {
			gap = e.lineskip
		}
		e.mvl = append(e.mvl, glueNode{spec: glueSpec{width: gap}})
	}
	e.mvl = append(e.mvl, b)
	e.prevDepth = b.depth
}

// toItems projects an sp node list onto Knuth–Plass Items (in points). Infinite
// glue is represented as a large finite stretch/shrink so a fil parfillskip makes
// the last line flush left.
func toItems(list []node) []Item {
	items := make([]Item, len(list))
	for i, n := range list {
		switch c := n.(type) {
		case charNode:
			items[i] = Glyph(c.ch, spToPt(c.width), spToPt(c.height), spToPt(c.depth))
		case kernNode:
			items[i] = Box(spToPt(c.width))
		case glueNode:
			st := spToPt(c.spec.stretch)
			if c.spec.stretchOrder > 0 {
				st = 1e6
			}
			sh := spToPt(c.spec.shrink)
			if c.spec.shrinkOrder > 0 {
				sh = 1e6
			}
			items[i] = Glue(spToPt(c.spec.width), st, sh)
		case penaltyNode:
			items[i] = Penalty(0, float64(c.penalty), false)
		case ruleNode:
			items[i] = Box(spToPt(c.width))
		case *boxNode:
			items[i] = Box(spToPt(c.width))
		}
	}
	return items
}

// trimLeadingGlue drops a single leading glue node (TeX discards glue at a line's
// left edge, since the break has already absorbed it).
func trimLeadingGlue(seg []node) []node {
	if len(seg) > 0 {
		if _, isGlue := seg[0].(glueNode); isGlue {
			return seg[1:]
		}
	}
	return seg
}
