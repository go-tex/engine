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

// notePageAssembled publishes the page being assembled in \c@page.
func (e *Engine) notePageAssembled(pageNum int) {
	e.setCount(pageCounterReg, pageNum, true)
}
