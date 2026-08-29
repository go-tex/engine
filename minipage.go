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
	var body []tok
	depth := 0
	for {
		t, ok := e.getNext()
		if !ok {
			break // premature end of input: return whatever we gathered
		}
		switch {
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
			case e.endMacroExpandsToEnd(n):
				// \end{n} whose \end<n> macro produces the real terminator by
				// expansion rather than a literal token — elsarticle/ifacconf's
				// abstract is \setbox…=\vbox\bgroup…\begin{minipage}… closed by
				// \endabstract → \end{minipage}\hfill\egroup\egroup. Run \end<n> in
				// place so the generated \end{minipage} surfaces next iteration;
				// otherwise the raw scan runs past it to EOF, swallowing the body.
				e.push(e.eq["end"+n].body)
			default:
				// re-emit \end{n} into the body (an ordinary non-matching \end).
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
	return body
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
