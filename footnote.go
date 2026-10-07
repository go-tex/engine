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

import (
	"strconv"
	"strings"
)

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
	e.queueFootnoteText(n, text)
	e.emitFootnoteMark(n)
}

// queueFootnoteText builds the numbered note and holds it until the enclosing paragraph
// attaches it to the vertical list. Split out of doFootnote so \footnotetext can reach it:
// the two halves of a footnote are separable in LaTeX and were not here.
func (e *Engine) queueFootnoteText(n int, text []tok) {
	// Body = \footnotesize "N. " + text, set as a mini-paragraph to the body width.
	//
	// ⛔ IN BRACES, because typesetGroupToVbox saves the vertical-list state and the
	// current font and NOT the leading — a documented property other callers rely
	// on (subfigure.go) — while \footnotesize moves \baselineskip. So every
	// footnote left the BODY set at the note's leading, for the whole rest of the
	// document: measured in book at 12pt, \the\baselineskip read 14.5pt before the
	// note and 12.0pt after it, and with \baselinestretch{1.25} 18.125 -> 15.0.
	//
	// The braces are the engine's own grouping, which restores it: a plain
	// {\footnotesize …} written in a document always came back correctly, and that
	// is what said the SANDBOX was not the place to fix this. Scoping it here keeps
	// the other six callers — floats, minipage, multicols, parbox, subfigure,
	// two-column spans — exactly as they were; putting a group inside the sandbox
	// broke multicols, whose body crosses it with groups open.
	label := []tok{chTok('{', catBegin), csTok("footnotesize")}
	label = append(label, numberToks(n)...)
	label = append(label, chTok('.', catOther), chTok(' ', catSpace))
	body := append(label, text...)
	body = append(body, chTok('}', catEnd))
	e.pendingFootnotes = append(e.pendingFootnotes, e.typesetGroupToVbox(body))
}

// emitFootnoteMark drops the raised reference number at the current point.
func (e *Engine) emitFootnoteMark(n int) {
	if e.curFont == nil {
		return
	}
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = append(e.parList, e.footnoteMarker(n))
}

// doFootnoteMark implements \footnotemark and doFootnoteText implements \footnotetext: the
// two halves of a footnote, placed separately. latex.ltx:13202-13206 and :13219-13222 —
//
//	\def\footnotemark{\@ifnextchar[\@xfootnotemark
//	  {\stepcounter{footnote}\protected@xdef\@thefnmark{\thefootnote}\@footnotemark}}
//	\def\footnotetext{\@ifnextchar[\@xfootnotenext
//	  {\protected@xdef\@thefnmark{\thempfn}\@footnotetext}}
//
// ⛔ \footnotemark was undefined — 10 uses over 7 corpus papers, and a hard stop for three of
// them — while \footnotetext was DEFINED as a stub that swallowed the note whole
// (\def\footnotetext{\@ifnextbracket\@gobbleoptarg\@gobble}, latex.go). So the pair lost the
// text and reported nothing: the census counts what the engine does not have, and a stub is
// something it has. Ten corpus papers use one or both. gobblers.py in go-tex/measure is the
// inventory that question produced.
//
// ⛔ Note which one steps the counter. \footnotemark does; \footnotetext does NOT, because it
// pairs with a mark that already stepped. Getting that backwards numbers every later note
// one too high. The bracket forms set the number explicitly and, in LaTeX, inside a group —
// so they do not disturb the counter either.
func (e *Engine) doFootnoteMark() {
	n, explicit := e.scanOptFootnoteNumber()
	if !explicit {
		e.footnoteCounter++
		n = e.footnoteCounter
	}
	e.emitFootnoteMark(n)
}

func (e *Engine) doFootnoteText() {
	n, explicit := e.scanOptFootnoteNumber()
	if !explicit {
		n = e.footnoteCounter
	}
	e.queueFootnoteText(n, e.grabUndelimited())
}

// scanOptFootnoteNumber reads the optional [n] both commands take. A bracket whose content
// is not a number is consumed and ignored rather than guessed at.
func (e *Engine) scanOptFootnoteNumber() (int, bool) {
	toks, ok := e.scanOptBracketToks()
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(e.toksToString(toks)))
	if err != nil {
		return 0, false
	}
	return n, true
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
	// \thispagestyle's override rides the vertical list and applies to the page it
	// landed on, then goes with it (see pageStyleNode).
	style := e.pageStyle
	if s, rest, found := takePageStyle(vlist); found {
		style, vlist = s, rest
	}
	if style == "empty" {
		return vpackSP(vlist, packNatural, 0)
	}
	e.curPageNum = pageNum // so \thepage in a header/footer field is this page

	// "fancy" always assembles head and foot; a LaTeX page style that declared
	// \@oddhead takes the same path, since the only difference is where the fields
	// came from (see latexHead). \pagestyle{headings} lands here.
	//
	// ⛔ The \@oddhead test has to be read TOGETHER with the style in force, not
	// instead of it. book.cls and report.cls say \pagestyle{headings} in their
	// preamble, so \@oddhead is defined for the whole document — and a page asking
	// for "plain" came down this path anyway, which put its folio in the running
	// head at the TOP where the class wants it centred at the FOOT. That is not a
	// rare page: every chapter opening in every book and report is a
	// \thispagestyle{plain} page, and so is the first page of its contents list.
	// Measured against tectonic on an 11pt book: the folio centred at y=694.40
	// there, flush left at y=102.89 here.
	if style == "fancy" || (style != "plain" && e.hasLatexHead()) {
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
		// \headsep separates the head from the text block (25pt in the standard
		// classes), not a fixed 6pt. classes.dtx, "Page Layout": the text starts at
		// 1in + \voffset + \topmargin + \headheight + \headsep, so the band above it
		// is exactly those two lengths — which is also why renderVMargin lifts the
		// page by them when a head is drawn (geometry.go). Measured against the
		// reference on 2405.18549 page 3, our head sat 72pt from the top with 10pt
		// under it where the reference has 37pt and 31pt.
		top = append(top, glueNode{spec: glueSpec{width: e.classDimen("headsep", 25*unity)}})
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
		// \vsize is the TEXT block. The head band sits above it — the page is lifted
		// by exactly that much (renderVMargin/headBand) — so the assembled box is
		// taller by the band and the body still gets its full \textheight. Packing
		// the whole thing to \vsize instead took the band out of the text, three
		// lines a page on a document with a running head.
		return vpackSP(page, packTo, e.vsize+e.headBand())
	}
	return vpackSP(page, packNatural, 0)
}

// NOTE on \footnotesep: LaTeX puts a \rule\z@\footnotesep at the HEAD of every
// note (latex.ltx:13199). It is a strut — it sets a minimum height for the note's
// own box — not glue between notes, so adding it here as a gap is wrong: tried,
// and it pushed the pitch from 10.41 to 11.06 against a reference of 9.63. What
// was left of that 0.78 was the note being set on the BODY leading rather than
// \footnotesize's 9.5pt.
//
// That held change has landed: \@setfontsize now applies the leading for every
// size, not only \normalsize. Measured on a note long enough to wrap, the pitch
// between two of its lines goes 12.0 -> 9.5 against a reference of 9.5, so the
// residual this note described is gone and \footnotesep stays out of here.

// takePageStyle lifts a \thispagestyle override out of a page's vertical list and
// returns the list without it. The node carries no dimension, but it is removed
// rather than left for vpack to walk past: a page's list is measured and shipped,
// and a marker that survives into either is one more thing to explain.
func takePageStyle(vlist []node) (style string, rest []node, found bool) {
	for i, n := range vlist {
		if ps, ok := n.(pageStyleNode); ok {
			out := make([]node, 0, len(vlist)-1)
			out = append(out, vlist[:i]...)
			out = append(out, vlist[i+1:]...)
			return ps.style, out, true
		}
	}
	return "", vlist, false
}
