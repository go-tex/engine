// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the numbered equation environment. \begin{equation} steps
// \c@equation and freezes the number into \@currentlabel (so \label captures it and
// \eqref reproduces it), then \@equationbody captures the math up to \end{equation},
// typesets it as display math centred on its line, and sets the number "(N)" flush
// right. \label inside the body is executed (not typeset); \nonumber suppresses the
// number.

import "strings"

// doEquationBody is the \@equationbody primitive: the kernel's \equation macro has
// already stepped the counter and set \@currentlabel, so here we capture the math,
// render it and place it with its right-flushed number.
func (e *Engine) doEquationBody() {
	number := e.toksToString(e.expandList([]tok{csTok("theequation")}))
	src, numbered := e.collectMathUntilEnd("equation")
	m := e.makeMath(src, true)

	e.endParagraph()
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	row := []node{fil, m, fil}
	if numbered {
		row = append(row, e.textToHbox("("+number+")"))
	}
	e.contribute(hpackSP(row, packTo, e.hsize))
}

// collectMathUntilEnd reads raw tokens up to \end{name}, reconstructing the math
// source string for go-tex/math. \label is executed in place (it records the
// current \@currentlabel — the equation number), and \nonumber flips the numbered
// flag off; neither reaches the math source.
func (e *Engine) collectMathUntilEnd(name string) (src string, numbered bool) {
	numbered = true
	var b strings.Builder
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		switch {
		case t.cs_ && t.cs == "end":
			nm := e.readBraceName()
			if nm == name {
				return b.String(), numbered
			}
			b.WriteString("\\end{" + nm + "}") // a non-matching \end (unusual in math)
		case t.cs_ && t.cs == "label":
			e.doLabel() // record the label = the equation number, don't typeset it
		case t.cs_ && t.cs == "nonumber":
			numbered = false
		case t.cs_:
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		default:
			b.WriteRune(t.ch)
		}
	}
	return b.String(), numbered
}

// textToHbox packs a literal string as an hbox in the current font (used for the
// equation number).
func (e *Engine) textToHbox(s string) *boxNode {
	if e.curFont == nil {
		return hpackSP(nil, packNatural, 0)
	}
	var l []node
	for _, r := range s {
		l = e.appendChar(l, r)
	}
	return hpackSP(l, packNatural, 0)
}
