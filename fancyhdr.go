// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements a practical subset of the fancyhdr package: running headers
// and footers. \pagestyle{fancy} turns them on (with a rule under the header by
// default); \lhead/\chead/\rhead and \lfoot/\cfoot/\rfoot set the six fields (left,
// centre, right for the header and the footer), \fancyhead[LCR]{}/\fancyfoot[LCR]{}
// set them by position, and \fancyhf{} clears them all. Each field is stored as a
// token list and typeset per page at assembly time, so \thepage in a field reflects
// the real page number. Rich fonts/colours in a field are honoured (the field is run
// through the normal typesetter); \headrulewidth/\footrulewidth customisation is not
// modelled — the header rule is on and the footer rule off, the fancyhdr defaults.

// field indices into e.fancyHF.
const (
	fldHL = iota // header left
	fldHC        // header centre
	fldHR        // header right
	fldFL        // footer left
	fldFC        // footer centre
	fldFR        // footer right
)

// readBraceToks reads a {…} group as a token list. grabGroup assumes the opening
// brace is already consumed (its depth starts at 1), so we skip to and eat the '{'
// first; a missing brace yields an empty list.
func (e *Engine) readBraceToks() []tok {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return nil
	}
	return e.grabGroup()
}

// setFancyField stores the token list of a header/footer field.
func (e *Engine) setFancyField(idx int) {
	e.fancyHF[idx] = e.readBraceToks()
}

// doFancyhf clears all six header/footer fields (\fancyhf{}); an optional argument
// is accepted and ignored.
func (e *Engine) doFancyhf() {
	e.readBraceToks()
	for i := range e.fancyHF {
		e.fancyHF[i] = nil
	}
}

// doFancyPos implements \fancyhead[pos]{content} / \fancyfoot[pos]{content}: base is
// fldHL for the header or fldFL for the footer; the optional [L/C/R] letters select
// which of the three columns to set (default all three).
func (e *Engine) doFancyPos(base int) {
	pos := e.scanFancyPos()
	toks := e.readBraceToks()
	set := func(off int) { e.fancyHF[base+off] = append([]tok{}, toks...) }
	if pos == 0 { // no bracket ⇒ all three columns
		set(0)
		set(1)
		set(2)
		return
	}
	if pos&1 != 0 {
		set(0) // L
	}
	if pos&2 != 0 {
		set(1) // C
	}
	if pos&4 != 0 {
		set(2) // R
	}
}

// scanFancyPos reads an optional [L C R] argument and returns a bitmask (1=L, 2=C,
// 4=R); 0 means no bracket was present.
func (e *Engine) scanFancyPos() int {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || t.cs_ || t.ch != '[' {
		if ok {
			e.back(t)
		}
		return 0
	}
	mask := 0
	for {
		u, ok := e.getNext()
		if !ok || (!u.cs_ && u.ch == ']') {
			break
		}
		switch {
		case !u.cs_ && (u.ch == 'l' || u.ch == 'L'):
			mask |= 1
		case !u.cs_ && (u.ch == 'c' || u.ch == 'C'):
			mask |= 2
		case !u.cs_ && (u.ch == 'r' || u.ch == 'R'):
			mask |= 4
		}
	}
	if mask == 0 {
		mask = 7 // an empty [] targets all three, as fancyhdr does
	}
	return mask
}

// typesetFieldToHbox runs a field's token list through the typesetter in isolation
// and returns the resulting line as a natural-width hbox, so a header/footer field
// keeps its fonts, colours and \thepage. An empty field yields an empty hbox.
func (e *Engine) typesetFieldToHbox(toks []tok) *boxNode {
	return hpackSP(e.typesetFieldToList(toks), packNatural, 0)
}

// typesetFieldToList is typesetFieldToHbox without the packing: it returns the
// horizontal list itself.
//
// ⛔ THE DIFFERENCE IS WHETHER AN \hfil IN THE FIELD CAN STRETCH. Packing here
// and handing back a box means any glue inside it has already been set at the
// box's natural width, and a later hpack to \hsize then slides that rigid box to
// the left. \@oddhead is `{\slshape\rightmark}\hfil\thepage` — one field with the
// \hfil INSIDE it — so a caller that needs LaTeX's \hb@xt@\textwidth{\@thehead}
// (ltoutput.dtx:3363) has to pack the LIST. fancyhdr's three fields are a
// different shape: each is packed naturally and the \hfil's sit BETWEEN them
// (fancyLine), which is why that path was right and this one was not.
func (e *Engine) typesetFieldToList(toks []tok) []node {
	if len(toks) == 0 {
		return nil
	}
	savedMvl, savedPar, savedIn, savedPD := e.mvl, e.parList, e.inPar, e.prevDepth
	savedBuilding, savedFont, savedHsize := e.buildingFootnote, e.curFont, e.hsize
	e.mvl, e.parList, e.inPar, e.prevDepth = nil, nil, false, ignoreDepth
	e.buildingFootnote = true // guard endParagraph from flushing footnotes here
	e.hsize = 1 << 30         // effectively no line breaking: keep it one line
	savedRestricted := e.restrictedH
	e.restrictedH = false // a list of its own, even when called from inside a box
	defer func() { e.restrictedH = savedRestricted }()

	e.push(append([]tok{csTok("noindent")}, append(append([]tok{}, toks...), sentinel)...))
	for e.err == nil {
		t, ok := e.getXToken()
		if !ok || (t.cs_ && t.cs == sentinel.cs) {
			break
		}
		if !e.stepToken(t) {
			break
		}
	}
	list := e.parList

	e.mvl, e.parList, e.inPar, e.prevDepth = savedMvl, savedPar, savedIn, savedPD
	e.buildingFootnote, e.curFont, e.hsize = savedBuilding, savedFont, savedHsize
	return list
}

// fancyLine packs the three fields (left/centre/right) of a header or footer into an
// hbox to \hsize: left flush left, centre centred, right flush right.
func (e *Engine) fancyLine(l, c, r int) *boxNode {
	lb := e.typesetFieldToHbox(e.fancyHF[l])
	cb := e.typesetFieldToHbox(e.fancyHF[c])
	rb := e.typesetFieldToHbox(e.fancyHF[r])
	return hpackSP([]node{lb, e.hfil(), cb, e.hfil(), rb}, packTo, e.hsize)
}

// fancyHeader / fancyFooter build the running header / footer for a page, or nil
// when all three of the respective fields are empty.
func (e *Engine) fancyHeader() *boxNode {
	var line *boxNode
	if len(e.fancyHF[fldHL])+len(e.fancyHF[fldHC])+len(e.fancyHF[fldHR]) == 0 {
		line = e.latexHead() // no fancyhdr field: LaTeX's own \@oddhead, if any
	} else {
		line = e.fancyLine(fldHL, fldHC, fldHR)
	}
	if line == nil {
		return nil
	}
	return e.headToHeadheight(line)
}

// headToHeadheight is the \vbox the running head actually travels in.
// ltoutput.dtx:3350-3369, inside \@outputpage:
//
//	\setbox\@tempboxa \vbox to\headheight {%
//	  \vfil
//	  … \hb@xt@ \textwidth {\@thehead }%
//	}
//	\dp \@tempboxa \z@
//	\box \@tempboxa
//	\vskip \headsep
//
// ⛔⛔ THE \vfil IS WHAT FIXES THE BASELINE. A \vbox to h measures its height to
// the baseline of its LAST box, so with the glue above it the head's baseline
// sits exactly \headheight below the band's top WHATEVER the head contains. This
// engine emitted the line at its NATURAL height, so the baseline sat at the
// height of the tallest glyph in it instead — and therefore MOVED with the text.
//
// Measured against tectonic, same band top (87.90pt) in both engines, article
// 10pt \pagestyle{headings}, head baseline from the top of the page:
//
//	head text     reference      ours        deficit   glyph height
//	1 ALFA 1      99.90          95.05       4.85      cap   ≈ .717em
//	1 (QJPY) 1    99.90          95.39       4.51      paren ≈ .75em
//	1             99.90          94.55       5.35      digit ≈ .667em
//
// The reference is 99.90 in all three — identical to the hundredth of a point —
// and every one of our deficits is \headheight minus the height of the head's own
// glyphs. That is the whole defect, and it is why the witness varies the head's
// CONTENT rather than asserting one number. go-tex/engine#822.
//
// ⛔ THE DEPTH IS ZEROED, as \dp\@tempboxa\z@ does, because \headsep is measured
// from the head's BASELINE and not from under its descenders. Without it a head
// containing a `y` would push the whole text block down by that descender.
func (e *Engine) headToHeadheight(line *boxNode) *boxNode {
	b := vpackSP(
		[]node{glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}, line},
		packTo, e.bandDimen("headheight", 12*unity),
	)
	b.depth = 0
	return b
}

// latexHead is the running head LaTeX's OWN page styles declare, as opposed to
// fancyhdr's six fields. \ps@headings and every class that writes its own style
// define \@oddhead — article.cls:
//
//	\def\ps@headings{%
//	  \let\@oddfoot\@empty
//	  \def\@oddhead{{\slshape\rightmark}\hfil\thepage}%
//	  \let\@mkboth\markboth
//	  …
//
// doPagestyle already RUNS \ps@name, so those \def's take effect; nothing read
// them back, and 22 of the 128 corpus papers with a reference draw a head this way
// and drew none here (#426). The line is one hbox to \hsize and the \hfil's inside
// it place the pieces, which is exactly how LaTeX sets \@oddhead.
//
// ⛔ THE PAGE'S PARITY PICKS THE HEAD. latex.ltx:15387-15393 (\@outputpage):
//
//	\if@twoside \ifodd\count\z@ \let\@thehead\@oddhead … \else \let\@thehead\@evenhead …
//
// so under \if@twoside an even page sets \@evenhead. amsart is twoside by default
// (amsart.cls:350-351) and its two heads differ in ORDER — even is
// `\rlap{\thepage}\hfil\leftmark\hfil`, odd `\hfil\rightmark\hfil\llap{\thepage}`
// (:306-319) — so every even page of every amsart paper drew the odd head:
// "TITLE 2" at x0 292 where tectonic sets "2 TITLE" at x0 126 (2406.10437 p2,
// go-tex/engine#875). A oneside document never asks for the even one.
func (e *Engine) latexHead() *boxNode {
	m := e.pageSidedMacro("head", e.pageOrdinal())
	if m == nil || m.kind != mMacro || len(m.body) == 0 {
		return nil
	}
	// ⛔ THE LIST, NOT A BOX. LaTeX sets the head as \hb@xt@\textwidth{\@thehead}
	// (ltoutput.dtx:3363), so the \hfil that \@oddhead itself contains —
	// `{\slshape\rightmark}\hfil\thepage` — is what pushes the folio to the right
	// margin. Packing the field into its own natural-width box first set that
	// \hfil to zero and left the whole head huddled at the left: the running head
	// read "1 ALFA1", folio abutting the title, ending at x=180.35pt on a page
	// where the reference runs to 477.48pt.
	return hpackSP(e.typesetFieldToList(m.body), packTo, e.hsize)
}

// hasLatexHead reports whether \@oddhead would draw something, WITHOUT typesetting
// it: headBand is read before pagination (pdfdriver.go), so it must not run the
// typesetter.
func (e *Engine) hasLatexHead() bool {
	for _, name := range []string{"@oddhead", "@evenhead"} {
		m := e.eq[name]
		if m != nil && m.kind == mMacro && len(m.body) > 0 && (name == "@oddhead" || e.twoside()) {
			return true
		}
	}
	return false
}

// twoside is \if@twoside: a \newif's meaning IS its value (kernelhelpers.go).
func (e *Engine) twoside() bool {
	m := e.eq["if@twoside"]
	return m != nil && m.kind == mPrim && m.name == "iftrue"
}

// pageSidedMacro is \@oddhead / \@evenhead (kind "head") or \@oddfoot / \@evenfoot
// (kind "foot") for page number num, as \@outputpage chooses them: the even one on
// an even page of a twoside document, the odd one otherwise. A twoside document
// whose page style never declared the even one — the engine's own stand-in styles
// define only the odd — keeps the odd rather than drawing nothing.
func (e *Engine) pageSidedMacro(kind string, num int) *meaning {
	odd := e.eq["@odd"+kind]
	if e.twoside() && num%2 == 0 {
		if ev := e.eq["@even"+kind]; ev != nil && ev.kind == mMacro {
			return ev
		}
	}
	return odd
}

// pageMarginShift is how far an EVEN page's text block sits from where an odd
// page's does, in sp: \evensidemargin - \oddsidemargin under \if@twoside
// (latex.ltx:15387-15393 puts the block at 1in + \hoffset + the page's own margin).
// bk10.clo:147-182 makes them differ by 53.79pt on a letter-paper book. Only the
// class's own margins are modelled: a document that loaded geometry states its
// left margin itself (geometry.go).
func (e *Engine) pageMarginShift(num int) int {
	if e.geom != nil || !e.twoside() || num%2 != 0 {
		return 0
	}
	if _, _, ok := e.paperSizePt(); !ok {
		return 0
	}
	osm, ok1 := e.namedDimen("oddsidemargin")
	esm, ok2 := e.namedDimen("evensidemargin")
	if !ok1 || !ok2 {
		return 0
	}
	return esm - osm
}

func (e *Engine) fancyFooter() *boxNode {
	if len(e.fancyHF[fldFL])+len(e.fancyHF[fldFC])+len(e.fancyHF[fldFR]) == 0 {
		return nil
	}
	return e.fancyLine(fldFL, fldFC, fldFR)
}
