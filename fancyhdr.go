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
	if len(toks) == 0 {
		return hpackSP(nil, packNatural, 0)
	}
	savedMvl, savedPar, savedIn, savedPD := e.mvl, e.parList, e.inPar, e.prevDepth
	savedBuilding, savedFont, savedHsize := e.buildingFootnote, e.curFont, e.hsize
	e.mvl, e.parList, e.inPar, e.prevDepth = nil, nil, false, ignoreDepth
	e.buildingFootnote = true // guard endParagraph from flushing footnotes here
	e.hsize = 1 << 30         // effectively no line breaking: keep it one line

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
	line := hpackSP(e.parList, packNatural, 0)

	e.mvl, e.parList, e.inPar, e.prevDepth = savedMvl, savedPar, savedIn, savedPD
	e.buildingFootnote, e.curFont, e.hsize = savedBuilding, savedFont, savedHsize
	return line
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
	if len(e.fancyHF[fldHL])+len(e.fancyHF[fldHC])+len(e.fancyHF[fldHR]) == 0 {
		return nil
	}
	return e.fancyLine(fldHL, fldHC, fldHR)
}

func (e *Engine) fancyFooter() *boxNode {
	if len(e.fancyHF[fldFL])+len(e.fancyHF[fldFC])+len(e.fancyHF[fldFR]) == 0 {
		return nil
	}
	return e.fancyLine(fldFL, fldFC, fldFR)
}
