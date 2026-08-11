// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the listings package's code-block facilities on top of the
// engine's verbatim machinery (see verbatim.go): the lstlisting environment and
// the \lstinline inline command. Like verbatim, their bodies are read raw from the
// base input — no macro expansion, no category codes, so \, {, }, $, % and friends
// are ordinary characters, and spaces and line breaks are preserved exactly. The
// content is set in the tt font when one is bound (Options.MonoFont / \tt), else
// the current font.
//
// lstlisting accepts an optional "[key=value,…]" argument. Two keys change the
// output: numbers=left prepends a right-aligned line number to each line, and
// frame=single draws a thin rule around the whole block. Any other key
// (language=…, caption=…, basicstyle=…, …) is accepted and silently ignored so a
// real-world listing does not choke. In particular, LANGUAGE-AWARE SYNTAX
// HIGHLIGHTING IS OUT OF SCOPE: language= is parsed but never colourises the code;
// per-language tokenising and colouring is a future enhancement.

import (
	"fmt"
	"strconv"
	"strings"
)

// lstOptions is the parsed, honoured subset of a listings option string. Keys the
// engine does not model leave every field at its zero value.
type lstOptions struct {
	numbers bool // numbers=left (any value other than "" / "none" turns numbering on)
	frame   bool // frame=single (any value other than "" / "none" draws a frame)
}

// parseLstOptions parses a listings "[key=value,…]" option body into the honoured
// subset. Whitespace around keys and values is trimmed; unknown keys are ignored;
// a key with no "=value" is treated as present-but-empty. numbers/frame are on for
// any explicit value other than the literal "none" (so numbers=left, numbers=right
// and frame=single all read as on, while numbers=none / frame=none read as off).
func parseLstOptions(s string) lstOptions {
	var o lstOptions
	if strings.TrimSpace(s) == "" {
		return o
	}
	for _, part := range strings.Split(s, ",") {
		key, val, _ := strings.Cut(part, "=")
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		on := val != "" && val != "none"
		switch key {
		case "numbers":
			o.numbers = on
		case "frame":
			o.frame = on
		}
	}
	return o
}

// scanRawOptBracket reads an optional "[...]" straight from the raw base input at
// the cursor — the counterpart of scanOptBracketToks for the verbatim-style
// environments whose bodies are consumed raw rather than tokenized. Leading spaces
// and tabs are skipped; if a '[' follows, the text up to the next ']' is returned
// and the cursor advances past the ']'. With no bracket present nothing is
// consumed and ("", false) is returned.
func (e *Engine) scanRawOptBracket() (string, bool) {
	i := e.bpos
	for i < len(e.base) && (e.base[i] == ' ' || e.base[i] == '\t') {
		i++
	}
	if i >= len(e.base) || e.base[i] != '[' {
		return "", false
	}
	j := i + 1
	for j < len(e.base) && e.base[j] != ']' {
		j++
	}
	opts := string(e.base[i+1 : j])
	if j < len(e.base) {
		j++ // consume the closing ']'
	}
	e.bpos = j
	return opts, true
}

// doLstlisting typesets a \begin{lstlisting}[opts] … \end{lstlisting} block. It is
// reached via \begin, so the cursor sits just past "{lstlisting}"; it reads the
// optional [opts] and then copies the raw source up to the literal
// "\end{lstlisting}", setting each line verbatim in the tt font (mirroring
// doVerbatim's raw scan and its trimming of the newline right after \begin and
// right before \end).
func (e *Engine) doLstlisting() {
	opts, _ := e.scanRawOptBracket()
	o := parseLstOptions(opts)

	const end = `\end{lstlisting}`
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
	leadingNL := strings.HasPrefix(content, "\n")
	content = strings.TrimPrefix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	// The source line the first content character lives on (for glyph stamping).
	line, _ := e.lineColAt(startOff)
	if leadingNL {
		line++
	}

	e.endParagraph() // flush any open paragraph before the block
	font := e.verbFont()
	if font == nil {
		return
	}

	lines := strings.Split(content, "\n")
	// A right-aligned gutter wide enough for the largest line number, when numbering.
	digits := len(strconv.Itoa(len(lines)))

	e.mvlAppendGap() // a little space above the block
	if o.frame {
		// Collect the line boxes into a vbox with interline glue, then wrap the vbox
		// in a frame (reusing boxframe.go's frameNode at the \fbox defaults) and put
		// the whole thing on the page as a single unit.
		var vlist []node
		prevDepth := ignoreDepth
		for i, ln := range lines {
			b := e.verbatimLine(e.lstText(ln, i+1, digits, o.numbers), font, line+i)
			if prevDepth > ignoreDepth {
				gap := e.baselineskip - prevDepth - b.height
				if gap < e.lineskip {
					gap = e.lineskip
				}
				vlist = append(vlist, glueNode{spec: glueSpec{width: gap}})
			}
			vlist = append(vlist, b)
			prevDepth = b.depth
		}
		vbox := vpackSP(vlist, packNatural, 0)
		fr := frameNode{inner: vbox, sep: fboxSep, rule: fboxRule}
		// Wrap the frame in an hbox so it sits on the main vertical list as an
		// ordinary box (with proper interline glue and page-break handling).
		e.appendToPage(hpackSP([]node{fr}, packNatural, 0))
	} else {
		for i, ln := range lines {
			e.appendToPage(e.verbatimLine(e.lstText(ln, i+1, digits, o.numbers), font, line+i))
		}
	}
	e.mvlAppendGap()
}

// lstText returns the literal text of one listing line, optionally prefixed with a
// right-aligned line number in a fixed-width gutter. Because the tt font is
// monospaced, padding the number with leading spaces aligns the numbers and keeps
// the code columns straight; the two trailing spaces separate the number from the
// code.
func (e *Engine) lstText(line string, n, digits int, numbers bool) string {
	if !numbers {
		return line
	}
	return fmt.Sprintf("%*d  %s", digits, n, line)
}

// doLstinline implements \lstinline: an inline verbatim like \verb, with an
// optional leading [opts] (accepted; the honoured options do not affect inline
// rendering). The code is delimited either by a matching pair of an arbitrary
// character (\lstinline|code|) or, when that character is '{', by the closing '}'
// (\lstinline[opts]{code}). The text joins the current paragraph inline in the tt
// font.
func (e *Engine) doLstinline() {
	e.scanRawOptBracket() // optional [opts]; ignored for inline rendering
	if e.bpos >= len(e.base) {
		return
	}
	open := e.base[e.bpos]
	e.bpos++
	closer := open
	if open == '{' {
		closer = '}'
	}
	start := e.bpos
	for e.bpos < len(e.base) && e.base[e.bpos] != closer {
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
