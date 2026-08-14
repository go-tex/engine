// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"unicode/utf8"

	texmath "github.com/go-tex/math"
)

// This file gives the engine math mode. A $…$ (or $$…$$) group is delegated
// whole to the go-tex/math typesetter — the engine does not reimplement the math
// sub-language; it collects the raw math source, hands it to that library, and
// embeds the resulting self-contained SVG as a math box on the current list. The
// library reports only overall width/height (no baseline), so the box is centred
// on the text baseline — an honest approximation until go-tex/math exposes the
// math axis.

// mathRendererT is the engine field's type (a go-tex/math renderer).
type mathRendererT = *texmath.Renderer

// mathNode is a rendered piece of math: its self-contained SVG plus the box
// dimensions (sp) used to place and line-break it.
type mathNode struct {
	svg                  string
	width, height, depth int
}

func (mathNode) isNode() {}

// mathRenderer lazily builds the go-tex/math renderer with its embedded MATH font.
func (e *Engine) mathRenderer() *texmath.Renderer {
	if e.mathR == nil {
		r, err := texmath.New(texmath.DefaultFont())
		if err != nil {
			e.fail("cannot init math renderer: " + err.Error())
			return nil
		}
		e.mathR = r
	}
	return e.mathR
}

// mathSize is the point size at which math is rendered: the current text font's
// size, or 10pt when no font is set.
func (e *Engine) mathSize() int {
	if e.curFont != nil {
		if s := e.curFont.sizePt(); s > 0 {
			return s
		}
	}
	return 10
}

// scanMathSource reads raw (unexpanded) tokens up to the closing math shift,
// reconstructing the math source string for go-tex/math. It returns the source
// and whether the math was display style ($$…$$).
func (e *Engine) scanMathSource() (string, bool) {
	display := false
	if t, ok := e.getNext(); ok {
		if t.cat == catMath && !t.cs_ { // opening $$ ⇒ display math
			display = true
		} else {
			e.back(t)
		}
	}
	var b strings.Builder
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		if t.cat == catMath && !t.cs_ {
			if display { // consume the second $ of the closing $$
				if u, ok := e.getNext(); ok && !(u.cat == catMath && !u.cs_) {
					e.back(u)
				}
			}
			break
		}
		if t.cs_ {
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		} else {
			b.WriteRune(t.ch)
		}
	}
	return b.String(), display
}

// makeMath renders a math source string to a math box.
func (e *Engine) makeMath(src string, display bool) mathNode {
	r := e.mathRenderer()
	if r == nil {
		return mathNode{}
	}
	size := e.mathSize()
	svg, m, err := e.renderMathResolvingMacros(r, src, display, size)
	if err != nil {
		if e.tolerant() {
			// Best-effort preview: a real document's math may use a command
			// go-tex/math does not yet know (a package macro, a user \def, an
			// unimplemented symbol). Drop this one equation rather than aborting the
			// whole compile, tallying the trigger so it can be reported/prioritised.
			e.recordMathSkip(err.Error())
			return mathNode{}
		}
		e.fail("math error in $" + src + "$: " + err.Error())
		return mathNode{}
	}
	// go-tex/math reports the true box: the baseline is at y=Height in the SVG,
	// so the math baseline aligns with the surrounding text baseline (Height above,
	// Depth below) instead of being centred on height/2 — which dropped inline math
	// below the line — and the tight width removes the old padding's side-space.
	return mathNode{svg: svg, width: ptToSP(m.Width), height: ptToSP(m.Height), depth: ptToSP(m.Depth)}
}

// renderMathResolvingMacros renders src, and when go-tex/math rejects a command it
// does not know, it expands that command IN THE SOURCE when — and only when — the
// command is a parameterless user/kernel macro (kind mMacro), then retries. This
// recovers user math shorthands the raw-token scanner emits verbatim, e.g. a
// document's \newcommand\F{\mathbb F} or \def\Z{\mathbb{Z}}: go-tex/math never
// learned \F, but it does know \mathbb, so substituting the macro's replacement
// lets the equation render instead of being dropped.
//
// The substitution is a strict fallback: the literal source is always tried first,
// so every construct go-tex/math already understands (\frac, \sum, \cdot, \mathbf…)
// renders exactly as before and is never expanded — expansion happens only on the
// specific command that failed. That makes the pass regression-free by construction:
// anything that used to render still renders unchanged. A per-name guard (seen) and a
// bounded try count keep a self-referential or non-shrinking macro from looping.
func (e *Engine) renderMathResolvingMacros(r *texmath.Renderer, src string, display bool, size int) (string, texmath.Metrics, error) {
	seen := map[string]bool{}
	var svg string
	var m texmath.Metrics
	var err error
	for tries := 0; tries < 64; tries++ {
		if display {
			svg, m, err = r.RenderDisplaySVGMetrics(src, size)
		} else {
			svg, m, err = r.RenderSVGMetrics(src, size)
		}
		if err == nil {
			return svg, m, nil
		}
		name := unknownMathCommand(err.Error())
		if name == "" || seen[name] {
			return svg, m, err
		}
		next, ok := e.expandMacroInMathSource(src, name)
		if !ok {
			return svg, m, err
		}
		seen[name] = true
		src = next
	}
	return svg, m, err
}

// expandMacroInMathSource substitutes the user/kernel macro \name in a go-tex/math
// source string. A parameterless macro's every occurrence is replaced by its
// replacement text; an argument macro (\newcommand\abs[1]{\lvert#1\rvert},
// \ensuremath, …) has each occurrence's brace/char/cs arguments parsed out of the
// source and substituted into #1…#9 of the body. Primitives, undefined names, and
// macros whose arguments cannot be parsed return ("", false) — left verbatim for
// go-tex/math. scanMathSource emits every control sequence as backslash-name-space,
// which this matches and reproduces.
func (e *Engine) expandMacroInMathSource(src, name string) (string, bool) {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return "", false
	}
	if len(m.params) == 0 {
		return replaceMathCS(src, name, e.toksToString(m.body)), true
	}
	needle := "\\" + name + " "
	n := len(m.params)
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+len(needle):]
		args, consumed, ok := parseMathArgs(rest, n)
		if !ok {
			out.WriteString(needle) // leave this occurrence for go-tex/math to reject
			src = rest
			continue
		}
		out.WriteString(e.substituteMathBody(m.body, args))
		src = rest[consumed:]
		changed = true
	}
	if !changed {
		return "", false
	}
	out.WriteString(src)
	return out.String(), true
}

// replaceMathCS substitutes every occurrence of the control sequence \name in a
// go-tex/math source string with exp. scanMathSource emits every control sequence as
// a backslash, its name, and a single trailing space, so \name is matched with that
// exact trailing space — which also prevents \F from matching inside \Foo.
func replaceMathCS(src, name, exp string) string {
	return strings.ReplaceAll(src, "\\"+name+" ", exp+" ")
}

// parseMathArgs reads n arguments from the start of a go-tex/math source string
// (after a macro's name-space): a {balanced group} (returned without its outer
// braces), a whole control sequence (\name, kept with its trailing space), or a
// single rune. It returns the arguments, how many bytes they consumed, and false if
// an argument is missing or a group is unbalanced.
func parseMathArgs(s string, n int) ([]string, int, bool) {
	p := 0
	args := make([]string, 0, n)
	for k := 0; k < n; k++ {
		for p < len(s) && s[p] == ' ' {
			p++
		}
		if p >= len(s) {
			return nil, 0, false
		}
		switch s[p] {
		case '{':
			depth, start := 0, p
			for p < len(s) {
				if s[p] == '{' {
					depth++
				} else if s[p] == '}' {
					depth--
				}
				p++
				if depth == 0 {
					break
				}
			}
			if depth != 0 {
				return nil, 0, false
			}
			args = append(args, s[start+1:p-1])
		case '\\':
			start := p
			p++
			for p < len(s) && (s[p] == '@' || (s[p] >= 'a' && s[p] <= 'z') || (s[p] >= 'A' && s[p] <= 'Z')) {
				p++
			}
			arg := s[start:p]
			if p < len(s) && s[p] == ' ' {
				p++
			}
			args = append(args, arg+" ")
		default:
			r, w := utf8.DecodeRuneInString(s[p:])
			args = append(args, string(r))
			p += w
		}
	}
	return args, p, true
}

// substituteMathBody renders a macro body to go-tex/math source, replacing each #k
// parameter token with args[k-1]. Non-parameter tokens use scanMathSource's
// encoding (backslash-name-space for a cs, the rune otherwise).
func (e *Engine) substituteMathBody(body []tok, args []string) string {
	var b strings.Builder
	for _, t := range body {
		if !t.cs_ && t.cat == catParam && t.ch >= '1' && t.ch <= '9' {
			if i := int(t.ch - '1'); i < len(args) {
				b.WriteString(args[i])
			}
			continue
		}
		if t.cs_ {
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		} else {
			b.WriteRune(t.ch)
		}
	}
	return b.String()
}

// unknownMathCommand extracts the command name X from a go-tex/math "unknown command
// \X" error, or "" when the message is a different failure. The name runs over letters
// and @ (a LaTeX-internal control word), matching a TeX control-word token.
func unknownMathCommand(errMsg string) string {
	const marker = "unknown command \\"
	i := strings.Index(errMsg, marker)
	if i < 0 {
		return ""
	}
	rest := errMsg[i+len(marker):]
	j := 0
	for j < len(rest) && (rest[j] == '@' || (rest[j] >= 'a' && rest[j] <= 'z') || (rest[j] >= 'A' && rest[j] <= 'Z')) {
		j++
	}
	return rest[:j]
}

// recordMathSkip tallies a dropped equation under skippedCS. When the error is
// go-tex/math's "unknown command \X" it keys on that command (so the report shows
// which math macro to implement next); otherwise it keys on a generic "$math$".
func (e *Engine) recordMathSkip(errMsg string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	key := "$math$"
	if name := unknownMathCommand(errMsg); name != "" {
		key = "\\" + name
	}
	e.skippedCS[key]++
}

// doMath handles a math-shift token: collect the source, render it, and place the
// math box. Inline math ($…$) joins the current line; display math ($$…$$) ends
// the paragraph and is centred on its own line (to \hsize with \hfil on each side).
func (e *Engine) doMath() {
	src, display := e.scanMathSource()
	e.placeMath(e.makeMath(src, display), display)
}

// doDelimitedMath handles LaTeX's \(…\) (inline) and \[…\] (display): it collects
// the raw math source up to the closing control sequence and places it.
func (e *Engine) doDelimitedMath(closeName string, display bool) {
	src := e.collectMathUntilCS(closeName)
	e.placeMath(e.makeMath(src, display), display)
}

// collectMathUntilCS reads raw tokens (no expansion) up to a control sequence
// named close, reconstructing the math source for go-tex/math.
func (e *Engine) collectMathUntilCS(close string) string {
	var b strings.Builder
	for {
		t, ok := e.getNext()
		if !ok || (t.cs_ && t.cs == close) {
			break
		}
		if t.cs_ && t.cs != close && e.expandsToCloseCS(t, close) {
			// A user macro standing in for the closing \] or \) (e.g.
			// \newcommand\dclose{\]}): read raw here it hides the terminator, so the
			// scanner would run to EOF and swallow the rest of the document. Expand it
			// in place so the real close cs surfaces next iteration. Narrow: only a
			// parameterless macro whose body begins with the exact close cs, so ordinary
			// math macros stay verbatim in the collected source.
			e.expandMacro(e.meaningOf(t))
			continue
		}
		if t.cs_ {
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		} else {
			b.WriteRune(t.ch)
		}
	}
	return b.String()
}

// placeMath positions a math box: display math ends the paragraph and is centred
// on its own line; inline math joins the current line (starting one if needed).
func (e *Engine) placeMath(m mathNode, display bool) {
	if display {
		e.endParagraph()
		fil := glueNode{spec: glueSpec{stretch: unity, stretchOrder: 1}}
		e.contribute(hpackSP([]node{fil, m, fil}, packTo, e.hsize))
		return
	}
	if !e.inPar {
		e.beginParagraph(false)
	}
	e.parList = append(e.parList, m)
}
