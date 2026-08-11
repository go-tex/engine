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
	segments := splitAtForcedBreaks(e.parList) // \\ (a forced -10000 penalty) ends a line
	e.parList = nil
	for _, seg := range segments {
		if len(seg) > 0 {
			e.layoutSegment(seg)
		}
	}
	// Attach any footnotes referenced in this paragraph to the vertical list,
	// just after it, so they land on (and reserve room in) the same page.
	e.flushFootnotes()
}

// splitAtForcedBreaks divides a paragraph's horizontal list at explicit forced
// breaks (\\, i.e. a penalty ≤ −10000) so each fragment is line-broken as its own
// run of lines — an explicit line break, independent of the optimiser.
func splitAtForcedBreaks(list []node) [][]node {
	var segs [][]node
	var cur []node
	for _, n := range list {
		if p, ok := n.(penaltyNode); ok && p.penalty <= -10000 {
			segs = append(segs, cur)
			cur = nil
			continue
		}
		cur = append(cur, n)
	}
	return append(segs, cur)
}

// layoutSegment hyphenates, line-breaks (Knuth–Plass with an emergency pass) and
// contributes the lines of one paragraph fragment to the main vertical list.
func (e *Engine) layoutSegment(hlist []node) {
	list := e.hyphenateList(hlist) // insert discretionary hyphens (if patterns loaded)

	// \parfillskip (0pt plus 1fil) fills the last line; a forced break ends it.
	list = append(list,
		glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}},
		penaltyNode{penalty: -int(InfPenalty)})

	items := toItems(list)
	lineWidth := spToPt(e.hsize)
	lines, ok := KnuthPlass(items, lineWidth, 200, 10)
	// The first pass can "succeed" with a single over/underfull line via the forced
	// final break. When it leaves a bad line, run the emergency pass at a large
	// tolerance so discretionary (hyphen) and no-stretch breaks become feasible —
	// TeX's second pass. Adopt it only if it removes the bad line.
	if !ok || hasBadLine(lines) {
		if l2, ok2 := KnuthPlass(items, lineWidth, maxBadRatio, 10); ok2 {
			if !ok || len(l2) > len(lines) || !hasBadLine(l2) {
				lines, ok = l2, true
			}
		}
	}
	if !ok || len(lines) == 0 {
		lines = []Line{{Start: 0, End: len(list)}} // last resort: one line, nothing lost
	}
	for _, ln := range lines {
		seg := trimLeadingGlue(list[ln.Start:ln.End])
		// If the line was broken at a discretionary, append a hyphen. Copy the
		// segment first: it aliases list's backing array, so appending in place
		// would overwrite the next line's first node.
		if ln.End < len(list) {
			if _, isDisc := list[ln.End].(discNode); isDisc && e.curFont != nil {
				w, h, dd := e.curFont.charDimsSP('-')
				seg = append(append([]node{}, seg...), charNode{ch: '-', width: w, height: h, depth: dd})
			}
		}
		seg = e.applyLineSkips(seg)
		e.appendToPage(hpackSP(seg, packTo, e.hsize))
	}
}

// applyLineSkips wraps a line's material with \leftskip and \rightskip glue (when
// non-zero). \rightskip = 0pt plus 1fil gives ragged-right (flush-left) lines.
func (e *Engine) applyLineSkips(seg []node) []node {
	if e.leftskip != (glueSpec{}) {
		seg = append([]node{glueNode{spec: e.leftskip}}, seg...)
	}
	if e.rightskip != (glueSpec{}) {
		seg = append(append([]node{}, seg...), glueNode{spec: e.rightskip})
	}
	return seg
}

// hasBadLine reports whether any line is overfull or badly underfull (a ratio
// well outside the normal [-1, small] range — the capped/infinite bad values).
func hasBadLine(lines []Line) bool {
	for _, ln := range lines {
		if ln.Ratio < -1 || ln.Ratio > 100 {
			return true
		}
	}
	return false
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
		case mathNode:
			items[i] = Box(spToPt(c.width)) // a math box: fixed width, a valid break after

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
		case discNode:
			items[i] = Penalty(0, float64(c.penalty), true) // a flagged (hyphen) break

		case ruleNode:
			items[i] = Box(spToPt(c.width))
		case *boxNode:
			items[i] = Box(spToPt(c.width))
		case frameNode:
			items[i] = Box(spToPt(c.width()))
		case linkNode:
			items[i] = Box(spToPt(c.width()))
		}
	}
	return items
}

// trimLeadingGlue drops a leading glue or discretionary node (TeX discards glue
// at a line's left edge, and a discretionary carried to a line start is not set).
func trimLeadingGlue(seg []node) []node {
	for len(seg) > 0 {
		switch seg[0].(type) {
		case glueNode, discNode:
			seg = seg[1:]
		default:
			return seg
		}
	}
	return seg
}
