// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's minipage environment: \begin{minipage}[pos]{width}
// … \end{minipage}. It is the environment form of \parbox — the body is the tokens
// between \begin{minipage} and the matching \end{minipage} rather than a braced
// group. The body is typeset (paragraphs, lists, nested environments — the normal
// LaTeX treatment) to the requested width and packed into a vbox placed inline, so
// several minipages sit side by side. The optional [pos] sets the box's vertical
// reference point — t (top line's baseline), b (bottom line's baseline) or c (the
// default, vertically centred), exactly as for \parbox.

// doMinipage implements \begin{minipage}[pos]{width} … \end{minipage}. It reads the
// optional [pos] and mandatory {width}, collects the environment body up to the
// matching \end{minipage}, typesets it at \hsize == width (saved/restored around the
// build), fixes the resulting vbox to that width, re-anchors it per [pos] and places
// it inline.
func (e *Engine) doMinipage() {
	pos := e.scanOptBracketPos() // t / c / b (default c)
	width := e.readBraceDimen()
	body := e.collectEnvBody("minipage")

	// \noindent prefix: like \parbox, the body's first paragraph has no \parindent
	// box (which would overflow a narrow box and defeat line breaking).
	content := append([]tok{csTok("noindent")}, body...)

	savedHsize := e.hsize
	e.hsize = width
	vbox := e.typesetGroupToVbox(content) // breaks the paragraphs to e.hsize == width
	e.hsize = savedHsize

	vbox.width = width
	e.place(alignParbox(vbox, pos))
}

// collectEnvBody reads raw tokens (no expansion) up to the matching \end{name} and
// returns the body tokens. It tracks nesting: a \begin{name} deeper in the body
// increments the depth and a \end{name} decrements it, so only the \end{name} at
// depth 0 terminates collection. \begin/\end of any other environment (a nested
// list, say) are copied verbatim into the body so they are processed normally when
// the body is later typeset. The pattern mirrors collectTabularBody.
func (e *Engine) collectEnvBody(name string) []tok {
	// Where the input stood before the scan, so nothing is lost if the \end never
	// comes. A body collected up front assumes the environment's \end is IN the input;
	// it need not be. beamer's rounded block opens \begin{minipage} inside
	// \beamerboxesrounded and only produces the matching \end later, from
	// \endbeamerboxesrounded — so the scanner drains what is pending, carries on into
	// the document text and swallows the rest of it. Measured, a talk with one rounded
	// block rendered ZERO pages (issue #115).
	mark, level := e.markInput(), len(e.levels)
	var body []tok
	depth := 0
	// Where the last \end{other} was stored, and for which environment. A scan that is
	// about to leave the file it began in gets ONE chance to reconsider it: see the
	// rewind below.
	lastEnd, lastEndName, rewound := -1, "", false
	for {
		t, ok := e.getNext()
		if ok && !rewound && len(e.levels) < level && lastEnd >= 0 && e.endMacroLeadsToEnd(lastEndName) {
			// The scan has run off the end of the file it began in, and the last
			// \end{other} it stored leads to an \end after all — through one more macro,
			// which the narrow rule cannot see (endMacroLeadsToEnd). beamer's \column
			// opens \begin{minipage} and stores its \end{minipage} in \beamer@colclose,
			// run at \end{columns}: read raw, that \end{columns} was stored and the scan
			// went on into the document behind the file.
			//
			// Rewind to it and put the stored tail back. Nothing is re-EXECUTED: what is
			// pushed back was gathered, never run — which is what makes this sound where
			// re-running the whole scan is not.
			rewound = true
			tail := append([]tok(nil), body[lastEnd:]...)
			body = body[:lastEnd]
			hdr := 1 + len(braceNameToks(lastEndName))
			e.back(t)
			if len(tail) > hdr {
				e.push(append([]tok(nil), tail[hdr:]...))
			}
			e.push(append([]tok(nil), e.eq["end"+lastEndName].body...))
			lastEnd = -1
			continue
		}
		if !ok {
			// The \end never came. Put back everything the scan read, so the material
			// flows as ordinary text — a locally wrong box, not a lost document.
			//
			// Only when the scan stayed on ONE input level: a mark records where the
			// mouth stood in the buffer it was reading, and a file opened or finished
			// mid-scan makes that position meaningless. Restoring across one cost two
			// arXiv papers, one of them everything it had.
			if len(e.levels) == level {
				e.restoreInput(mark)
				return nil
			}
			return body
		}
		switch {
		case e.runsCsname(t):
			// \csname builds a control sequence that may be the \end this scanner hunts
			// (see runsCsname): run it so the sequence surfaces on the next iteration.
			e.runCsname(t)
		case depth == 0 && name == "minipage" && t.cs_ && t.cs == "column" && e.expandsToEnd(csTok("beamer@colclose")):
			// beamer's \column begins by running \beamer@colclose, which holds the
			// PREVIOUS column's closer (beamerbaseframecomponents.sty:281-283):
			//
			//	\newcommand<>\beamer@columncom[2][\beamer@colmode]{%
			//	  \beamer@colclose
			//	  \def\beamer@colclose{\end{minipage}\hfill\end{actionenv}\ignorespaces}%
			//
			// so a \column met while collecting a minipage's body IS that minipage's
			// deferred \end — and a raw scan never sees it, \column taking arguments
			// where the narrow expandsToEnd rule only reaches parameterless macros.
			// Read raw, the first column swallowed its sibling and everything after the
			// frame: a talk with two columns in a [fragile] frame rendered one page.
			//
			// The body ends here and \column goes back to open the next column;
			// \beamer@colclose is emptied because its \end{minipage} has just been
			// honoured, which is what beamer itself does after running it (:269).
			e.back(t)
			e.define("beamer@colclose", &meaning{kind: mMacro}, true)
			e.endEnvGroup()
			return body
		case t.cs_ && t.cs != "end" && t.cs != "begin" && e.expandsToEnd(t):
			// A user macro standing in for \end{...} (e.g. \newcommand\emp{\end{minipage}}):
			// read raw here it hides the \end, so the body scanner would run to EOF and
			// swallow the rest of the document. Expand it in place so the real \end token
			// surfaces next iteration (where the depth bookkeeping then handles it).
			// Narrow: only a parameterless macro whose body begins with \end (see
			// expandsToEnd), so ordinary body macros pass through into the body verbatim.
			e.expandMacro(e.meaningOf(t))
		case t.cs_ && t.cs == "end":
			n := e.readBraceName()
			switch {
			case n == name && depth == 0:
				e.endEnvGroup() // this \end was read here, so \end's \endgroup will not run
				return body     // the matching \end{name}: consume it and stop
			case n == name:
				depth-- // a nested instance closes; re-emit for the nested typeset
				body = append(body, csTok("end"))
				body = append(body, braceNameToks(n)...)
			case name == "minipage" && depth == 0 && e.endMacroLeadsToEnd(n) &&
				e.expandsToEnd(csTok("beamer@colclose")):
				// \end{columns} closes the open column the same way \column does: its
				// \endcolumns begins with \beamer@colclose (beamerbaseframecomponents
				// .sty:237), which holds this minipage's \end. Stored raw, the scan ran
				// past the whole columns block and shredded the frame — 17 pages to 1.
				// Push \end<n>'s body so \beamer@colclose, and then its \end{minipage},
				// surface for the depth bookkeeping.
				e.push(e.eq["end"+n].body)
			case e.endMacroExpandsToEnd(n):
				// \end{n} whose \end<n> macro produces the real terminator by
				// expansion rather than a literal token — elsarticle/ifacconf's
				// abstract is \setbox…=\vbox\bgroup…\begin{minipage}… closed by
				// \endabstract → \end{minipage}\hfill\egroup\egroup. Run \end<n> in
				// place so the generated \end{minipage} surfaces next iteration;
				// otherwise the raw scan runs past it to EOF, swallowing the body.
				e.push(e.eq["end"+n].body)
			default:
				// re-emit \end{n} into the body (an ordinary non-matching \end), and
				// remember where, in case the scan later runs out of file (see above).
				if depth == 0 {
					lastEnd, lastEndName = len(body), n
				}
				body = append(body, csTok("end"))
				body = append(body, braceNameToks(n)...)
			}
		case t.cs_ && t.cs == "begin":
			n := e.readBraceName()
			if n == name {
				depth++ // a nested instance of this environment
			}
			// re-emit \begin{n} so the nested environment is opened when the body
			// is typeset (a nested minipage re-collects its own body then).
			body = append(body, csTok("begin"))
			body = append(body, braceNameToks(n)...)
		default:
			body = append(body, t)
		}
	}
}

// braceNameToks rebuilds the token sequence for a {name} group: an explicit-begin
// brace, the name's characters, and an explicit-end brace. Used to re-emit a
// \begin{other}/\end{other} that collectEnvBody read while scanning for its own
// terminator.
func braceNameToks(name string) []tok {
	toks := make([]tok, 0, len(name)+2)
	toks = append(toks, chTok('{', catBegin))
	for _, r := range name {
		toks = append(toks, chTok(r, catOther))
	}
	toks = append(toks, chTok('}', catEnd))
	return toks
}
