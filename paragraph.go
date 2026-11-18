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
	shape := e.paragraphShape(lineWidth)
	list, lines, ok := e.breakSegment(hlist, lineWidth, shape)
	if !ok || len(lines) == 0 {
		// ⛔ NOT "one line, nothing lost" ANY MORE. That kept every node and gave
		// the reader a single line 254pt off the page; everything was there and
		// none of it readable. See lastresort.go, including why #766 is what made
		// this path reachable.
		lines = greedyLines(list, lineWidth)
	}
	// ⛔ SAY WHICH LINES CAME OUT BAD. The breaker has judged this since line
	// breaking was written — hasBadLine decides which pass runs — and no caller
	// could ever see it, so a document whose lines run into the margin compiled
	// in silence. Counted HERE, on the lines actually adopted, rather than
	// inside breakSegment where the rejected attempts of earlier passes would be
	// counted too. See badbox.go.
	e.noteBadLines(list, lines, lineWidth)
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
		// ⛔ \vadjust material comes OUT of the line before it is packaged, so it
		// contributes no width — tex.web §12900-12903: with `adjust_tail` non-null,
		// hpack "removes all occurrences of ins_node, mark_node and adjust_node items
		// and appends the resulting material onto the list that ends at adjust_tail".
		// §17435 arms that list immediately before hpack.
		seg, adjusts := extractAdjusts(seg)
		// ⛔ THE SHAPE IS APPLIED HERE, AND IT IS NOT A SHIFT. TeX hpacks the line
		// to ITS OWN line_width and gives the box shift_amount := cur_indent
		// (§847-850). This engine has no shift on a line box, so the same geometry
		// is built from the material it already uses for \leftskip: a RIGID kern of
		// the margin in front, and a target reduced by whatever the line gave up.
		//
		// ⚠ BOTH ARE NEEDED, and a negative \hangindent is why. There the margin is
		// ZERO and the line is still shorter by |indent| (:16729-16730), so a kern
		// alone would leave the target at the full measure and the rightskip glue
		// would stretch the text back across it — the indent would vanish into the
		// justification. Reducing the target instead leaves the slack as empty page,
		// which is what indenting from the right means.
		indent, lnWidth := shape.LineShape(i + 1)
		if indent != 0 {
			seg = append([]node{kernNode{width: ptToSP(indent)}}, seg...)
		}
		// The skips go OUTSIDE the margin: \leftskip first, then the hanging
		// indent, which is the order tex.web sets them in.
		seg = e.applyLineSkips(seg)
		e.appendToPage(hpackSP(seg, packTo, e.hsize-ptToSP(lineWidth-indent-lnWidth)))
		// ⛔ AND IT GOES IN AFTER THE LINE BOX, not before. §17413-17418 is explicit
		// about the order: append_to_vlist(just_box) first, THEN the adjustment, then
		// adjust_tail is cleared. Emitting it first would put a \vadjust's space above
		// the line it was written on.
		for _, a := range adjusts {
			for _, n := range a {
				e.contribute(n)
			}
		}
	}
	e.normalParagraph()
}

// normalParagraph is tex.web's procedure of the same name (:20756-20761), run at
// the end of a paragraph and from eight other places, `vmode+par_end` — every
// \par — among them:
//
//	procedure normal_paragraph;
//	begin if looseness<>0 then eq_word_define(int_base+looseness_code,0);
//	if hang_indent<>0 then eq_word_define(dimen_base+hang_indent_code,0);
//	if hang_after<>1 then eq_word_define(int_base+hang_after_code,1);
//	if par_shape_ptr<>null then eq_define(par_shape_loc,shape_ref,null);
//	end;
//
// ⛔⛔ A PARAGRAPH SHAPE LASTS EXACTLY ONE PARAGRAPH, and this engine kept it for
// ever. While \hangindent was accept-and-ignore that cost nothing and no test
// could have seen it; #795 made it shape the line breaker, and the same omission
// then hung every FOLLOWING paragraph. Measured before this landed: the same
// second paragraph came out as
//
//	after a hung first:   "measure that is wrong" / "by a centimetre is wrong"
//	after a plain first:  "sure that is wrong by a cen-" / "timetre is wrong on every"
//
// — a document's second paragraph set differently because of what its FIRST one
// asked for. A latent bug that one change turned load-bearing, which is the
// shape of defect worth naming: the parameter's LIFETIME only matters once the
// parameter matters.
//
// ⚠ Most LaTeX survived it anyway, because an assignment inside a group is
// restored by the save stack and \@hangfrom and friends are grouped. The
// top-level assignment is the one that leaked.
//
// It writes through setCount/setDimen rather than the slices, so the assignment
// takes part in grouping exactly as TeX's eq_word_define does — a reset that
// escaped its group would be a second bug of the same family.
func (e *Engine) normalParagraph() {
	if e.namedInt("looseness") != 0 {
		if m := e.eq["looseness"]; m != nil && m.kind == mCountRef {
			e.setCount(m.code, 0, false)
		}
	}
	if e.hangIndentReg >= 0 && e.getDimen(e.hangIndentReg) != 0 {
		e.setDimen(e.hangIndentReg, 0, false)
	}
	if e.namedInt("hangafter") != 1 {
		if m := e.eq["hangafter"]; m != nil && m.kind == mCountRef {
			e.setCount(m.code, 1, false)
		}
	}
	// :20760, `if par_shape_ptr<>null then eq_define(par_shape_loc,shape_ref,null)`.
	//
	// ⚠ This one is NOT grouped the way the three above are: parShape is a plain
	// field, not a register, so it is not on the save stack and a \parshape set
	// inside a group is not restored when the group ends. TeX's eq_define does
	// restore it. For a shape that lasts one paragraph the difference is only
	// visible to a group that opens and closes WITHIN a paragraph without a \par,
	// which is narrow — but it is a real divergence and better said than hidden.
	e.parShape = nil
}

// extractAdjusts takes the \vadjust material out of a line and returns the line
// without it, plus each adjustment's vertical list in the order it was written.
//
// ⛔ It COPIES rather than filtering in place: seg aliases the paragraph's backing
// array, and writing to it would corrupt the next line's nodes — the same trap the
// discretionary-hyphen branch above documents. When there is nothing to take out, the
// original slice is returned untouched, so the common case allocates nothing.
//
// ⛔ Several adjustments on one line keep their order (§13009-13010 walks to the end of
// the adjustment list before appending the next one), because two \vadjust on a line
// are two pieces of vertical material and swapping them swaps what the reader sees.
func extractAdjusts(seg []node) ([]node, [][]node) {
	has := false
	for _, n := range seg {
		if _, ok := n.(adjustNode); ok {
			has = true
			break
		}
	}
	if !has {
		return seg, nil
	}
	out := make([]node, 0, len(seg))
	var adj [][]node
	for _, n := range seg {
		if a, ok := n.(adjustNode); ok {
			adj = append(adj, a.list)
			continue
		}
		out = append(out, n)
	}
	return out, adj
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

// paragraphShape builds the Measure this paragraph is broken on, which is
// tex.web §847-850's choice of line_width per line.
//
// ⛔ A SHAPE IS A LINE-BREAKING PARAMETER, NOT A PLACEMENT ONE. The width is an
// input to the badness of every candidate break, so a paragraph asking for
// \hangindent is not the uniform paragraph moved over — it BREAKS somewhere
// else. Measured against tectonic on go-tex/engine#757's witness: not only was
// the indent missing, a phrase on the reference's line 4 was on our line 3.
//
// \hangafter is an INTEGER parameter, not a dimension, and the two scan
// differently — `\hangafter=1 integers follow` loses the "in" if it is read as a
// dimension looking for a unit. hangafter_probe_test.go pins that, and this
// reads it through namedInt for the same reason.
//
// ⚠ \parshape is NOT here: it is not a primitive in this engine at all, so a
// document writing it gets an undefined command rather than a silently dropped
// shape. go-typeset/linebreak's ParShape is ready for it when it is added.
func (e *Engine) paragraphShape(lineWidth float64) Measure {
	// ⭐ \parshape WINS, and :16706 is where that is settled:
	//
	//	if par_shape_ptr=null then (the hang_indent branch)
	//	else (the parshape branch)
	//
	// so a document setting both gets the shape and \hangindent is not consulted
	// at all. Reading them in the other order would silently prefer the simpler
	// parameter wherever a class set it for its own reasons.
	if len(e.parShape) > 0 {
		return ParShape(e.parShape)
	}
	if e.hangIndentReg < 0 {
		return Uniform(lineWidth)
	}
	indent := spToPt(e.getDimen(e.hangIndentReg))
	if indent == 0 {
		// tex.web:16689: "if hang_indent=0, the value of hang_after is
		// irrelevant" — and a uniform measure is cheaper to consult.
		return Uniform(lineWidth)
	}
	return HangIndent(lineWidth, indent, e.namedInt("hangafter"))
}

// breakSegment runs TeX's line-breaking passes over one paragraph fragment and
// returns the node list its line indices refer to (tex.web §16986-16999). The
// FIRST pass tries the paragraph with no hyphenation at all, at \pretolerance:
// discretionaries do not exist yet, so an unhyphenated solution wins whenever one
// is feasible — which is why real TeX hyphenates far less than an optimiser that
// always has the hyphens in hand. Only when that pass finds nothing is the second
// pass run over the hyphenated list at \tolerance, and then an emergency pass at
// a large tolerance if a bad line survives.
func (e *Engine) breakSegment(hlist []node, lineWidth float64, shape Measure) ([]node, []Line, bool) {
	p := LineBreakParams{
		Tolerance:   float64(e.namedInt("tolerance")),
		LinePenalty: float64(e.namedInt("linepenalty")),
		AdjDemerits: float64(e.namedInt("adjdemerits")),
		DoubleHyph:  float64(e.namedInt("doublehyphendemerits")),
		FinalHyph:   float64(e.namedInt("finalhyphendemerits")),
		Background:  e.skipBackground(),
		// \looseness: how many lines longer (or shorter) the paragraph should come
		// out than the breaking would otherwise make it (tex.web:17197-17237). It
		// was declared in texparams.go and read by nothing — go-tex/engine#754 item
		// (C), go-typeset/linebreak#12 item (7) — so a document that set it was
		// ignored in silence. go-typeset/linebreak v1.12.0 added the choice; this
		// hands the parameter over.
		//
		// ⛔ IT IS NOT A SECOND SEARCH. Every feasible way of ending the paragraph
		// is already an active node when the last break is tried, and \looseness
		// picks among them; a paragraph with one feasible ending cannot be loosened
		// at all. The pass then FAILS unless it is the final one (:17203), which is
		// exactly the shape of the three passes below: the pretolerance pass
		// declines, and the pass carrying FinalPass takes what it reached.
		Looseness: e.namedInt("looseness"),
	}
	if pre := e.namedInt("pretolerance"); pre >= 0 {
		q := p
		q.Tolerance = float64(pre)
		// ⛔⛔ AND THE RAGGED SPECIAL CASE IS GONE, because the rule now applies.
		//
		// This pass used to raise its own tolerance to maxBadRatio whenever the
		// background was infinite, and then accept a merely-loose answer so long as
		// no line overflowed. Both halves were standing in for one sentence of
		// tex.web (§16790-16793): a line carrying any fil has badness 0 and fitness
		// `decent`, so under \raggedright every line is PERFECT and this pass — which
		// has no hyphens at all — succeeds by the ordinary test.
		//
		// It could not say so while the library flattened a fil into a large finite
		// stretch (go-typeset/linebreak#12 item 1). Since its v1.5.0 the order is
		// carried and \leftskip/\rightskip are the background, so `q.Tolerance` is
		// \pretolerance and nothing else, and `hasBadLine` is the only test needed.
		//
		// ⚠ THIS IS STILL NOT "DO NOT HYPHENATE WHEN RAGGED". TeX hyphenates under
		// \raggedright when a word would otherwise overfull the line; what changes is
		// that it has no reason to otherwise, and an overfull line still has
		// b = awful_bad and sends the paragraph to the next pass.
		plain := e.withParFill(hlist)
		if lines, ok := KnuthPlassShaped(toItems(plain), shape, q); ok && !hasBadLine(lines) {
			return plain, lines, true
		}
	}
	list := e.withParFill(e.hyphenateList(hlist)) // insert discretionary hyphens (if patterns loaded)
	items := toItems(list)
	// ⛔⛔ THE SECOND PASS IS THE FINAL PASS HERE, AND tex.web §863 SAYS SO IN ONE LINE:
	//
	//	second_pass:=true; final_pass:=(emergency_stretch<=0);
	//
	// set on ENTERING the second pass. So the second pass is the final one ONLY
	// when \emergencystretch is zero; with a positive one there is a THIRD pass,
	// and it is that pass which carries the stretch.
	//
	// ⚠ THE FLAG USED TO BE LEFT OFF, with a measured reason that has since been FIXED
	// AT ITS SOURCE. Arming it hyphenated titles — "THETI-TLE" across eleven class
	// tests — because the library's artificial_demerits clause sat OUTSIDE §16769's
	// branch and fired at ordinary breakpoints merely past the threshold, at zero cost,
	// where tex.web says `goto continue`. That, and the forced break recorded whatever
	// its badness, are go-typeset/linebreak#12 item (5) and its neighbour, both fixed
	// in v1.4.0 — which is why this line can now say what TeX says.
	//
	// ⛔ AND IT USED TO SAY \emergencystretch IS ALWAYS ZERO HERE, which was true of
	// this engine and not of the documents: \emergencystretch was accept-and-ignore
	// (go-typeset/linebreak#12 item 6), so `final_pass` was hard-coded true and the
	// third pass could not exist. The library grew the knob in v1.9.0 and this reads
	// the dimen instead.
	//
	// ⭐ IT IS NOT A RARE PARAMETER, WHICH IS WHY THIS MATTERS. latex.go:1376 is
	// LaTeX's own \sloppy:
	//
	//	\def\sloppy{\tolerance 9999\emergencystretch 3em\hfuzz .5pt\vfuzz\hfuzz}
	//
	// so every \sloppy paragraph asks for 3em of emergency stretch. Measured with
	// gotex-paramcensus over the 154-paper list: 4 papers set \emergencystretch
	// directly and 8 more use \sloppy, disjointly — 12 of 154, where counting only
	// the literal name said 4.
	emergency := 0.0
	if sp, ok := e.namedDimen("emergencystretch"); ok && sp > 0 {
		emergency = spToPt(sp)
	}
	p.FinalPass = emergency <= 0
	lines, ok := KnuthPlassShaped(items, shape, p)
	// THE THIRD PASS, §863: re-break with \emergencystretch in the background. TeX
	// runs it only when the second pass left something unset, and only when there
	// is stretch to add — with none, the second pass above was already final and
	// this is unreachable, so a document that does not set the parameter reaches
	// identical code.
	if emergency > 0 && (!ok || hasBadLine(lines)) {
		q := p
		q.FinalPass = true
		q.EmergencyStretch = emergency
		if l3, ok3 := KnuthPlassShaped(items, shape, q); ok3 {
			lines, ok = l3, ok3
		}
	}
	// ⛔⛔ THERE IS NO THIRD, WIDE-OPEN PASS HERE ANY MORE, and this note exists
	// because for a while the comment in its place said so three times over, in
	// three tenses — "the wide-open pass STAYS", "AND IT IS GONE", "SO WHY IS IT
	// STILL HERE" — each generation appended above or below the last.
	//
	// ⛔ A reader cannot tell which tense is current, and one did not: go-tex/engine#754's
	// audit of the delegated layers reports that this function "raises the tolerance
	// to 1e4 instead (paragraph.go:251-258)", citing line numbers that by then
	// pointed INTO the comment rather than at code. The pass was already gone. A
	// stale comment cost a published audit one of its six findings, which is what a
	// dated claim does when it is left beside a newer one.
	//
	// What follows is the whole story once, in order.
	//
	// WHAT IT WAS. If the pass above returned a bad line, run the paragraph again at
	// \tolerance=maxBadRatio and adopt that. Nothing in tex.web does this: §863 has
	// exactly three passes — \pretolerance without hyphens, \tolerance with them, and
	// a third that adds \emergencystretch to the BACKGROUND — and none raises the
	// tolerance. Raising it says "every line is acceptable however badly it fits",
	// which is a different algorithm with a different answer, and its answer was:
	// never leave a line overfull, take more hyphens instead.
	//
	// WHAT IT COST, measured on #768's witness (\hsize=45pt, \fussy, `Typographical
	// incomprehensibilities overwhelm every paragraph.`) against tectonic 0.17.0,
	// with the face, the interword space and \tolerance verified identical:
	//
	//	reference      5 lines, FOUR overfull boxes (18.1, 13.92, 14.3, 18.16pt)
	//	with this pass 7 lines, every one of them fitting
	//	without it     5 lines, and lines 1-3 the reference's to 0.02pt
	//	               (`Typographical`, `incomprehen-`, `sibilities over-`)
	//
	// Pass 2 had ALREADY FOUND the reference's answer and this one threw it away,
	// because `hasBadLine` is true of a solution TeX deliberately chose: at
	// \tolerance=200 a short line with no stretch has badness 10000 and is not
	// feasible, so TeX overhangs the margin rather than take the hyphens.
	//
	// WHY IT COULD NOT SIMPLY GO. One removal attempt (#778) turned six tests red,
	// four of them saying the same thing — a word that MUST be hyphenated stopped
	// being hyphenated (TestRaggedTextStillHyphenatesAWordThatCannotFit,
	// TestAnExceptionBreaksWhereTheDocumentSaysAndNowhereElse,
	// TestHyphenationInsertsHyphen, TestPageIsOneTextElement). That was
	// go-typeset/linebreak#12 item (1) seen from the consumer: the library carried ONE
	// float of stretch and flattened `0pt plus 1fil` into a large finite number, so
	// under \raggedright a line TeX scores at badness 0 was scored 10000 here and was
	// INFEASIBLE. TeX's pass 2 had a solution and hyphenated inside it; ours had none,
	// fell to artificial demerits, and took the single overfull line. The wide-open
	// tolerance hid that by making everything feasible — it was treating the symptom.
	//
	// AND WHY IT IS GONE. The library's v1.5.0 carries the glue orders and makes
	// \leftskip/\rightskip the background, which removes the cause; #780 consumed
	// that here, and the pass went with it. #768's finding (2) was this pass.
	//
	// ⚠ The two KnuthPlassShaped calls above are therefore the whole of it:
	// \pretolerance, then \tolerance with FinalPass set when \emergencystretch is
	// zero, plus §863's genuine third pass when it is not.
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
	// ⛔ AN EARLIER VERSION OF THIS COMMENT ARGUED THAT len(e.skip) WAS RIGHT HERE,
	// because the fallback above is LaTeX's default rather than the register's zero.
	// That reasoning was wrong, and the way it was wrong is worth keeping: len asks
	// "stored yet", so it would have answered the SAME unstored register two ways —
	// a low one reads 0pt through the index, a high one falls back to 0pt plus 1fil.
	// Whatever the right default is, it cannot depend on which register number the
	// allocator happened to hand out. registerPoolSize asks "legal", consistently,
	// and the fallback is left for a name that is not a skip register at all.
	if m := e.eq["parfillskip"]; m != nil && m.kind == mSkipRef && m.code >= 0 && m.code < registerPoolSize {
		fill = e.getSkip(m.code)
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

// skipBackground is tex.web §16307-16315: \leftskip and \rightskip are not items
// in the list, they are the BACKGROUND every line's totals start from.
//
//	q:=left_skip; r:=right_skip; background[1]:=width(q)+width(r);
//	background[2+stretch_order(q)]:=stretch(q);
//	background[2+stretch_order(r)]:=background[2+stretch_order(r)]+stretch(r);
//	background[6]:=shrink(q)+shrink(r);
//
// ⛔⛔ THIS IS WHERE \raggedright BELONGS, AND IT USED TO BE TWO WORKAROUNDS.
// The skips were added to each line AFTER breaking (applyLineSkips), so the
// breaker never saw them: it broke the paragraph justified and the ragged glue
// was painted on afterwards. Two patches stood in for this — the measure was
// never narrowed by \leftskip (#583), and the first pass had its TOLERANCE
// opened when the background happened to be infinite, to imitate the badness 0
// that §16790-16793 gives such a line outright.
//
// With the background where TeX puts it, both become unnecessary: a fil in the
// background makes every line perfect by the rule rather than by a special case,
// and the width narrows the measure because that is what `background[1]` is.
//
// ⚠ Shrink is summed without order: §16806, "Shrinkability is never infinite in
// a paragraph".
// ⛔⛔ AND THE WIDTH IS DELIBERATELY NOT CARRIED, BECAUSE THE CALLER ALREADY
// TOOK IT OFF THE MEASURE. #780 added this background and set Width here too,
// which counted \leftskip and \rightskip TWICE: breakSegment is handed
//
//	lineWidth := spToPt(e.hsize - e.leftskip.width - e.rightskip.width)
//
// at paragraph.go:81, and the background was then added to every line's natural
// width on top of that.
//
// tex.web keeps line_width at \hsize and puts the widths in background[1]
// (§16307-16315); this engine subtracts them from the measure instead. The two
// are the same arithmetic and either is faithful — doing BOTH is not.
//
// Measured on #583's own witness (`\rightskip=100pt` on a 343.7pt measure, the
// right margin at 477.50):
//
//	reference      3 lines, `A fairly long subsection title that has to wrap onto a`, x1 377.85
//	before #780    3 lines, the same split, x1 377.87  — matched
//	#780           5 lines, `A fairly long subsection title that`, stretched to 377.87
//	this           3 lines, the reference's split again
//
// The margin was right in all of them; what the double count moved was how much
// text the breaker thought would fit, and five stretched lines where the
// reference sets three is the visible cost. #583's first defect was already
// fixed before #780 and #780 reopened it.
func (e *Engine) skipBackground() LineBackground {
	var bg LineBackground
	for _, s := range []glueSpec{e.leftskip, e.rightskip} {
		bg.Stretch[glueOrder(s.stretchOrder)] += spToPt(s.stretch)
		bg.Shrink += spToPt(s.shrink)
	}
	return bg
}

// raggedBackground reports whether the glue that will be added to every line
// carries INFINITE stretch — \raggedright's `0pt plus 1fil`, and \centering's
// pair of them.
//
// ⚠ IT IS THE ORDER, NOT THE AMOUNT. A \rightskip of `0pt plus 100pt` is a very
// stretchy margin and still a finite one: TeX balances looseness against it as
// usual. Only an infinite order collapses badness to 0 (§852), and that is the
// distinction \raggedright turns on.
func (e *Engine) raggedBackground() bool {
	return e.rightskip.stretchOrder > 0 || e.leftskip.stretchOrder > 0
}

// hasBadLine reports whether any line is overfull or badly underfull (a ratio
// well outside the normal [-1, small] range — the capped/infinite bad values).
//
// ⛔⛔ AND A RATIO IS NOTHING WITHOUT ITS ORDER. This read `ln.Ratio > 100`
// alone, which was harmless only while a fil was flattened into the finite
// stretch column: the \parfillskip's 1e6 made the last line's ratio come out
// near zero, by accident.
//
// With the orders real (go-typeset/linebreak v1.5.0), Ratio on such a line is
// measured against its ONE unit of fil, so an ordinary last line taking up 300pt
// of slack reports 300 — and this said "bad line" about EVERY paragraph. The
// first pass therefore failed on all of them and every document went to the
// hyphenating pass, which is a far larger change than the one being made: the
// symptom that found it was a space vanishing between a formula and the next
// word, three breaks away.
//
// tex.web has no such ambiguity because it never divides: §16790-16793 gives a
// line with any infinite stretch b = 0 and asks no further question. So the
// test applies to the finite order only — and overfull stays unconditional,
// because shrinking has no orders at all (§16806).
func hasBadLine(lines []Line) bool {
	for _, ln := range lines {
		if ln.Ratio < -1 {
			return true // overfull past its shrink, at any order
		}
		if ln.StretchOrder == 0 && ln.Ratio > 100 {
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
			// ⛔⛔ THE ORDER IS CARRIED, NOT FLATTENED. This read
			//
			//	if c.spec.stretchOrder > 0 { st = 1e6 }
			//
			// which turned `plus 1fil` into a large FINITE stretch. tex.web
			// §16790-16793 gives any line carrying fil/fill/filll badness 0 and
			// fitness `decent` — it is perfect, not merely very good — so a flattened
			// fil came out with a large but finite badness and a \raggedright
			// paragraph failed the first pass and took the hyphens of a justified one.
			// go-typeset/linebreak carries the four columns since v1.5.0 (its #12
			// item 1), so the order can simply be passed through.
			//
			// ⚠ SHRINK STILL HAS NO ORDERS, and that is tex.web's own sentence
			// (§16806): "Shrinkability is never infinite in a paragraph". A
			// `minus 1fil` is still flattened, because the breaker has nowhere to put
			// it; it can only ever make a line narrower, and a line being shrunk has
			// no infinite case at all.
			sh := spToPt(c.spec.shrink)
			if c.spec.shrinkOrder > 0 {
				sh = 1e6
			}
			items[i] = GlueOrder(spToPt(c.spec.width), spToPt(c.spec.stretch),
				glueOrder(c.spec.stretchOrder), sh)
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
