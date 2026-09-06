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
	e.applyTwoColumnMeasure() // halve e.hsize to the column width on the first paragraph (two-column mode)
	lineWidth := spToPt(e.hsize)
	list, lines, ok := e.breakSegment(hlist, lineWidth)
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
// breakSegment runs TeX's line-breaking passes over one paragraph fragment and
// returns the node list its line indices refer to (tex.web §16986-16999). The
// FIRST pass tries the paragraph with no hyphenation at all, at \pretolerance:
// discretionaries do not exist yet, so an unhyphenated solution wins whenever one
// is feasible — which is why real TeX hyphenates far less than an optimiser that
// always has the hyphens in hand. Only when that pass finds nothing is the second
// pass run over the hyphenated list at \tolerance, and then an emergency pass at
// a large tolerance if a bad line survives.
func (e *Engine) breakSegment(hlist []node, lineWidth float64) ([]node, []Line, bool) {
	p := LineBreakParams{
		Tolerance:   float64(e.namedInt("tolerance")),
		LinePenalty: float64(e.namedInt("linepenalty")),
		AdjDemerits: float64(e.namedInt("adjdemerits")),
		DoubleHyph:  float64(e.namedInt("doublehyphendemerits")),
		FinalHyph:   float64(e.namedInt("finalhyphendemerits")),
	}
	if pre := e.namedInt("pretolerance"); pre >= 0 {
		q := p
		q.Tolerance = float64(pre)
		plain := e.withParFill(hlist)
		if lines, ok := KnuthPlassWith(toItems(plain), lineWidth, q); ok && !hasBadLine(lines) {
			return plain, lines, true
		}
	}
	list := e.withParFill(e.hyphenateList(hlist)) // insert discretionary hyphens (if patterns loaded)
	items := toItems(list)
	lines, ok := KnuthPlassWith(items, lineWidth, p)
	// The second pass can "succeed" with a single over/underfull line via the forced
	// final break. When it leaves a bad line, run the emergency pass at a large
	// tolerance so discretionary (hyphen) and no-stretch breaks become feasible.
	// Adopt it only if it removes the bad line.
	if !ok || hasBadLine(lines) {
		q := p
		q.Tolerance = maxBadRatio
		if l2, ok2 := KnuthPlassWith(items, lineWidth, q); ok2 {
			if !ok || len(l2) > len(lines) || !hasBadLine(l2) {
				lines, ok = l2, true
			}
		}
	}
	return list, lines, ok
}

// withParFill closes a horizontal list the way TeX ends every paragraph: with the
// \parfillskip PARAMETER, whatever it holds (tex.web:16084,
// "link(tail):=new_param_glue(par_fill_skip_code)"), and a forced break. It
// copies, so the caller's list is left alone.
//
// The value was hard-coded here as 0pt plus 1fil — LaTeX's default (latex.ltx:546)
// but only its default. \centering and \raggedleft set \parfillskip to zero
// (latex.ltx:11018, 11028) precisely so the last line does NOT get a third
// infinite glue; with it hard-coded, a centred line carried \leftskip, \rightskip
// AND \parfillskip and settled a third of the way across instead of halfway.
func (e *Engine) withParFill(list []node) []node {
	fill := glueSpec{stretch: unity, stretchOrder: 1}
	if m := e.eq["parfillskip"]; m != nil && m.kind == mSkipRef && m.code >= 0 && m.code < len(e.skip) {
		fill = e.skip[m.code]
	}
	return append(append([]node{}, list...),
		glueNode{spec: fill},
		penaltyNode{penalty: -int(InfPenalty)})
}

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
			// A flagged (hyphen) break; consecutive flagged breaks are demerited. The
			// width is 0: the hyphen glyph is real material only on the line that
			// breaks here (layoutSegment appends it), and the linebreak library folds
			// a penalty's width into the running line total even when the break is NOT
			// taken there — so a non-zero width here would inflate every line holding
			// an unbroken hyphenation point. A corpus A/B (see the PR) confirmed 0 is
			// the faithful choice on the current library.
			items[i] = Penalty(0, float64(c.penalty), true)

		case ruleNode:
			items[i] = Box(spToPt(c.width))
		case *boxNode:
			items[i] = Box(spToPt(c.width))
		case frameNode:
			items[i] = Box(spToPt(c.width()))
		case transformNode:
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
