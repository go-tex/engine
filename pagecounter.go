// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \c@page is \count0, the register TeX's output routine reads as the page number
// (latex.ltx: \countdef\c@page=0 under \@definecounter{page}; plain.tex: \pageno is
// \count0). It is a real count register, so \the\c@page, \ifnum\c@page>3,
// \ifodd\c@page, \advance\c@page and \stepcounter{page} all work; before this it was
// an undefined control sequence and \the\c@page printed "@page" (3 corpus papers;
// 2401.13343 alone has 69 uses).
//
// ⚠ This engine breaks pages AFTER the whole text is read, so in running text the
// register still holds its initial value 1, as \thepage does there (see its comment
// in primitives.go). During assembly of a page, in a header or footer field, it holds
// that page's ordinal (assemblePageBox), which is what \ifodd\c@page needs.

// pageCounterReg is the register \c@page names.
const pageCounterReg = 0

// installPageCounter defines \c@page as register 0, empties \cl@page (what
// \stepcounter runs) and starts the page at 1.
func (e *Engine) installPageCounter() {
	e.define("c@page", &meaning{kind: mCountRef, code: pageCounterReg}, true)
	if e.eq["cl@page"] == nil {
		e.define("cl@page", &meaning{kind: mMacro}, true)
	}
	e.setCount(pageCounterReg, 1, true)
}

// pageCounterSetNode carries a write to \c@page through page breaking, as
// pageStyleNode carries \thispagestyle. The page builder runs after the whole text is
// read, so at the moment \setcounter{page}{5} or \pagenumbering{roman} is called
// nothing knows which page it falls on; a node on the vertical list lands on that page
// by construction, which is how TeX gets the same answer: \c@page is read when the
// page SHIPS, and the write happened while the contribution list was being built.
//
//	\def\pagenumbering#1{\global\c@page \@ne
//	  \gdef\thepage{\csname @#1\endcsname \c@page}}        latex.ltx:10818-10820
//
// so \pagenumbering is a reset to 1 AND a new \thepage; \setcounter{page}{n} is only
// the reset to n. style 0 keeps the current format.
type pageCounterSetNode struct {
	value int
	style byte
}

func (pageCounterSetNode) isNode() {}

// contributePageCounterSet puts a \c@page write on the main vertical list. Inside a
// paragraph it must not END the paragraph (\setcounter is no vertical command), so it
// waits for the paragraph's end instead.
func (e *Engine) contributePageCounterSet(s pageCounterSetNode) {
	if e.inPar {
		e.parSets = append(e.parSets, s)
		return
	}
	e.contribute(s)
}

// mvlHasNodesBesidesPageCounter reports whether the main vertical list holds anything
// but \c@page writes. article.cls ends with \pagenumbering{arabic}\onecolumn, and a
// write is no reason for \onecolumn to open a region of its own: that region would
// hold the write alone and ship a blank page.
func (e *Engine) mvlHasNodesBesidesPageCounter() bool {
	for _, n := range e.mvl {
		if _, ok := n.(pageCounterSetNode); !ok {
			return true
		}
	}
	return false
}

// takePageCounterSets lifts the \c@page writes out of a vertical list, in order, and
// returns the list without them (a marker left in a page is one more thing to explain).
func takePageCounterSets(vlist []node) (sets []pageCounterSetNode, rest []node) {
	rest = vlist
	for i, n := range vlist {
		if s, ok := n.(pageCounterSetNode); ok {
			if sets == nil {
				rest = append([]node(nil), vlist[:i]...)
			}
			sets = append(sets, s)
		} else if sets != nil {
			rest = append(rest, n)
		}
	}
	return sets, rest
}

// folioFor applies the writes found on a page to the running folio and returns the
// number printed on the page whose ordinal (its place among the shipped pages) is
// ordinal. Later pages count up from the last write because the offset is kept.
func (e *Engine) folioFor(ordinal int, sets []pageCounterSetNode) int {
	for _, s := range sets {
		e.folioDelta = s.value - ordinal
		if s.style != 0 {
			e.pageNumStyle = s.style
		}
	}
	return ordinal + e.folioDelta
}

// notePageAssembled publishes the page being assembled in \c@page.
func (e *Engine) notePageAssembled(pageNum int) {
	e.setCount(pageCounterReg, pageNum, true)
}
