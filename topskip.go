// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \topskip is the glue TeX puts above the FIRST box or rule of a page, with its
// natural width reduced by that box's height (tex.web §1001, "Initialize the
// current page, insert the \topskip glue"):
//
//	q := new_skip_param(top_skip_code);
//	if width(temp_ptr) > height(p) then width(temp_ptr) := width(temp_ptr)-height(p)
//	else width(temp_ptr) := 0;
//	link(q) := p
//
// So the first BASELINE of a page sits \topskip from the top of the text block
// whenever the line is shorter than that, and a taller first line pushes down
// from the top with no glue at all. Its stretch and shrink are kept as stated;
// only the natural width is reduced. article sets \topskip to 10pt
// (size10.clo:7).
//
// ⛔ WITHOUT IT EVERY PAGE STARTS TOO HIGH, and a page count cannot see it: in
// ordinary prose the first line is about 7pt tall, so the missing glue is ~3pt —
// never a whole line, so no page gains or loses one, and Σ stays put while every
// page is 3pt out. Measured on a SYMMETRIC witness, which is what makes the
// number trustworthy: the same document with a 2pt first line and with a 40pt
// first line, and the DIFFERENCE between the two positions taken inside each
// engine, so the word-box bias that makes poppler's boxes incomparable between
// engines (a constant ratio of 1.41) cancels out.
//
//	first line     ours      reference
//	2pt          127.50      138.69
//	40pt         165.36      168.58
//	difference    37.86       29.89
//
// The reference's 29.89 is 40−10: the 40pt line gets no glue and the 2pt line is
// floored at \topskip, so the gap between the two cases is \topskip less than the
// gap between the heights. Ours was 37.86 = 40−2, the raw height difference, which
// is the signature of no \topskip at all.
//
// It is applied in TWO places that must agree — assemblePage, which puts the glue
// on the page, and findPageBreak, which has to count its height or the page it
// chose overflows once the glue arrives. Both call topskipGlueFor, and
// TestTopskipHeightAndInsertionAgree asserts they cannot drift apart.
func (e *Engine) topskipGlueFor(list []node) (glueNode, int, bool) {
	for i, n := range list {
		h, ok := topskipFirstHeight(n)
		if !ok {
			continue
		}
		// The register's own value, whatever it holds — including a deliberate 0pt.
		// Its DEFAULT of 10pt is set where the register is allocated
		// (loadTeXParams, beside \parfillskip's), not patched up here: a fallback
		// in this function would be unreachable, since a register is always
		// allocated, and it would have to read 0 as "absent" to fire at all.
		spec, _ := e.topskipSpec()
		if spec.width > h {
			spec.width -= h
		} else {
			spec.width = 0
		}
		return glueNode{spec: spec}, i, true
	}
	return glueNode{}, 0, false
}

// topskipSpec is the \topskip register's glue and whether the register EXISTS.
//
// ⛔ The second result is not ceremony. namedSkip returns the zero glue both for a
// register holding 0pt and for a name that is not a register at all, and reading
// a zero as "absent" means a document that deliberately writes \topskip=0pt — a
// legitimate thing to do, and what a test needs in order to have a control —
// silently gets 10pt instead. A test asking for exactly that is what found it.
func (e *Engine) topskipSpec() (glueSpec, bool) {
	m := e.eq["topskip"]
	if m == nil || m.kind != mSkipRef || m.code < 0 || m.code >= len(e.skip) {
		return glueSpec{}, false
	}
	return e.skip[m.code], true
}

// topskipFirstHeight is the HEIGHT of a node that ends \topskip's search, and
// whether this node is one.
//
// ⛔ The height alone, never height+depth: §1001 reduces the glue by height(p),
// because what is being positioned is the first BASELINE. Charging the depth too
// would raise every page's first line by the descender of whatever happens to be
// on it.
//
// Glue, kern and penalty nodes before the first box are what TeX discards while
// page_contents is still empty (§1000), so they are skipped here rather than
// ending the search.
func topskipFirstHeight(n node) (int, bool) {
	switch c := n.(type) {
	case *boxNode:
		return c.height, true
	case ruleNode:
		return c.height, true
	case spanNode:
		if c.box != nil {
			return c.box.height, true
		}
		return 0, true
	}
	return 0, false
}

// withTopskip returns the page with its \topskip glue inserted before the first
// box, or unchanged when the page holds no box at all.
func (e *Engine) withTopskip(page []node) []node {
	g, at, ok := e.topskipGlueFor(page)
	if !ok {
		return page
	}
	out := make([]node, 0, len(page)+1)
	out = append(out, page[:at]...)
	out = append(out, g)
	out = append(out, page[at:]...)
	return out
}

// topskipHeightAt is the vertical room the \topskip of a page starting at
// list[start] will take, for the page breaker to charge before it measures
// anything else.
func (e *Engine) topskipHeightAt(list []node, start int) int {
	if start >= len(list) {
		return 0
	}
	g, _, ok := e.topskipGlueFor(list[start:])
	if !ok {
		return 0
	}
	return g.spec.width
}
