// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the numbered equation environment. \begin{equation} steps
// \c@equation and freezes the number into \@currentlabel (so \label captures it and
// \eqref reproduces it), then \@equationbody captures the math up to \end{equation},
// typesets it as display math centred on its line, and sets the number "(N)" flush
// right. \label inside the body records the equation's number (or its \tag);
// \nonumber / \notag suppress the number; \tag{x} replaces it with a custom "(x)"
// and \tag*{x} with a bare "x" — neither consumes an automatic number.

import "strings"

// eqMeta is what a display-math body collector found besides the source: whether it
// is numbered, an optional \tag (custom number) and its starred (no-parens) flag,
// and any \label keys attached to it.
type eqMeta struct {
	numbered bool
	tag      string
	tagStar  bool
	labels   []string
}

// doEquationBody is the \@equationbody primitive: the kernel's \equation macro has
// already stepped the counter and set \@currentlabel, so here we capture the math,
// render it and place it with its number (auto, tagged, or suppressed).
func (e *Engine) doEquationBody() {
	number := e.toksToString(e.expandList([]tok{csTok("theequation")}))
	src, meta := e.collectMathUntilEnd("equation")
	m := e.makeMath(src, true)

	// A \tag or a suppressed number does not consume an automatic number, so undo
	// the advance the kernel's \equation macro performed.
	if meta.tag != "" || !meta.numbered {
		e.unstepEquationCounter()
	}
	labelVal := number
	if meta.tag != "" {
		labelVal = meta.tag
	}
	for _, k := range meta.labels {
		e.setLabel(k, labelVal)
		e.recordRefMeta(k) // keep the typed-reference metadata (\autoref/\cref)
	}

	e.endParagraph()
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	row := []node{fil, m, fil}
	if box := e.eqNumberBox(meta, number); box != nil {
		row = append(row, box)
	}
	e.contribute(hpackSP(row, packTo, e.hsize))
}

// eqNumberBox builds the number box for a display line: the \tag (parenthesised
// unless starred), else the automatic "(N)" when numbered, else nothing.
func (e *Engine) eqNumberBox(meta eqMeta, autoNum string) *boxNode {
	switch {
	case meta.tag != "" && meta.tagStar:
		return e.textToHbox(meta.tag)
	case meta.tag != "":
		return e.textToHbox("(" + meta.tag + ")")
	case meta.numbered:
		return e.textToHbox("(" + autoNum + ")")
	}
	return nil
}

// collectMathUntilEnd reads raw tokens up to \end{name}, reconstructing the math
// source string for go-tex/math. \nonumber/\notag, \tag/\tag* and \label are pulled
// out of the cell (executed as metadata, not sent to the math renderer). \label keys
// are deferred so they can be recorded against the final number (which \tag may set).
func (e *Engine) collectMathUntilEnd(name string) (src string, meta eqMeta) {
	meta.numbered = true
	var b strings.Builder
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		switch {
		case t.cs_ && t.cs == "end":
			if e.readBraceName() == name {
				return b.String(), meta
			}
			b.WriteString("\\end{" + name + "}") // a non-matching \end (unusual in math)
		case t.cs_ && t.cs == "label":
			meta.labels = append(meta.labels, e.readBraceName())
		case t.cs_ && (t.cs == "nonumber" || t.cs == "notag"):
			meta.numbered = false
		case t.cs_ && t.cs == "tag":
			meta.tag, meta.tagStar = e.readTag()
		case t.cs_:
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		default:
			b.WriteRune(t.ch)
		}
	}
	return b.String(), meta
}

// readTag reads a \tag argument: an optional leading * (bare, no parentheses) then a
// braced {text}. Returns the tag text and whether it was starred.
func (e *Engine) readTag() (text string, star bool) {
	if t, ok := e.getNext(); ok {
		if !t.cs_ && t.ch == '*' {
			star = true
		} else {
			e.back(t)
		}
	}
	return e.readBraceName(), star
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
