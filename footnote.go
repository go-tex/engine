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

// footnoteReserve is the extra vertical space a note reserves beyond its body —
// the separator rule and the gaps around it — so the page leaves room for the
// foot area assemblePage builds.
const footnoteReserve = 16 * unity

// doFootnote implements \footnote{text}: step the counter, typeset the numbered
// note into a vbox held until the enclosing paragraph attaches it to the vertical
// list, and drop a raised reference number at the current point.
func (e *Engine) doFootnote() {
	text := e.grabUndelimited()
	e.footnoteCounter++
	n := e.footnoteCounter

	// Body = "N. " + text, set as a mini-paragraph to the body width.
	label := append(numberToks(n), chTok('.', catOther), chTok(' ', catSpace))
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
	savedBuilding := e.buildingFootnote
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
	e.buildingFootnote = savedBuilding
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
// foot of the page.
func (e *Engine) assemblePage(page []node) *boxNode {
	var content []node
	var notes []*boxNode
	for _, n := range page {
		if fn, ok := n.(footnoteNode); ok {
			notes = append(notes, fn.body)
			continue
		}
		content = append(content, n)
	}
	if len(notes) == 0 {
		return vpackSP(content, packNatural, 0)
	}
	vlist := append([]node{}, content...)
	vlist = append(vlist,
		glueNode{spec: glueSpec{width: 10 * unity}},           // gap above the rule
		ruleNode{width: e.hsize * 2 / 5, height: defaultRule}, // \footnoterule
		glueNode{spec: glueSpec{width: 4 * unity}},            // gap below the rule
	)
	for i, b := range notes {
		if i > 0 {
			vlist = append(vlist, glueNode{spec: glueSpec{width: 3 * unity}})
		}
		vlist = append(vlist, b)
	}
	return vpackSP(vlist, packNatural, 0)
}
