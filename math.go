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
	svg string
	// src is the formula's LaTeX source, carried so the searchable text layer
	// can say what the formula IS. A rendered formula is glyph outlines like any
	// other text, and without this a page reads "The mass is exactly" with a
	// silent hole where the equation was. The source is what the author typed
	// and what they would type to search for it again; a spoken form ("E equals
	// m c squared") is a separate, language-dependent problem.
	src                  string
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
	depth := 0 // brace nesting: a $ only ends the math at the top level
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		if isFileEndSentinel(t) { // an unterminated $/$$ must not cross the file boundary
			e.back(t)
			break
		}
		// \csname can only build a control sequence, and this scanner reads raw
		// tokens: left unexecuted it reaches the maths layer as the literal word
		// "csname". \end{env} goes the same way — and a class that builds its
		// display out of $$ closes it INSIDE \end<env>, so running it is the only
		// way the closing $$ ever arrives (iopart.cls: \def\endequation{$$}).
		// \end{array} and \end{cases}, whose \end<env> holds no math shift, stay
		// literal for the maths layer to read.
		if e.runsCsname(t) {
			e.runCsname(t)
			continue
		}
		if t.cs_ && t.cs == "end" {
			if name, ok := e.peekBracedName(); ok {
				if m := e.eq["end"+name]; m != nil && m.kind == mMacro && bodyHasMathShift(m.body) {
					e.expandMacro(m)
					continue
				}
				e.push(braceNameToks(name))
			}
		}
		// A $ inside a \mbox{…$…$…}/\text{…} group (depth > 0) is that group's own
		// nested inline math, NOT the closing shift — treating it as the close used
		// to truncate the math here and desynchronise every $ in the rest of the
		// document (a runaway that swallowed paragraphs of text as "math").
		if t.cat == catMath && !t.cs_ && depth == 0 {
			if display { // consume the second $ of the closing $$
				if u, ok := e.getNext(); ok && !(u.cat == catMath && !u.cs_) {
					e.back(u)
				}
			}
			break
		}
		if !t.cs_ {
			switch t.cat {
			case catBegin:
				depth++
			case catEnd:
				if depth > 0 {
					depth--
				}
			}
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

// peekBracedName reads a {name} that follows, returning the name; when the next
// token is not a brace the input is left exactly as it was. The caller pushes the
// name back with braceNameToks when it decides not to act on it.
func (e *Engine) peekBracedName() (string, bool) {
	t, ok := e.getNext()
	if !ok {
		return "", false
	}
	if !(t.cat == catBegin && !t.cs_) {
		e.back(t)
		return "", false
	}
	var b strings.Builder
	for {
		u, ok := e.getNext()
		if !ok {
			return b.String(), true
		}
		if !u.cs_ && u.cat == catEnd {
			return b.String(), true
		}
		if !u.cs_ {
			b.WriteRune(u.ch)
		}
	}
}

// bodyHasMathShift reports whether a macro body carries a math shift ($), which is
// what makes \end<env> the thing that closes a $$ display.
func bodyHasMathShift(body []tok) bool {
	for _, t := range body {
		if !t.cs_ && t.cat == catMath {
			return true
		}
	}
	return false
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
	// go-tex/math paints glyphs with fill="currentColor" so a formula can follow
	// the surrounding text colour. Bake the ACTUAL current colour in (black by
	// default, or the active \textcolor) instead of leaving currentColor to be
	// resolved by the host page's CSS `color` — which on a themed page (the
	// playground) is a faint default, rendering formulas nearly invisible or, in
	// a browser's auto-dark mode, inconsistently with the body text. An explicit
	// colour makes math and text always render identically.
	color := "black"
	if e.curColor != 0 {
		color = hexColor(e.curColor)
	}
	svg = strings.ReplaceAll(svg, "currentColor", color)
	// go-tex/math reports the true box: the baseline is at y=Height in the SVG,
	// so the math baseline aligns with the surrounding text baseline (Height above,
	// Depth below) instead of being centred on height/2 — which dropped inline math
	// below the line — and the tight width removes the old padding's side-space.
	return mathNode{svg: svg, src: src, width: ptToSP(m.Width), height: ptToSP(m.Height), depth: ptToSP(m.Depth)}
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
	var svg string
	var m texmath.Metrics
	var err error
	// The overall try bound stops a mutually-recursive pair (\a→\b→\a) that keeps
	// changing the source forever. Progress within it is guarded per step by the
	// fixpoint check below, not by a "seen once" set — a document's own \newcommand
	// is often used NESTED (\prn{…\prn{…}}), so the same name must be expandable more
	// than once as the outer occurrence exposes the inner ones.
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
		if name == "" {
			return svg, m, err
		}
		next, ok := e.expandMacroInMathSource(src, name)
		if !ok {
			// Colour commands (\color, \textcolor, …) are primitives go-tex/math
			// cannot render. Strip them from the source — keeping any content
			// argument — so the equation typesets in the surrounding colour instead
			// of being dropped whole.
			if stripped, sok := stripMathColor(src, name); sok {
				src = stripped
				continue
			}
			// Cross-reference / citation commands (\ref, \eqref, \cite …) are engine
			// primitives that resolve to text at typeset time, so go-tex/math cannot
			// render them. Substitute their resolved text (or a neutral placeholder)
			// into the source, so an equation carrying a \eqref{…} still typesets.
			if resolved, rok := e.resolveMathRef(src, name); rok {
				src = resolved
				continue
			}
			// Non-rendering commands (spacing \hspace/\mspace; metadata \label,
			// \nonumber, …) must not drop the equation: spacing becomes a space, the
			// rest is removed, then retry.
			// siunitx quantities: composed here rather than dropped, with the unit
			// upright as siunitx sets it.
			if q, qok := e.resolveMathSIUnitx(src, name); qok {
				src = q
				continue
			}
			if noised, nok := stripMathNoise(src, name); nok {
				src = noised
				continue
			}
			// Glue primitives (\hskip\mathindent and friends) carry a <glue>, not a
			// {group}: they and the glue they take are removed.
			if glued, gok := stripMathGlue(src, name); gok {
				src = glued
				continue
			}
			return svg, m, err
		}
		if next == src {
			// A self-referential macro (\def\x{\x}) expands to itself: no progress,
			// so stop rather than spin. The equation is dropped and recorded.
			return svg, m, err
		}
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

// mathColorCmds are the xcolor/color commands the engine implements as primitives
// (so the math macro-retry cannot expand them) and which go-tex/math does not
// render. Each maps to the count of mandatory {brace} arguments and the 0-based
// index of the one to keep as content (-1 = drop the command outright). A leading
// optional [model] argument is skipped separately.
var mathColorCmds = map[string]struct{ mand, keep int }{
	"color":       {1, -1}, // \color[model]{name}
	"pagecolor":   {1, -1},
	"normalcolor": {0, -1},
	"textcolor":   {2, 1}, // \textcolor[model]{name}{content}
	"colorbox":    {2, 1}, // \colorbox[model]{name}{content}
	"fcolorbox":   {3, 2}, // \fcolorbox[model]{frame}{bg}{content}
}

// stripMathColor removes every occurrence of the colour command \name from a
// go-tex/math source string, keeping any content argument, and reports whether it
// changed anything. The equation then renders in the surrounding colour rather than
// being dropped. An occurrence whose arguments cannot be parsed is left verbatim
// (for go-tex/math to reject) so a malformed source is never silently corrupted.
func stripMathColor(src, name string) (string, bool) {
	spec, ok := mathColorCmds[name]
	if !ok {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := skipMathOptArg(src[i+len(needle):])
		args, consumed, ok := parseMathArgs(rest, spec.mand)
		if !ok {
			out.WriteString(needle)
			src = src[i+len(needle):]
			continue
		}
		if spec.keep >= 0 && spec.keep < len(args) {
			out.WriteString(args[spec.keep])
		}
		src = rest[consumed:]
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// mathRefArity is the set of cross-reference / citation commands the engine
// resolves inside math source, each taking one {key} argument (plus, for the cite
// family, an optional [note]). Their value is text, so go-tex/math cannot render
// them; resolveMathRef substitutes the resolved label or a neutral placeholder.
var mathRefArity = map[string]bool{
	"ref": true, "eqref": true, "pageref": true, "autoref": true,
	"cref": true, "Cref": true, "nameref": true, "vref": true,
	"cite": true, "citep": true, "citet": true, "citealp": true,
	"citealt": true, "citeauthor": true, "citeyear": true, "Citep": true, "Citet": true,
}

// mathSIUnitxArity is siunitx's quantity commands and how many braced arguments
// each takes. The engine already composes them for text (siunitx.go); in a formula
// they reached the maths layer as unknown commands and cost the whole equation —
// 158 of the 4172 formulas the arXiv corpus drops are \SI.
var mathSIUnitxArity = map[string]int{
	"num": 1, "ang": 1, "si": 1, "unit": 1, "SI": 2, "qty": 2,
}

// resolveMathSIUnitx substitutes a siunitx quantity into a maths source string,
// composing it with the same functions the text-mode primitives use.
//
// The unit goes inside \mathrm, which is siunitx's own default (unit-font-command =
// \mathrm, siunitx.sty:6209, and \__siunitx_print_math_auxvi:Nn \mathrm at :3910)
// and the SI rule it implements: a unit symbol is upright, never italic — which is
// what a maths layer would otherwise make of "m/s".
func (e *Engine) resolveMathSIUnitx(src, name string) (string, bool) {
	n, ok := mathSIUnitxArity[name]
	if !ok {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := skipMathOptArg(src[i+len(needle):]) // \SI[per-mode=symbol]{…}{…}
		args, consumed, ok := parseMathArgs(rest, n)
		if !ok {
			out.WriteString(needle)
			src = src[i+len(needle):]
			continue
		}
		out.WriteString(mathQuantityText(name, args))
		src = rest[consumed:]
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// mathQuantityText composes one siunitx quantity as maths source.
func mathQuantityText(name string, args []string) string {
	unit := func(a string) string {
		u := mathThinSpace(formatUnitTokens(tokenizeTeX(a)))
		if u == "" {
			return ""
		}
		return "\\mathrm {" + u + "}"
	}
	switch name {
	case "num":
		v, err := formatNumber(args[0])
		if err != nil {
			return ""
		}
		return mathThinSpace(v)
	case "ang":
		return mathThinSpace(formatAngle(args[0]))
	case "si", "unit":
		return unit(args[0])
	default: // SI, qty
		v, err := formatNumber(args[0])
		u := unit(args[1])
		switch {
		case err != nil || v == "":
			return u
		case u == "":
			return mathThinSpace(v)
		}
		return mathThinSpace(v) + "\\, " + u
	}
}

// mathThinSpace rewrites the \thinspace the text composer emits as the \, the maths
// layer knows; both are TeX's thin space, spelled for their own mode.
func mathThinSpace(s string) string {
	return strings.ReplaceAll(s, "\\thinspace ", "\\, ")
}

// resolveMathRef replaces every \name{key} cross-reference or citation in a math
// source string with its resolved text, reporting whether it changed anything.
func (e *Engine) resolveMathRef(src, name string) (string, bool) {
	if !mathRefArity[name] {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := skipMathOptArg(src[i+len(needle):]) // \cite[p. 3]{key}
		args, consumed, ok := parseMathArgs(rest, 1)
		if !ok {
			out.WriteString(needle)
			src = src[i+len(needle):]
			continue
		}
		out.WriteString(e.mathRefText(name, args[0]))
		src = rest[consumed:]
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// mathRefText is the text a resolved cross-reference / citation contributes to
// math. Label lookups reuse the shared refText resolver ("??" when unresolved).
func (e *Engine) mathRefText(name, key string) string {
	switch name {
	case "eqref":
		return "(" + e.refText(key) + ")"
	case "cite", "citep", "citet", "citealp", "citealt", "citeauthor", "citeyear", "Citep", "Citet":
		return "[?]" // the bibliography is not resolved in the math pass
	default: // \ref \autoref \cref \Cref \nameref \vref \pageref
		return e.refText(key)
	}
}

// mathNoise are commands that carry no math to render but must not drop the
// equation: horizontal spacing (replaced by a thick space the math layer knows) and
// metadata (removed). mand is the count of {brace} arguments to consume.
var mathNoise = map[string]struct {
	mand int
	repl string
}{
	"hspace": {1, `\; `}, "mspace": {1, `\; `}, "vspace": {1, ``},
	"label": {1, ``}, "nonumber": {0, ``}, "notag": {0, ``},
	"qedhere": {0, ``}, "noalign": {1, ``},
}

// mathGlue are the glue primitives a class writes INTO a display: iopart.cls opens
// every equation with \displaystyle\hskip\mathindent to indent it instead of
// centring it. Their argument is a <glue>, not a {group}, so they cannot go in
// mathNoise — and left in place they cost the whole equation, since go-tex/math
// renders no glue.
var mathGlue = map[string]bool{
	"hskip": true, "vskip": true, "kern": true, "mskip": true, "mkern": true,
}

// stripMathGlue removes every occurrence of a glue primitive and the <glue> that
// follows it from a math source string, reporting whether it changed anything. The
// glue is either a register or macro (\mathindent, \parindent) or a dimen with its
// unit and optional plus/minus parts (10pt plus 2pt minus 1pt).
func stripMathGlue(src, name string) (string, bool) {
	if !mathGlue[name] {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+len(needle):]
		n, ok := scanMathGlue(rest)
		if !ok {
			// Not a glue after all: leave the command alone rather than eat what
			// follows it, and let the renderer say what it makes of it.
			out.WriteString(needle)
			src = rest
			continue
		}
		src = rest[n:]
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// scanMathGlue returns how many bytes at the start of s a <glue> occupies, and
// whether one is there at all. scanMathSource writes every control sequence as a
// backslash, its name and one trailing space, which is how a register spelling is
// recognised here.
func scanMathGlue(s string) (int, bool) {
	p := 0
	for p < len(s) && s[p] == ' ' {
		p++
	}
	if p < len(s) && s[p] == '\\' { // \mathindent, \parindent, \baselineskip…
		p++
		for p < len(s) && isMathLetter(s[p]) {
			p++
		}
		if p < len(s) && s[p] == ' ' {
			p++
		}
		return p, true
	}
	q, ok := scanMathDimen(s, p)
	if !ok {
		return 0, false
	}
	p = q
	for _, word := range []string{"plus", "minus"} {
		r := p
		for r < len(s) && s[r] == ' ' {
			r++
		}
		if strings.HasPrefix(s[r:], word) {
			if q, ok := scanMathDimen(s, r+len(word)); ok {
				p = q
			}
		}
	}
	return p, true
}

// isMathLetter reports whether c is an ASCII letter, which is all a TeX control
// word or a unit is made of.
func isMathLetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// scanMathDimen returns the end of a <dimen> (a signed number and its two-letter
// unit) starting at i, and whether one is there.
func scanMathDimen(s string, i int) (int, bool) {
	p := i
	for p < len(s) && s[p] == ' ' {
		p++
	}
	for p < len(s) && (s[p] == '-' || s[p] == '+') {
		p++
	}
	digits := p
	for p < len(s) && (s[p] >= '0' && s[p] <= '9' || s[p] == '.') {
		p++
	}
	if p == digits {
		return 0, false
	}
	for p < len(s) && s[p] == ' ' {
		p++
	}
	if p+2 > len(s) || !isMathLetter(s[p]) || !isMathLetter(s[p+1]) {
		return 0, false
	}
	return p + 2, true
}

// stripMathNoise rewrites every occurrence of a non-rendering command \name in a
// math source string per mathNoise, reporting whether it changed anything. An
// occurrence whose arguments cannot be parsed is left verbatim.
func stripMathNoise(src, name string) (string, bool) {
	spec, ok := mathNoise[name]
	if !ok {
		return src, false
	}
	needle := "\\" + name + " "
	var out strings.Builder
	changed := false
	for {
		i := strings.Index(src, needle)
		if i < 0 {
			break
		}
		out.WriteString(src[:i])
		rest := src[i+len(needle):]
		if spec.mand > 0 {
			_, consumed, ok := parseMathArgs(rest, spec.mand)
			if !ok {
				out.WriteString(needle)
				src = rest
				continue
			}
			rest = rest[consumed:]
		}
		out.WriteString(spec.repl)
		src = rest
		changed = true
	}
	out.WriteString(src)
	return out.String(), changed
}

// skipMathOptArg drops a leading optional [..] argument (and any spaces before it)
// from a go-tex/math source string, e.g. the colour model in \color[rgb]{1,0,0}.
func skipMathOptArg(s string) string {
	p := 0
	for p < len(s) && s[p] == ' ' {
		p++
	}
	if p < len(s) && s[p] == '[' {
		for p < len(s) && s[p] != ']' {
			p++
		}
		if p < len(s) {
			p++ // consume ']'
		}
		return s[p:]
	}
	return s
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
	if rest == "" {
		return ""
	}
	// A control word is one or more letters (@ included, for LaTeX internals).
	j := 0
	for j < len(rest) && (rest[j] == '@' || (rest[j] >= 'a' && rest[j] <= 'z') || (rest[j] >= 'A' && rest[j] <= 'Z')) {
		j++
	}
	if j > 0 {
		return rest[:j]
	}
	// Otherwise it is a control symbol: a single non-letter (\1, \%, \#, \_ …).
	// Returning it lets the macro-retry expand a document's \newcommand{\1}{…} and
	// keys the skip tally on the real command instead of the generic "$math$".
	// Whitespace is not a command name (a malformed "\ " error), so ignore it.
	r, _ := utf8.DecodeRuneInString(rest)
	if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
		return ""
	}
	return string(r)
}

// recordMathSkip tallies a dropped equation. When the error is go-tex/math's
// "unknown command \X" it keys on that command (so the report shows which math macro
// to implement next); otherwise it keys on a generic "$math$". The tally is written
// to two maps under the SAME key: skippedCS keeps math drops in the general
// SkippedCommands surface (unchanged, for existing callers), while mathDropped is the
// dedicated view Diagnostics lifts out of the text-mode Skipped tally into its own
// MathDropped field — so a formula's silently-lost content is visible as math, not
// conflated with undefined text commands.
func (e *Engine) recordMathSkip(errMsg string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	if e.mathDropped == nil {
		e.mathDropped = map[string]int{}
	}
	key := "$math$"
	if name := unknownMathCommand(errMsg); name != "" {
		key = "\\" + name
	}
	e.skippedCS[key]++
	e.mathDropped[key]++
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
		// Do not scan past the end of the file that opened the math: an unterminated
		// \[ / \( (e.g. one reached through a class's runaway display-math patch)
		// would otherwise consume the file-end sentinel and the rest of the document.
		if isFileEndSentinel(t) {
			e.back(t)
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
		e.placeDisplay([]*boxNode{hpackSP([]node{fil, m, fil}, packTo, e.hsize)})
		return
	}
	if !e.inPar {
		e.beginParagraph(false)
	}
	e.parList = append(e.parList, m)
}
