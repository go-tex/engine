// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's verbatim material: the verbatim environment and
// the \verb command. Both set their content literally — no macro expansion, no
// category-code interpretation, spaces and line breaks preserved — in the tt
// font when one is bound (otherwise the current font). Because the content is
// read raw from the base input rather than tokenized, the usual catcodes of \, {,
// $, %, … do not apply, exactly as \verbatim demands.

import "strings"

// doVerbatim typesets a verbatim environment. It is reached via \begin{verbatim},
// so the input cursor sits just past "{verbatim}"; it copies the raw source up to
// the literal "\end{verbatim}" and sets each line on its own, left-aligned.
func (e *Engine) doVerbatim() {
	const end = `\end{verbatim}`
	startOff := e.bpos
	rest := string(e.base[e.bpos:])
	idx := strings.Index(rest, end)
	var content string
	if idx < 0 {
		content = rest
		e.bpos = len(e.base)
	} else {
		content = rest[:idx]
		e.bpos += len([]rune(rest[:idx])) + len([]rune(end))
	}
	e.endEnvGroup() // this environment reads its own \end, so it closes \begin's group
	// Verbatim ignores the newline right after \begin{verbatim} and the one just
	// before \end{verbatim}.
	leadingNL := strings.HasPrefix(content, "\n")
	content = strings.TrimPrefix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	// The source line the first content character lives on (for stamping).
	line, _ := e.lineColAt(startOff)
	if leadingNL {
		line++
	}

	e.endParagraph() // flush any open paragraph before the verbatim block
	font := e.verbFont()
	if font == nil {
		return
	}
	e.mvlAppendGap() // a little space above the block
	for _, ln := range strings.Split(content, "\n") {
		e.appendToPage(e.verbatimLine(ln, font, line))
		line++
	}
	e.mvlAppendGap()
}

// doVerb implements \verb<c>…<c>: the delimiter is the first character after the
// command, and the literal text runs to its next occurrence. The text joins the
// current paragraph inline.
func (e *Engine) doVerb() {
	if e.bpos >= len(e.base) {
		return
	}
	delim := e.base[e.bpos]
	e.bpos++
	start := e.bpos
	for e.bpos < len(e.base) && e.base[e.bpos] != delim {
		e.bpos++
	}
	text := string(e.base[start:e.bpos])
	if e.bpos < len(e.base) {
		e.bpos++ // consume the closing delimiter
	}
	font := e.verbFont()
	if font == nil {
		return
	}
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = append(e.parList, e.verbNodes(text, font, e.curSrcLine)...)
}

// doVerbFancy implements fancyvrb's \Verb[options]{text} and \Verb[options]|text|.
// The braced form is the one modern fancyvrb documents, and it is the one this
// engine must read from TOKENS rather than from the character buffer: a paper
// writes \newcommand{\jl}[1]{\small\Verb{#1}} and uses it a hundred times, so by
// the time \Verb runs its argument is a macro parameter, long gone from the file.
// Undefined, those hundred fragments of code were skipped with their text.
//
// The delimited form still reads the buffer, exactly as \verb does.
func (e *Engine) doVerbFancy() {
	// While the argument is still in the FILE, everything is peeked raw: reading a
	// token to look ahead would move the buffer cursor past the delimiter and push
	// the token onto the list, and \Verb|code| would then take the character after
	// the bar as its delimiter (measured: "|autre_code()|" came out "utre_code()||").
	if len(e.lists) == 0 {
		e.skipRawSpace()
		if e.bpos < len(e.base) && e.base[e.bpos] == '[' {
			for e.bpos < len(e.base) && e.base[e.bpos] != ']' {
				e.bpos++
			}
			if e.bpos < len(e.base) {
				e.bpos++
			}
			e.skipRawSpace()
		}
		if e.bpos < len(e.base) && e.base[e.bpos] != '{' {
			e.doVerb() // \Verb|text|: the classic delimited form
			return
		}
	} else {
		e.scanOptBracketToks() // [commandchars=…, fontsize=…]: no verbatim styling here
	}
	text := e.toksToString(e.readBraceToksRaw())
	font := e.verbFont()
	if font == nil {
		return
	}
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = append(e.parList, e.verbNodes(text, font, e.curSrcLine)...)
}

// skipRawSpace advances past spaces in the character buffer, without tokenising:
// a look-ahead that goes through the mouth cannot be undone for \verb-like
// scanning, which reads characters rather than tokens.
func (e *Engine) skipRawSpace() {
	for e.bpos < len(e.base) && (e.base[e.bpos] == ' ' || e.base[e.bpos] == '\t') {
		e.bpos++
	}
}

// verbFont returns the tt font when one is bound, else the current font.
func (e *Engine) verbFont() fontFace {
	if m := e.eq["tt"]; m != nil && m.kind == mFont && m.font != nil {
		return m.font
	}
	return e.curFont
}

// verbatimLine packs one literal line into a left-aligned hbox in the verbatim
// font, preserving spaces as fixed-width kerns (no kerning, no ligatures).
func (e *Engine) verbatimLine(s string, font fontFace, line int) *boxNode {
	return hpackSP(e.verbNodes(s, font, line), packNatural, 0)
}

// verbNodes turns literal text into a node list: each character is a charNode in
// the verbatim font, each space/tab a fixed kern the width of the font's space.
func (e *Engine) verbNodes(s string, font fontFace, line int) []node {
	space := font.spaceSP().width
	var out []node
	for _, r := range s {
		if r == ' ' || r == '\t' {
			out = append(out, kernNode{width: space})
			continue
		}
		w, h, d := font.charDimsSP(r)
		// Stamp the current colour like ordinary text (see appendChar): \color
		// applies to verbatim too, so \textcolor{red}{\verb|x|} — and a coloured
		// \url under hyperref's colorlinks — reach the drivers in that colour.
		out = append(out, charNode{ch: r, width: w, height: h, depth: d, srcLine: line, color: e.curColor})
	}
	return out
}

// mvlAppendGap adds a small vertical gap (\smallskip-ish) to the main list.
func (e *Engine) mvlAppendGap() {
	e.mvl = append(e.mvl, glueNode{spec: glueSpec{width: 3 * unity}})
	e.prevDepth = ignoreDepth
}
