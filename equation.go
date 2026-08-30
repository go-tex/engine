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
	e.placeDisplay([]*boxNode{hpackSP(row, packTo, e.hsize)})
	// collectMathUntilEnd read this environment's own \end, so \end — and the
	// \gotex@endenv that closes the group \begin opened — never runs. Close it here,
	// AFTER the labels are recorded: \end closes the group only once \endequation
	// has finished, and the reference metadata is part of that.
	e.endEnvGroup()
}

// doEquationStar handles \begin{equation*} (an unnumbered single-line display): the
// same centred display as \begin{equation} but with no automatic number and no
// counter step — the amsmath equivalent of \[ … \]. A \tag still prints and a
// \label captures it. Without this, \begin{equation*} is an undefined environment:
// it is skipped and every \frac / \sum / \left inside is dropped as an unknown
// text-mode command.
func (e *Engine) doEquationStar(name string) {
	src, meta := e.collectMathUntilEnd(name)
	meta.numbered = false // a starred environment never prints an automatic number
	m := e.makeMath(src, true)
	if meta.tag != "" {
		for _, k := range meta.labels {
			e.setLabel(k, meta.tag)
			e.recordRefMeta(k)
		}
	}
	e.endParagraph()
	fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
	row := []node{fil, m, fil}
	if box := e.eqNumberBox(meta, ""); box != nil { // only a \tag prints for a starred env
		row = append(row, box)
	}
	e.placeDisplay([]*boxNode{hpackSP(row, packTo, e.hsize)})
	e.endEnvGroup() // see doEquationBody
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
		case e.runsCsname(t):
			e.runCsname(t) // see runsCsname: \csname may build this environment's \end
		case t.cs_ && t.cs != "end" && e.expandsToEnd(t):
			// A user macro standing in for \end{...} (e.g. \newcommand\enq{\end{equation}}):
			// its tokens are read raw here, so without expanding it the loop would never
			// see \end and would swallow the rest of the document. Expand it in place and
			// re-read; the real \end token surfaces on the next iteration.
			e.expandMacro(e.meaningOf(t))
		case t.cs_ && t.cs == "end":
			endName := e.readBraceName()
			if endName == name {
				return b.String(), meta
			}
			// A nested \end (e.g. \end{aligned}, \end{bmatrix}, \end{cases} inside the
			// equation) must be written back with ITS OWN name — using the outer name
			// here turned \begin{aligned}…\end{aligned} into …\end{equation}, so the math
			// layer reported "aligned closed by equation" and dropped the equation.
			b.WriteString("\\end{" + endName + "}")
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

// expandsToEnd reports whether the control sequence t is a parameterless macro
// whose replacement text begins with \end — i.e. a user shorthand for the
// environment's closing tag (\newcommand\enq{\end{equation}}). Such a macro is
// otherwise read raw by the math-body scanner and would hide the \end. It is a
// thin alias for expandsToCloseCS with the closing cs "end".
func (e *Engine) expandsToEnd(t tok) bool { return e.expandsToCloseCS(t, "end") }

// endMacroExpandsToEnd reports whether \end<name> is a parameterless macro whose
// replacement text begins with \end — i.e. \end{name} is a wrapper that produces
// another environment's terminator by expansion (ifacconf's \endabstract →
// \end{minipage}…). A body scanner hunting for \end{inner} must run such a wrapper
// in place, or the real terminator never surfaces and it swallows to EOF.
func (e *Engine) endMacroExpandsToEnd(name string) bool {
	m := e.eq["end"+name]
	return m != nil && m.kind == mMacro && len(m.params) == 0 &&
		len(m.body) > 0 && m.body[0].cs_ && m.body[0].cs == "end"
}

// endMacroLeadsToEnd reports whether \end<name> reaches an \end through ONE more
// parameterless macro. beamer's columns are built that way:
//
//	\newcommand<>\beamer@columncom[2][\beamer@colmode]{%
//	  \beamer@colclose
//	  \def\beamer@colclose{\end{minipage}\hfill\end{actionenv}\ignorespaces}%
//	  \begin{actionenv}#3\begin{minipage}…}
//
// so \column opens a minipage and STORES its \end{minipage} in \beamer@colclose, run at
// the next \column or at \end{columns}: \endcolumns begins with \beamer@colclose, not
// with \end.
//
// It is a LAST RESORT, never a first rule. Applying it from the start runs an
// environment's terminator too early — measured, that cost 28 pages over 200 talks and
// took one from 17 pages to 1. collectEnvBody consults it only where the scan is about
// to run off the end of the file it began in.
func (e *Engine) endMacroLeadsToEnd(name string) bool {
	m := e.eq["end"+name]
	if m == nil || m.kind != mMacro || len(m.params) != 0 || len(m.body) == 0 {
		return false
	}
	if b := m.body[0]; b.cs_ {
		return b.cs == "end" || e.expandsToCloseCS(b, "end")
	}
	return false
}

// expandsToCloseCS reports whether the control sequence t is a parameterless
// macro whose replacement text begins with the control sequence named close —
// i.e. a user shorthand that stands in for a raw-token scanner's terminator
// (\newcommand\enq{\end{equation}} for a \end-hunting scanner, or
// \newcommand\dclose{\]} for a \[…\] scanner that closes on \]). Such a macro is
// read raw by an environment/argument body scanner, so without expanding it the
// terminator token never surfaces and the scanner swallows the rest of the input.
// The check is deliberately narrow — no parameters and body[0] == the exact close
// cs — so ordinary content macros (\R, \mathbb, or a user \def that merely mentions
// the close cs mid-body) are never expanded here, keeping the scanned source and
// verbatim cells intact.
func (e *Engine) expandsToCloseCS(t tok, close string) bool {
	m := e.meaningOf(t)
	if m == nil || m.kind != mMacro || len(m.params) != 0 || len(m.body) == 0 {
		return false
	}
	return m.body[0].cs_ && m.body[0].cs == close
}

// runsCsname reports whether t is the \csname primitive, which a body scanner that
// reads RAW must run rather than store.
//
// \csname can only produce a control sequence, and that sequence may be the \end the
// scanner is hunting. beamer reaches every one of its templates that way —
// \usebeamertemplate{X} is \csname beamer@@tmpl@X\endcsname — and the rounded block's
// closing template is \end{beamerboxesrounded}, whose own first move is
// \end{minipage}. Stored raw, that \end never surfaced: the minipage scanner ran to
// the end of the document and took the talk with it. Measured, \usetheme{Warsaw},
// Madrid, Copenhagen and Frankfurt — every theme whose inner theme is `rounded` —
// rendered ZERO pages.
func (e *Engine) runsCsname(t tok) bool {
	if !t.cs_ {
		return false
	}
	m := e.meaningOf(t)
	return m != nil && m.kind == mPrim && m.name == "csname"
}

// runCsname executes \csname…\endcsname so the control sequence it builds is the next
// token the scanner sees.
func (e *Engine) runCsname(t tok) {
	if m := e.meaningOf(t); m != nil && m.prim != nil {
		m.prim(e)
	}
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
