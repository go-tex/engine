// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \footnote — a simplified form of TeX's insertion
// mechanism. \footnote{text} places a raised reference number where it is called
// and typesets a numbered note that migrates to the bottom of the page the marker
// falls on. The note travels as a footnoteNode on the main vertical list whose
// page-height contribution reserves room for the note (so the page breaks early
// enough); the page builder then lifts the notes out of the content flow and
// stacks them below a short rule at the foot of the page.
//
// Not modelled (future work, matching the page-builder's own TODOs): splitting a
// long note across pages, \footnotemark/\footnotetext separation, and per-note
// \footnotesize. A note is set at the body size.

import "strconv"

// footnoteNode carries a rendered footnote body down the main vertical list. It
// contributes its height to the page (reserving space) but is not painted inline;
// the page builder collects it into the foot of the page (see assemblePage).
type footnoteNode struct{ body *boxNode }

func (footnoteNode) isNode() {}

// footinsSkip is \skip\footins: the space a page gives up to the foot area the
// first time a footnote lands on it. ONCE per page, not once per note —
// tex.web:19638 reduces page_goal by width(skip n) inside "Create a page insertion
// node", which runs only for the FIRST \insert n of a page; every later insert
// costs its own height and nothing more (l.19613).
//
// size1x.clo:203 states it per class size (9 / 10 / 10.8pt); setPtsize puts those
// in the register. A zero reading means nobody set it — the register is allocated
// by the class kernel and was never given a value — so the 10pt default stands in
// rather than being taken for a document that wants no space at all.
func (e *Engine) footinsSkip() int {
	if m := e.eq["footins"]; m != nil && m.kind == mCountRef && m.code >= 0 && m.code < len(e.count) {
		if n := e.count[m.code]; n >= 0 && n < len(e.skip) {
			if w := e.skip[n].width; w > 0 {
				return w
			}
		}
	}
	return 9 * unity // size10.clo:203
}

// setFootinsSkip stores v in \skip\footins, through the \footins count the way a
// class would write it.
func (e *Engine) setFootinsSkip(v int) {
	if m := e.eq["footins"]; m != nil && m.kind == mCountRef && m.code >= 0 && m.code < len(e.count) {
		if n := e.count[m.code]; n >= 0 && n < len(e.skip) {
			e.skip[n] = glueSpec{width: v, stretch: 4 * unity, shrink: 2 * unity}
		}
	}
}

// doFootnote implements \footnote{text}: step the counter, typeset the numbered
// note into a vbox held until the enclosing paragraph attaches it to the vertical
// list, and drop a raised reference number at the current point.
func (e *Engine) doFootnote() {
	text := e.grabUndelimited()
	e.footnoteCounter++
	n := e.footnoteCounter

	// Body = \footnotesize "N. " + text, set as a mini-paragraph to the body
	// width (the size is scoped to the sandbox by typesetGroupToVbox).
	label := []tok{csTok("footnotesize")}
	label = append(label, numberToks(n)...)
	label = append(label, chTok('.', catOther), chTok(' ', catSpace))
	body := e.typesetGroupToVbox(append(label, text...))
	e.pendingFootnotes = append(e.pendingFootnotes, body)

	// Inline raised reference number.
	if e.curFont != nil {
		if !e.inPar {
			e.beginParagraph(true)
		}
		e.parList = append(e.parList, e.footnoteMarker(n))
	}
}

// footnoteMarker builds the raised reference number placed inline at the \footnote
// call — the digits packed into an hbox shifted up by ~0.4em.
func (e *Engine) footnoteMarker(n int) node {
	var mk []node
	for _, r := range strconv.Itoa(n) {
		w, h, d := e.curFont.charDimsSP(r)
		mk = append(mk, charNode{ch: r, width: w, height: h, depth: d, srcLine: e.curSrcLine})
	}
	box := hpackSP(mk, packNatural, 0)
	box.shift = -(e.curFont.sizePt() * unity * 2 / 5) // negative shift raises (superscript)
	return box
}

// numberToks turns an integer into a run of digit character tokens.
func numberToks(n int) []tok {
	var out []tok
	for _, r := range strconv.Itoa(n) {
		out = append(out, chTok(r, catOther))
	}
	return out
}

// typesetGroupToVbox runs a token list through the main loop in isolation and
// returns the material it produced as a vbox — used to set a footnote body. The
// engine's horizontal/vertical state is saved and restored, and buildingFootnote
// guards endParagraph from flushing pending notes into this sandbox.
func (e *Engine) typesetGroupToVbox(toks []tok) *boxNode {
	savedMvl, savedPar, savedIn, savedPD := e.mvl, e.parList, e.inPar, e.prevDepth
	savedBuilding, savedFont := e.buildingFootnote, e.curFont
	e.mvl, e.parList, e.inPar, e.prevDepth = nil, nil, false, ignoreDepth
	e.buildingFootnote = true

	e.push(append(append([]tok{}, toks...), csTok("par"), sentinel))
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			break
		}
		if !e.stepToken(t) {
			break
		}
	}
	e.endParagraph()
	body := vpackSP(e.mvl, packNatural, 0)

	e.mvl, e.parList, e.inPar, e.prevDepth = savedMvl, savedPar, savedIn, savedPD
	e.buildingFootnote, e.curFont = savedBuilding, savedFont
	return body
}

// flushFootnotes moves any notes accumulated during the just-finished paragraph
// onto the main vertical list (as footnoteNodes, right after the paragraph that
// referenced them). No-op inside a footnote's own build.
func (e *Engine) flushFootnotes() {
	if e.buildingFootnote || len(e.pendingFootnotes) == 0 {
		return
	}
	for _, b := range e.pendingFootnotes {
		e.mvl = append(e.mvl, footnoteNode{body: b})
	}
	e.pendingFootnotes = e.pendingFootnotes[:0]
}

// assemblePage packs a page's slice of the vertical list, lifting footnoteNodes
// out of the content flow and stacking their bodies below a separator rule at the
// foot of the page. pageNum is this page's 1-based ordinal; when the page style is
// not "empty" a centred page number is placed at the very foot (see pagenum.go).
func (e *Engine) assemblePage(page []node, pageNum int) *boxNode {
	var content []node
	var notes []*boxNode
	for _, n := range page {
		if fn, ok := n.(footnoteNode); ok {
			notes = append(notes, fn.body)
			continue
		}
		content = append(content, n)
	}
	vlist := append([]node{}, content...)
	if len(notes) > 0 {
		// \skip\footins above the rule, then \footnoterule — which costs NOTHING
		// net: \kern-3pt, a .4pt rule, \kern2.6pt (latex.ltx:13162-13163). The
		// gaps here were a flat 10pt and 4pt, 5.4pt more than the reference puts
		// there.
		vlist = append(vlist,
			glueNode{spec: glueSpec{width: e.footinsSkip()}},
			kernNode{width: -ptToSP(3)},
			ruleNode{width: e.hsize * 2 / 5, height: defaultRule},
			kernNode{width: ptToSP(2.6)},
		)
		for i, b := range notes {
			if i > 0 {
				vlist = append(vlist, glueNode{spec: glueSpec{width: 3 * unity}})
			}
			vlist = append(vlist, b)
		}
	}
	if e.pageStyle == "empty" {
		return vpackSP(vlist, packNatural, 0)
	}
	e.curPageNum = pageNum // so \thepage in a header/footer field is this page

	if e.pageStyle == "fancy" {
		return e.assembleFancyPage(vlist)
	}
	// "plain": a centred page number pushed to the foot with vertical fil, filling
	// the page to \vsize so the number sits at the bottom of the text area.
	vlist = append(vlist,
		glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}, // vfil
		e.pageFooter(pageNum),
	)
	if e.vsize > 0 {
		return vpackSP(vlist, packTo, e.vsize)
	}
	return vpackSP(vlist, packNatural, 0)
}

// assembleFancyPage builds a \pagestyle{fancy} page: the running header (with its
// rule) above the content, and the running footer (with its rule) at the foot, the
// content stretched to \vsize between them. body is the content+footnotes vlist.
func (e *Engine) assembleFancyPage(body []node) *boxNode {
	var top []node
	if h := e.fancyHeader(); h != nil {
		top = append(top, h)
		if e.headRule > 0 {
			top = append(top,
				glueNode{spec: glueSpec{width: 2 * unity}},
				ruleNode{width: e.hsize, height: e.headRule},
			)
		}
		top = append(top, glueNode{spec: glueSpec{width: 6 * unity}}) // gap below header
	}
	page := append(top, body...)
	page = append(page, glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}) // vfil
	if f := e.fancyFooter(); f != nil {
		if e.footRule > 0 {
			page = append(page,
				ruleNode{width: e.hsize, height: e.footRule},
				glueNode{spec: glueSpec{width: 2 * unity}},
			)
		}
		page = append(page, f)
	}
	if e.vsize > 0 {
		return vpackSP(page, packTo, e.vsize)
	}
	return vpackSP(page, packNatural, 0)
}

// NOTE on \footnotesep: LaTeX puts a \rule\z@\footnotesep at the HEAD of every
// note (latex.ltx:13199). It is a strut — it sets a minimum height for the note's
// own box — not glue between notes, so adding it here as a gap is wrong: tried,
// and it pushed the pitch from 10.41 to 11.06 against a reference of 9.63. What is
// left of that 0.78 is the note being set on the BODY leading rather than
// \footnotesize's 9.5pt, which is a different change (the held one).
