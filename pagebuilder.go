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

// vKind is how a node moves TeX's page_total / page_depth pair.
type vKind int

const (
	vNothing  vKind = iota // penalty, mark, whatsit: neither value moves
	vBoxLike               // a box or rule (tex.web:19519-19520)
	vGlueLike              // glue or kern (tex.web:19550)
)

// pageSoFar is TeX's page_total and page_depth (tex.web:19226-19229), which are
// TWO numbers and not one.
//
// ⛔ page_depth IS NOT CHARGED AGAINST page_goal. The badness that decides the
// break compares page_total alone (tex.web:19594-19599); the depth of the last box
// on the page enters page_total only when the NEXT box or glue arrives
// (tex.web:19519, 19550), and is capped at \maxdepth with the excess charged
// (tex.web:19525-19529). vert_break does the same for \vsplit, with cur_height and
// prev_dp (tex.web:18967, 19001) — and tex.web:18995 is the whole point in one
// line: the box that comes out of the break is cur_height+prev_dp tall, while the
// DECISION is taken on cur_height.
//
// Charging height+depth in one sum, as this did, loses a line on any page that
// fills exactly. Measured on a witness with nothing unusual set (default \parskip,
// ordinary prose, one column, article 10pt): \textheight 550pt, \baselineskip 12pt,
// \topskip 10pt — all ten vertical registers read identical to the reference's from
// inside both engines — and the reference puts 46 lines on page 2 because
// 10 + 45×12 = 550 is EXACTLY the goal and equality is accepted. We stopped at 45,
// with the first baseline matching to 0.02pt and the last exactly one \baselineskip
// higher, because the 46th line's own ~1.9pt descender was charged as if it had to
// fit above the bottom of the text block. It does not: that is what \maxdepth's 5pt
// is for. (go-tex/engine#622)
// ⛔⛔ AND THE PAGE'S GLUE, which this tracked none of. TeX keeps eight quantities in
// page_so_far (tex.web:19226-19232): the goal, the total, THREE ORDERS OF INFINITE
// STRETCH plus the finite one, the shrink, and the depth. This struct had two of
// them, so pageBadness could only ever be a function of two heights — and TeX's
// badness is a function of the shortfall OVER THE GLUE AVAILABLE TO ABSORB IT
// (:19593-19599). The two ask different questions: a page ending in \vfill is badness
// 0 to TeX and was ~10000 here. Since the breakpoint cost is `badness + penalty` on
// both sides, every penalty was being compared against numbers on the wrong scale —
// which is why contributing faithful list penalties made the corpus measurably worse
// (go-tex/engine#685, #687).
type pageSoFar struct {
	total   int    // page_total: charged against \vsize
	depth   int    // page_depth: NOT charged, capped at \maxdepth
	stretch [4]int // page_so_far[2..5]: finite, fil, fill, filll
	shrink  int    // page_so_far[6]
}

// add moves the pair for one node, exactly as tex.web:19519-19529 and 19550 do.
func (p *pageSoFar) add(n node, maxDepth int) {
	// The glue a node carries is accumulated whatever its kind: tex.web:19542-19548
	// adds the stretch and shrink of every glue node contributed to the page, and
	// 19550 then moves the heights. A box contributes none of its own.
	if g, ok := n.(glueNode); ok {
		o := g.spec.stretchOrder
		if o < 0 || o >= len(p.stretch) {
			o = 0
		}
		p.stretch[o] += g.spec.stretch
		p.shrink += g.spec.shrink
	}
	h, d, kind := vHeightDepth(n)
	switch kind {
	case vBoxLike:
		p.total += p.depth + h
		p.depth = d
		if p.depth > maxDepth {
			p.total += p.depth - maxDepth
			p.depth = maxDepth
		}
	case vGlueLike:
		p.total += p.depth + h
		p.depth = 0
	}
}

// badnessOf is tex.web:19593-19599, the badness of the page as it stands.
//
//	if page_total<page_goal then
//	  if (page_so_far[3]<>0) or (page_so_far[4]<>0) or (page_so_far[5]<>0) then b:=0
//	  else b:=badness(page_goal-page_total,page_so_far[2])
//	else if page_total-page_goal>page_shrink then b:=awful_bad
//	else b:=badness(page_total-page_goal,page_shrink)
//
// ⛔ ANY INFINITE STRETCH MAKES THE PAGE PERFECT, not merely good: a page that can be
// filled exactly is never a bad place to stop, so TeX scores it 0 and lets the
// penalties alone decide. A \vfill at the end of a chapter is the everyday case.
//
// ⛔ AND OVERFULL IS NOT "TALLER THAN \vsize". It is taller by more than the page's
// glue can SHRINK. Refusing every h > vsize, as this did, throws away pages TeX
// accepts by tightening them.
func (p *pageSoFar) badnessOf(goal int) float64 {
	if p.total < goal {
		if p.stretch[1] != 0 || p.stretch[2] != 0 || p.stretch[3] != 0 {
			return 0
		}
		return float64(badness(goal-p.total, p.stretch[0]))
	}
	if p.total-goal > p.shrink {
		return awfulBad
	}
	return float64(badness(p.total-goal, p.shrink))
}

// vContribution is a node's contribution to the running page height (sp).
//
// It is the height AND the depth together — the natural size of the material,
// which is what a \vbox of it measures. The page breaker must not use it; it keeps
// the two apart through pageSoFar, for the reason written there.
func vContribution(n node) int {
	h, d, _ := vHeightDepth(n)
	return h + d
}

// vHeightDepth splits a node's vertical size into the height and the depth, and
// says which of TeX's three cases the node falls in.
func vHeightDepth(n node) (int, int, vKind) {
	switch c := n.(type) {
	case *boxNode:
		return c.height, c.depth, vBoxLike
	case spanNode: // a stray full-width band (normally lifted out by the two-column pager)
		if c.box != nil {
			return c.box.height, c.box.depth, vBoxLike
		}
		return 0, 0, vBoxLike
	case ruleNode:
		return c.height, c.depth, vBoxLike
	case kernNode:
		return c.width, 0, vGlueLike
	case glueNode:
		return c.spec.width, 0, vGlueLike
	case charNode:
		return c.height, c.depth, vBoxLike
	case mathNode:
		return c.height, c.depth, vBoxLike
	case imageNode:
		return c.height, c.depth, vBoxLike
	case frameNode:
		return c.height(), c.depth(), vBoxLike
	case decoNode:
		return c.height(), c.depth(), vBoxLike
	case transformNode:
		return c.height(), c.depth(), vBoxLike
	case linkNode:
		return c.height(), c.depth(), vBoxLike
	case internalLinkNode:
		return c.height(), c.depth(), vBoxLike
	case footnoteNode:
		// The note's own height, so the page breaks early enough to leave room for
		// the foot area even though the note is not painted inline (assemblePage
		// lifts it to the page bottom). The space the foot area itself costs —
		// \skip\footins — is charged ONCE per page, by findPageBreak, not here:
		// see footinsSkip.
		//
		// ⛔ Charged as a BOX and not as glue: a note reserves vertical room, so its
		// depth must be capped and carried like a box's. It is not painted here, so
		// nothing downstream reads the depth — but routing it through vGlueLike would
		// also ZERO page_depth, handing the page back the descender of the line the
		// note hangs off.
		return c.body.height, c.body.depth, vBoxLike
	}
	return 0, 0, vNothing
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
	// ⛔ The page starts with its \topskip glue, so its height is charged BEFORE
	// anything else. Left out, the break is chosen for a page that is then handed
	// one more glue node by assemblePage and overflows by that much — and the two
	// sites would disagree about what the page holds. Both go through
	// topskipGlueFor; TestTopskipHeightAndInsertionAgree asserts it.
	// page_total starts at the \topskip glue's width; page_depth at zero, since
	// glue zeroes it (tex.web:19550) and nothing has been contributed yet.
	page := pageSoFar{total: e.topskipHeightAt(list, start)}
	maxDepth := e.pageMaxDepth()
	sawFootnote := false
	best := -1
	least := math.Inf(1)
	for i := start; i < len(list); i++ {
		if ok, pen := legalPageBreak(list, i, start); ok {
			if pen <= -10000 {
				return i // forced break
			}
			// ⛔ awful_bad, not +Inf: tex.web:19556-19560 compares b against
			// awful_bad as a NUMBER, and :19572 fires the page when the cost reaches
			// it. A page whose excess exceeds its own shrink is beyond saving; one
			// that is merely tall is not.
			//
			// ⛔⛔ AND THIS IS THE ONLY THING THAT FIRES A FULL PAGE. build_page has no
			// "the total passed the goal" test anywhere (tex.web:19425-19432 loops
			// until the contribution list is empty; :19572 fires on c=awful_bad or a
			// forced penalty, and nothing else). This loop also returned as soon as
			// page_total crossed \vsize, at whatever node did the crossing — which
			// throws away the half of badnessOf that exists for exactly this moment:
			// a page is overfull when its excess beats its SHRINK, not when it is
			// merely taller than \vsize. With the early return in place, every page
			// whose glue could have been tightened stopped one item short, and the
			// shrink was computed and then never consulted.
			b := page.badnessOf(vsize)
			if b >= awfulBad { // overfull beyond what the page's glue can shrink
				if best >= 0 {
					return best
				}
				return i
			}
			// ⛔⛔ A PAGE THAT IS INFINITELY BAD COSTS `deplorable`, AND THE PENALTY DOES
			// NOT BUY IT BACK. tex.web:19556-19560 is a three-way choice, not a sum:
			//
			//	if pi<=eject_penalty then c:=pi
			//	else if b<inf_bad then c:=b+pi+insert_penalties
			//	else c:=deplorable
			//
			// The middle line — the one this had — applies ONLY while b < inf_bad. Once
			// the page is infinitely bad, every breakpoint on it costs the same 100000
			// whatever penalty sits there, so TeX stops letting a negative penalty make a
			// nearly-empty page look attractive. Summing instead, as this did, scored a
			// \penalty-300 on a badness-10000 page at 9700 — cheaper than a perfectly
			// full page at any positive penalty.
			//
			// ⚠ AND IT CHANGES NOTHING ON THE CORPUS, MEASURED TWICE. 154 arXiv papers
			// against tectonic, on this page builder and again on the one that fires a
			// page at a breakpoint rather than at the node that crosses \vsize (#693):
			// ZERO documents moved, both times — not an unchanged Σ that hides
			// compensating moves, zero displacement.
			//
			// This comment first claimed the rule was "the arithmetic behind" the three
			// measured regressions of #685, #690 and #691, which are all negative
			// penalties faithful to latex.ltx. That is an explanation nothing supports:
			// it was written before the sweep and the sweep refused it. The reason the
			// clause cannot reach those cases is visible in the rule itself — ties go to
			// the LATER breakpoint (`c <= least`), so when every candidate on a hopeless
			// page costs the same deplorable, the champion is the same one the sum would
			// have picked. It bites only where a negative penalty sits EARLIER than a
			// zero-penalty breakpoint on a page that stays infinitely bad, which the
			// test below builds deliberately and the corpus does not contain.
			//
			// It ships as a port, not as a fix: the equality oracle is tex.web, and a
			// faithful rule with no measured effect is worth more than an unfaithful one
			// with none.
			c := b + float64(pen)
			if b >= infBad {
				c = deplorable
			}
			if c <= least {
				least, best = c, i
			}
		}
		page.add(list[i], maxDepth)
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
			page.total += e.footinsSkip()
		}
	}
	// ⛔⛔ THE END OF THE LIST IS A BREAKPOINT, AND IT IS WHAT FIRES THE LAST PAGE.
	// This is not an afterthought to the change above: with build_page's own firing
	// rule and nothing else, a page that overflows with no breakpoint left after it
	// would simply never fire. tex.web:20486-20500 is how TeX gets there —
	//
	//	tail_append(new_null_box); width(tail):=hsize;
	//	tail_append(new_glue(fill_glue));
	//	tail_append(new_penalty(-@'10000000000));
	//	build_page;
	//
	// \end does not hand the residue to the shipper. It appends an empty box, a
	// \vfill and a forced penalty and runs build_page ONE MORE TIME, so the last
	// page is decided at a real breakpoint like every other page. Two details of
	// that fixture matter here and both are read off the order in :19457-19470:
	//
	//   - the null box charges the pending page_depth into page_total (:19519), so
	//     the depth of the last box on the page is finally paid;
	//   - the \vfill is a legal breakpoint whose badness is taken BEFORE its own
	//     stretch is added (update_heights runs after the champion test), so the
	//     \vfill does not rescue a page that is already too tall.
	//
	// So: an overfull page fires at its champion, and a page that still fits keeps
	// everything to the end. 0x3FFFFFFF is awful_bad, @'7777777777 in WEB.
	final := page
	final.total += final.depth
	final.depth = 0
	if best >= 0 && final.badnessOf(vsize) >= 0x3FFFFFFF {
		return best
	}
	return len(list)
}

// pageMaxDepth is TeX's page_max_depth: \maxdepth as it stood when the page specs
// were frozen (tex.web:19376 and freeze_page_specs).
//
// ⛔ Zero is a VALUE here, not a missing reading. INITEX leaves \maxdepth at 0pt
// and plain.tex sets 4pt, article 5pt (size10.clo, .5\topskip); a cap of 0 charges
// every descender immediately, which is what this engine did everywhere before.
// So a default of 0 is both faithful and the control: the same document with
// \maxdepth=0pt must paginate as it did, and only a document that sets \maxdepth
// gains the line back.
//
// A NEGATIVE \maxdepth is legal (tex.web:18944-18946 raises it explicitly) and
// needs no special case: the cap in pageSoFar.add then charges page_total MORE
// than the box's depth, which is what TeX does.
func (e *Engine) pageMaxDepth() int {
	d, _ := e.namedDimen("maxdepth")
	return d
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
		if i > start && precedesBreak(list[i-1]) {
			return true, 0
		}
	}
	return false, 0
}

// precedesBreak is TeX's precedes_break, and it is a test on the node TYPE:
//
//	@d precedes_break(#)==(type(#)<math_node)        tex.web:3109
//
// `math_node = 9` (`:3092`), so it is true for every type below it: hlist 0, vlist 1,
// rule 2, ins 3, mark 4, adjust 5, ligature 6, disc 7 — and **whatsit 8** (`:3085`).
// build_page applies it to the last node already on the page before it will accept a
// glue node as a breakpoint (`:19496`).
//
// ⛔⛔ THIS MUST AGREE WITH vHeightDepth ABOUT WHAT A BOX IS, and the predicate it
// replaced did not. vHeightDepth calls twelve kinds of node box-like — boxes, rules,
// chars, display maths, spans, images, frames, decorations, transforms, both kinds of
// link and a footnote — and charges each one's height and depth to page_total as TeX
// charges an hlist's. The breakpoint test named FOUR of them. So the page builder
// would add an image's height to the page and then refuse to break after it: a page
// carrying a figure, a framed block or a link kept growing past the only place TeX
// would have stopped it. Two functions that classify the same node must classify it
// the same way, and the way to make that true is to have one of them answer, not to
// keep two lists in step.
//
// ⚠ mathNode is the one place this engine's model and TeX's numbering disagree, and
// box-like is the right answer here: TeX's math_node is an inline on/off marker that
// lives in a HORIZONTAL list and never reaches the main vertical list, while a
// mathNode on this engine's vertical list is a display equation — which TeX sets as
// an hlist, type 0.
func precedesBreak(n node) bool {
	// A whatsit is type 8 and 8 < 9: \special, \write and friends do not stop a break
	// from being taken after them. They are not box-like, so vHeightDepth says
	// nothing about them.
	if _, ok := n.(specialNode); ok {
		return true
	}
	_, _, kind := vHeightDepth(n)
	return kind == vBoxLike
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

// splitVList splits a vertical list at the first break that keeps the height within
// vsize, returning the taken part and the remainder AS IT STANDS.
//
// It is TeX's vert_break, and the division of labour is TeX's too: vert_break chooses
// the break and touches nothing, and it is vsplit that calls prune_page_top on the
// remainder afterwards (tex.web:19059 then 19062). Keeping the pruning out of here is
// not tidiness — multicols slices through this function as well, and its slices must
// NOT be pruned. The oracle established that; balanceVList records it with the
// numbers.
//
// The running height is page_total and page_depth kept apart, capped at
// \splitmaxdepth: vert_break's cur_height and prev_dp with its d argument
// (tex.web:18959-18968), and vsplit passes split_max_depth for that d
// (tex.web:19059). Summing the two refuses a column that fills exactly, for the
// reason written on pageSoFar.
//
// ⚠ The break is still FIRST-FIT and not vert_break's least-cost choice, so a
// \penalty inside the list is not honoured. That is a second divergence with its own
// witness (a \penalty that should move a break) and it is deliberately not addressed
// here: two mechanisms in one change cannot be attributed, and this repository has
// already paid for a fix in two halves whose halves were never mutated separately.
func (e *Engine) splitVList(list []node, vsize int) ([]node, []node) {
	maxDepth := e.splitMaxDepth()
	var page pageSoFar
	for i, n := range list {
		probe := page
		probe.add(n, maxDepth)
		if probe.total > vsize && page.total > 0 {
			// ⛔⛔ THE GLUE OR KERN AT THE BREAK IS DISCARDED, and removing this line
			// is what misaligned multicols' columns. vert_break itself discards
			// nothing — vsplit prunes afterwards — but multicols does not prune, so
			// this is the only thing that keeps the leftover glue off the head of the
			// next column. Taken out, the witness went from the oracle's ten SHARED
			// baselines to twenty offset ones, median step 2.58pt instead of 11.96.
			switch n.(type) {
			case glueNode, kernNode:
				return list[:i], list[i+1:]
			}
			return list[:i], list[i:]
		}
		page = probe
	}
	return list, nil
}

// splitMaxDepth is \splitmaxdepth, the cap vsplit passes to vert_break
// (tex.web:19059). Zero is a value, not a missing reading, exactly as for \maxdepth:
// see pageMaxDepth.
func (e *Engine) splitMaxDepth() int {
	d, _ := e.namedDimen("splitmaxdepth")
	return d
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
	// vert_break chooses the break, and then vsplit prunes the remainder
	// (tex.web:19059 then 19062) — in that order and here, not inside splitVList,
	// because multicols shares the splitter and must not be pruned. The register then
	// gets the list TeX would put there: no discarded glue, a \splittopskip above its
	// first box, vpacked at its natural height (tex.web:19064).
	top, rest := e.splitVList(src.list, h)
	rest = e.prunePageTop(rest)
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
