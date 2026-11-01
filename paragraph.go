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
	hlist := e.parList
	e.parList = nil
	// ⛔⛔ ONE PARAGRAPH, ONE \parfillskip.
	//
	// This used to cut the list at every forced penalty and lay each fragment out as
	// its own paragraph, which gave every fragment its own \parfillskip. That glue is
	// 0pt plus 1fil: it takes ALL the slack when the line is packed to \hsize, so the
	// interword glue got none and a line ended by a forced break came out at its
	// NATURAL width instead of justified.
	//
	// tex.web §16066-16084 appends \parfillskip ONCE, in line_break, for the whole
	// paragraph. A forced penalty inside is an ordinary breakpoint of cost −∞, and the
	// lines before it are justified like any other. The breaker here already receives
	// those penalties — toItems maps penaltyNode to Penalty — so the split was never
	// what made the forced break happen, only what added the spurious glue.
	//
	// Measured against tectonic, x of the second word on a line ended by a forced
	// break: 456.72 there (the right margin) against 147.18 here.
	if len(hlist) > 0 {
		e.layoutSegment(hlist)
	}
	// Attach any footnotes referenced in this paragraph to the vertical list,
	// just after it, so they land on (and reserve room in) the same page.
	e.flushFootnotes()
}

// layoutSegment hyphenates, line-breaks (Knuth–Plass with an emergency pass) and
// contributes the lines of a paragraph to the main vertical list.
//
// ⛔ The paragraph's FIRST line keeps its leading glue; every later line is pruned.
// tex.web's post_line_break (17273-17279) justifies a line and appends it, and only
// THEN prunes the next one's head — 17297 explains why: the pruning drops the glue the
// break consumed. The first line follows no break and so keeps whatever the paragraph
// began with (#657, #659). With the per-fragment split gone that rule is just
// "i > 0" in the line loop, which is what it always meant.
func (e *Engine) layoutSegment(hlist []node) {
	e.applyTwoColumnMeasure() // halve e.hsize to the column width on the first paragraph (two-column mode)
	// The measure the TEXT is broken against is \hsize less the two skips that
	// will be glued to every line (applyLineSkips). TeX reaches the same place
	// from the other side: it starts each line's width accounting at
	// background := left_skip.width + right_skip.width (tex.web §827) and breaks
	// against \hsize, so the text itself never gets more than what is left.
	//
	// ⛔ Breaking against the whole \hsize and gluing the skips on afterwards
	// makes every line overfull by exactly their two widths, and the hpack that
	// follows then SHRINKS the inter-word glue to make it fit. The words come out
	// touching and the paragraph is set far wider than it asked for: with
	// \rightskip=100pt on a 343pt measure, the text still ran to within 13.5pt of
	// the right margin — the only narrowing was whatever shrink the glue had.
	// Every \quote and \quotation (leftskip and rightskip both 20pt, latex.go:550)
	// was set 40pt too wide this way, as was every contents entry of a real class.
	//
	// A \rightskip with infinite stretch and no width — \raggedright's
	// 0pt plus 1fil — narrows nothing, which is right: ragged setting does not
	// move the margin, it only stops justifying to it.
	lineWidth := spToPt(e.hsize - e.leftskip.width - e.rightskip.width)
	list, lines, ok := e.breakSegment(hlist, lineWidth)
	if !ok || len(lines) == 0 {
		lines = []Line{{Start: 0, End: len(list)}} // last resort: one line, nothing lost
	}
	for i, ln := range lines {
		// ⛔⛔ THE PARAGRAPH'S FIRST LINE IS NEVER PRUNED — and "the paragraph's", not
		// "this fragment's". go-typeset/linebreak v1.2.1 stopped discarding a
		// paragraph's opening glue; this is the other half, and together they close
		// go-tex/engine#657: \noindent\hskip 20pt went from x=133.77 to 153.70 against
		// the reference's 153.69.
		//
		// ⛔ This used to need a firstSegment flag, because endParagraph split the list
		// at every forced penalty and called this function once per fragment — so every
		// fragment arrived with i=0 and, without the flag, every fragment's head was
		// exempt from pruning. The split is gone (one paragraph, one \parfillskip), so
		// there is one call, i counts the paragraph's lines, and "i > 0" says exactly
		// what the rule is: the first line follows no break, every later one does.
		//
		// A \\ is a forced penalty INSIDE the list and the breaker takes it as a
		// breakpoint like any other, so a line after a \\ has i > 0 and IS pruned —
		// which is what TestHspaceStarSurvivesALineBreak asserts.
		seg := list[ln.Start:ln.End]
		if i > 0 {
			seg = trimLeadingGlue(seg)
		}
		// If the line was broken at a discretionary, append a hyphen. Copy the
		// segment first: it aliases list's backing array, so appending in place
		// would overwrite the next line's first node.
		if ln.End < len(list) {
			if d, isDisc := list[ln.End].(discNode); isDisc && e.curFont != nil && d.pre != "" {
				seg = append([]node{}, seg...)
				for _, r := range d.pre {
					w, h, dd := e.curFont.charDimsSP(r)
					seg = append(seg, charNode{ch: r, width: w, height: h, depth: dd, size: e.curFont.sizePt(), face: e.curFont})
				}
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
	// final break. Two more passes follow, in this order, and each is adopted only if
	// it removes the bad line.
	//
	// ⛔⛔ FIRST THE REAL FINAL PASS: \tolerance unchanged, artificial_demerits armed
	// (go-typeset/linebreak v1.2.2, Params.FinalPass; tex.web §16769-16771). At a
	// breakpoint where NOTHING is feasible and only one node is active, the break is
	// recorded at ZERO cost rather than the node being deactivated.
	//
	// Zero is what decides it. A line past inf_bad costs 1e8, so two hopeless lines
	// (2e8) otherwise lose to ONE overfull line (1e8) — and the breaker returned the
	// overfull one. Measured with spMock: two unbreakable 35pt words and a 3pt rigid
	// space on a 40pt measure came back as one line 33pt over the margin
	// (TestActiveTieDoesNotBreakLine), where the reference sets two.
	//
	// ⛔ AND IT MUST COME BEFORE THE WIDE-OPEN PASS, not with it. Raising the tolerance
	// to maxBadRatio makes a hopeless line FEASIBLE at its full 1e8, which DEFEATS the
	// clause: artificial demerits only fire when nothing feasible was recorded there.
	// Setting both at once left the test failing exactly as before.
	//
	// None of this showed while every forced-break fragment got its own \parfillskip:
	// that spurious glue was a legal breakpoint at the end of each fragment, and the
	// overfull single-line node was deactivated there BY ACCIDENT. Removing the glue
	// removed the accident, and the missing clause surfaced.
	// ⛔⛔ FinalPass IS NOT ARMED HERE YET, AND THAT IS A MEASURED DECISION.
	//
	// Arming it hyphenated titles — "THETI-TLE" across eleven class tests — because
	// TeX reaches its final pass only when pass 2 failed OUTRIGHT, and pass 2 does not
	// fail here for a reason the library does not yet reproduce: TeX refuses to record
	// a forced break whose line is past the threshold (tex.web §16769 → §16824-16831,
	// "else if b>threshold then goto deactivate"), while the library records every
	// forced break unconditionally. Until that is faithful the clause fires at
	// breakpoints TeX never reaches, and at ZERO cost, so it wins everywhere.
	//
	// Arming it TOGETHER with the wide-open tolerance below is worse still: raising the
	// tolerance makes a hopeless line feasible at its full 1e8, which defeats the
	// clause — artificial demerits only fire when nothing feasible was recorded there.
	//
	// So the wide-open pass stays as it was, alone, and #666 carries the rest.
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
	out := append([]node{}, list...)
	// ⛔⛔ AN INFINITE PENALTY GOES IN FRONT OF \parfillskip, AND A TRAILING GLUE IS
	// REPLACED BY IT RATHER THAN KEPT. tex.web §16077-16084, verbatim in its own words:
	//
	//	if is_char_node(tail) then tail_append(new_penalty(inf_penalty))
	//	else if type(tail)<>glue_node then tail_append(new_penalty(inf_penalty))
	//	else begin type(tail):=penalty_node; … penalty(tail):=inf_penalty; end;
	//	link(tail):=new_param_glue(par_fill_skip_code);
	//
	// and §16066-16071 says why, twice: the \parfillskip "is preceded by an infinite
	// penalty, so it will never be considered as a potential breakpoint", and the
	// trailing glue is removed "since spaces usually precede blank lines and instances
	// of $$".
	//
	// This appended the \parfillskip straight onto the list, so a space before \par
	// stayed as material AND the \parfillskip itself was a legal breakpoint — a glue
	// preceded by non-discardable material is one. A break there yields a last line
	// holding nothing but the fill.
	if n := len(out); n > 0 {
		if _, isGlue := out[n-1].(glueNode); isGlue {
			out[n-1] = penaltyNode{penalty: int(InfPenalty)}
		} else {
			out = append(out, penaltyNode{penalty: int(InfPenalty)})
		}
	} else {
		out = append(out, penaltyNode{penalty: int(InfPenalty)})
	}
	// ⛔ The forced break after the \parfillskip is OURS, not tex.web's: TeX's
	// line_break ends the last line by reaching the end of the list, while this
	// breaker needs the break named. It is the classic Knuth–Plass paragraph
	// terminator and it changes nothing about where the other lines fall.
	return append(out, glueNode{spec: fill}, penaltyNode{penalty: -int(InfPenalty)})
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

// interlineGlue is tex.web §679 append_to_vlist, the rule that puts a box's
// baseline \baselineskip below the previous one:
//
//	if prev_depth>ignore_depth then
//	  begin d:=width(baseline_skip)-prev_depth-height(b);
//	  if d<line_skip_limit then p:=new_param_glue(line_skip_code)
//	  else  begin p:=new_skip_param(baseline_skip_code);
//	    width(temp_ptr):=d;
//
// It is one function because it was FOUR copies. appendToPage had it,
// tabular.go's cell stacker had it, listings.go's line stacker had it, and
// halign.go — which stacks the rows of every \halign — did not: a three-row
// alignment grew 6.63pt per row where the reference grows 12.00pt, because the
// rows were butted together at their natural height with no glue at all. Three
// sites implementing a rule and a fourth ignoring it is not a rule.
//
// The threshold is \lineskiplimit and the replacement is \lineskip: two different
// parameters, 0pt and 1pt in plain TeX and LaTeX. This tested d < \lineskip,
// using \lineskip's width for both roles, which put the crossover a point too
// high. Isolating the glue itself — \ht of the vbox less the two boxes' heights,
// because the TOTAL hides it — against tectonic:
//
//	second box   d       tectonic   before    after
//	11.5pt       +0.5      0.50pt   1.00pt   0.50pt   <- the divergent band
//	13pt         −1.0      1.00pt   1.00pt   1.00pt   control: d < limit
//	5pt          +7.0      7.00pt   7.00pt   7.00pt   control: d ≥ limit
//
// ⚠ The total is NOT the instrument here. The same three vboxes measured whole
// read 19.16/19.12, 21.16/20.62 and 19.16/18.62 — the divergent case shows 0.04pt,
// the smallest of the three, because \hbox{A} is 6.62pt here against the
// reference's 7.16 and the two errors nearly cancel. Only the glue, derived, says
// 0.50 against 1.00.
//
// Still divergent, and out of scope for the same reason it was before: §679 takes
// \lineskip and \baselineskip as GLUE, with whatever stretch and shrink they
// carry, while this builds a rigid spec. That is not a local fix — \baselineskip
// scans a full glue and keeps only .width (primitives.go), and e.baselineskip is an
// int, so \baselineskip=12pt plus 2pt loses its stretch at the ASSIGNMENT. See
// go-tex/engine#441.
func (e *Engine) interlineGlue(prevDepth, height int) (glueNode, bool) {
	if prevDepth <= ignoreDepth {
		return glueNode{}, false
	}
	gap := e.baselineskip - prevDepth - height
	if gap < e.lineskiplimit {
		gap = e.lineskip
	}
	return glueNode{spec: glueSpec{width: gap}}, true
}

// appendToPage adds a box to the main vertical list, inserting interline glue so
// the baseline sits \baselineskip below the previous one (at least \lineskip).
func (e *Engine) appendToPage(b *boxNode) {
	if g, ok := e.interlineGlue(e.prevDepth, b.height); ok {
		e.mvl = append(e.mvl, g)
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

// trimLeadingGlue prunes the discardables at a line's left edge, tex.web §17297-17313,
// which says what they are in one sentence:
//
//	Glue and penalty and kern and math nodes are deleted at the beginning of a line,
//	except in the anomalous case that the node to be deleted is actually one of the
//	chosen breakpoints.
//
// and then loops, stopping at a char node, at a non-discardable, at an implicit kern,
// or at the next chosen breakpoint.
//
// ⛔⛔ PENALTIES TOO, AND THAT IS NOT A DETAIL. This dropped glue and discretionaries
// only. A line that begins at a forced break begins AT THE PENALTY — the breaker
// reports Line.Start on the penalty itself, not after it — so the first node was a
// penaltyNode, the pruning stopped there on sight, and the glue behind it survived.
//
// Measured on the reference's own \hspace* document, x of the first glyph of each line:
//
//	                      oracle   avant   après
//	A                     133.77   133.77  133.77
//	\hspace*{50pt}B       183.58   185.58  185.58   le filet le protège
//	\hspace{50pt}C        133.77   185.58  133.77   écartée, comme chez l'oracle
//
// It was invisible while endParagraph split the paragraph at forced penalties: the
// split REMOVED the penalty from the list, so a fragment began on the glue by
// construction and this function never had to step over one. Another thing the
// workaround was hiding (#666).
//
// ⭐ And this is exactly why \hspace* survives: \@hspacer puts a zero-width RULE in
// front of the glue (ltspace.dtx:1436-1437), a rule is NOT discardable, so the loop
// stops on it and the 50pt stays. Nothing here special-cases the star.
//
// ⛔ KERNS AND MATH NODES ARE LEFT ALONE HERE, AND THAT IS DELIBERATE.
//
// §17312 prunes a kern only when `subtype(q)<>explicit` is false — an IMPLICIT kern
// stops the pruning, because a font kern belongs to the pair of glyphs around it and
// dropping it at a line start would change the word's shape. kernNode carries no such
// distinction (stomach.go: width and `space`), so there is no way to prune the right
// ones; pruning all of them would corrupt kerning, and that is worse than leaving a
// small space at a margin.
//
// §17313's math_node is TeX's `$` boundary, a discardable marker. Ours is a math BOX
// with a width, a height and a depth — toItems maps it to Box — so pruning it would
// drop CONTENT, not space.
//
// Both are named rather than silently omitted: the next reader should see that the
// reference prunes more than this does, and why stopping here is the faithful choice
// given the node types that exist.
func trimLeadingGlue(seg []node) []node {
	for len(seg) > 0 {
		switch seg[0].(type) {
		case glueNode, discNode, penaltyNode:
			seg = seg[1:]
		default:
			return seg
		}
	}
	return seg
}
