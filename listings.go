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
	content, line := e.readRawEnvBody(`\end{lstlisting}`)
	e.renderVerbatimBlock(content, line, parseLstOptions(opts))
}

// readRawEnvBody copies the raw source from the cursor up to the literal end marker
// (e.g. "\end{lstlisting}") — no expansion, no category codes — closes the group
// \begin opened (the marker is consumed here, so the matching \end prim never
// runs), and trims the newline right after \begin and right before \end the way
// doVerbatim does. It returns the body text and the source line its first content
// character lives on (for glyph stamping). It is shared by every verbatim-body
// environment: lstlisting and minted read their bodies identically.
func (e *Engine) readRawEnvBody(end string) (content string, firstLine int) {
	// The body may not be in the character buffer at all. A minipage, a captured
	// float or a beamer column reads its whole body as TOKENS first and typesets it
	// afterwards, by which time the buffer's cursor sits PAST the captured body —
	// so scanning the buffer here copies the document that FOLLOWS and swallows it.
	// Measured: a minipage holding one lstlisting ate \end{minipage}, the text after
	// it and \end{document}, then printed the listing at the end of the page.
	if body, ok := e.rawEnvBodyFromLists(end); ok {
		e.endEnvGroup() // as below: this environment read its own \end
		line, _ := e.lineColAt(e.srcPos)
		return trimVerbEdges(body), line
	}
	startOff := e.bpos
	rest := string(e.base[e.bpos:])
	idx := strings.Index(rest, end)
	if idx < 0 {
		content = rest
		e.bpos = len(e.base)
	} else {
		content = rest[:idx]
		e.bpos += len([]rune(rest[:idx])) + len([]rune(end))
	}
	e.endEnvGroup() // this environment reads its own \end, so it closes \begin's group
	leadingNL := strings.HasPrefix(content, "\n")
	content = strings.TrimPrefix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	firstLine, _ = e.lineColAt(startOff)
	if leadingNL {
		firstLine++
	}
	return content, firstLine
}

// rawEnvBodyFromLists reads a verbatim environment's body out of the PENDING TOKEN
// LISTS, up to the \end{name} that end names, and reports whether it found it. It
// never touches the character buffer: when the marker is not in the lists it puts
// back everything it took and answers false, so an ordinary document — whose body
// is still in the buffer — takes the scan below exactly as before.
//
// The tokens went through the mouth when the enclosing body was captured, so this is
// verbatim only up to what tokenising already decided: a % comment took its line
// with it there. That is a smaller loss than the alternative, which is losing the
// rest of the document.
func (e *Engine) rawEnvBodyFromLists(end string) (string, bool) {
	name := strings.TrimSuffix(strings.TrimPrefix(end, `\end{`), "}")
	var taken []tok
	for len(e.lists) > 0 {
		t, ok := e.getNext()
		if !ok {
			break
		}
		taken = append(taken, t)
		if !t.cs_ || t.cs != "end" {
			continue
		}
		if n, ok := e.peekBraceNameFromLists(); ok && n == name {
			return e.toksToString(taken[:len(taken)-1]), true
		}
	}
	e.push(taken) // not ours: leave the input exactly as it was
	return "", false
}

// peekBraceNameFromLists reads a {name} group from the pending lists and CONSUMES it,
// used right after an \end token to see which environment it closes.
func (e *Engine) peekBraceNameFromLists() (string, bool) {
	var taken []tok
	var name []rune
	depth := 0
	for len(e.lists) > 0 {
		t, ok := e.getNext()
		if !ok {
			break
		}
		taken = append(taken, t)
		if t.cs_ {
			break // a control sequence inside the braces: not a plain environment name
		}
		switch t.cat {
		case catBegin:
			depth++
			if depth > 1 {
				e.push(taken)
				return "", false
			}
		case catEnd:
			return string(name), depth == 1
		default:
			if depth == 1 {
				name = append(name, t.ch)
			}
		}
	}
	e.push(taken)
	return "", false
}

// trimVerbEdges drops the newline right after \begin and right before \end, the way
// the buffer scan does.
func trimVerbEdges(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "\n"), "\n")
}

// renderVerbatimBlock sets content as a code block: each line verbatim in the tt
// font (see verbFont), starting at source line firstLine, with a little vertical
// gap above and below. When o.numbers is set each line gets a right-aligned number
// in a fixed-width gutter; when o.frame is set the whole block is wrapped in a
// frame. Shared by lstlisting and minted.
func (e *Engine) renderVerbatimBlock(content string, firstLine int, o lstOptions) {
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
			b := e.verbatimLine(e.lstText(ln, i+1, digits, o.numbers), font, firstLine+i)
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
			e.appendToPage(e.verbatimLine(e.lstText(ln, i+1, digits, o.numbers), font, firstLine+i))
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
	e.setInlineVerbatim()
}

// setInlineVerbatim reads a \verb-style delimited code span straight from the raw
// input at the cursor and sets it inline in the tt font. The delimiter is the next
// character, matched by its twin (\lstinline|code|), except '{' which is matched by
// '}' (\lstinline{code}). Shared by \lstinline and \mintinline.
func (e *Engine) setInlineVerbatim() {
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
