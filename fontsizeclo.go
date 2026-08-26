// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"
)

// sizeCloCommands are the font-size switches a size<NN>.clo (re)defines through
// \@setfontsize. In this engine \@setfontsize gobbles, so the clo leaves each one
// a no-op: a heading, caption or footnote set with \Large/\small/… came out at the
// body size with the body leading, losing the vertical space real LaTeX gives it.
// rewireSizeCommands turns each back into a working font+leading switch.
//
// \normalsize is deliberately NOT rewired. #75 already installs the class base size
// and its \baselineskip, and \normalsize is called on nearly every size reset — so
// hardcoding a \baselineskip into it would overwrite a document's own line spacing
// (\linespread, setspace) every time and densify the whole document. Left as the
// dead \@setfontsize gobble, \normalsize keeps executing its display-skip tail and
// leaves #75's base leading in force, which is what body text wants.
var sizeCloCommands = []string{
	"tiny", "scriptsize", "footnotesize", "small",
	"large", "Large", "LARGE", "huge", "Huge",
}

// rewireSizeCommands revives the size clo's \tiny…\Huge (\normalsize is left to
// #75, see the sizeCloCommands note). It runs
// once at \begin{document} (\AtBeginDocument, see amssubstrate.go), after the
// class, the size clo and any preamble redefinition have settled.
//
// For each command it reads the (size, leading) the ACTIVE clo baked into the
// command's body — \@setfontsize\<cmd>\<size>{<leading>} — so an [11pt]/[12pt] class
// gets its own larger clo table, not a hardcoded 10pt one. It then redefines the
// command as a \protected macro
//
//	\gotexsize<permille>\relax\gotexleading<leading>pt\relax
//
// where <permille> scales the class base font (the mechanism \large already used,
// see doFontSize) to the clo's absolute point size, and \gotexleading sets the
// matching \baselineskip (doLeading). Because the base font is itself the class
// base size (#75), scaling by permille = round(size*1000/base) lands exactly on the
// clo's size at every base.
//
// Making the macro \protected is what keeps it robust in a moving context — a
// section title flowing into the ToC, a caption into the LoF: like a real
// \DeclareRobustCommand it stays one token through an \edef instead of executing
// its font primitives mid-scan. That is the failure mode #75 avoided by refusing to
// make \@setfontsize itself functional; the same reasoning drives the leading
// through the font system rather than through the clo's tokens.
//
// A command whose body is not a clo \@setfontsize switch (no clo loaded, or a class
// that sizes another way) is left untouched.
func (e *Engine) rewireSizeCommands() {
	base := e.baseFontPx
	if base <= 0 {
		base = 10
	}
	for _, name := range sizeCloCommands {
		m := e.eq[name]
		if m == nil || m.kind != mMacro {
			continue
		}
		sizePt, leadPt, end, ok := e.parseSetfontsize(m.body)
		if !ok {
			continue
		}
		px := int(sizePt + 0.5)
		if px < 1 {
			px = 1
		}
		permille := (px*1000 + base/2) / base
		// Build the body as real tokens: \gotexsize<permille>\relax scales the base
		// font and \gotexleading<leading>sp\relax sets \baselineskip. The leading is
		// emitted in scaled points (an exact integer) so no decimal formatting can
		// drift it. stringToToks is deliberately NOT used — it renders a control word
		// as its literal characters, not a control sequence.
		body := []tok{csTok("gotexsize")}
		body = append(body, digitToks(permille)...)
		body = append(body, csTok("relax"), csTok("gotexleading"))
		body = append(body, digitToks(ptToSP(leadPt))...)
		body = append(body, chTok('s', catLetter), chTok('p', catLetter), csTok("relax"))
		// Keep everything the clo put AFTER \@setfontsize — the \abovedisplayskip /
		// \belowdisplayskip glue and the \@listi list parameters that \small and
		// \footnotesize carry. The dead \@setfontsize gobble left this tail running,
		// so dropping it would silently change the spacing around display math and
		// lists across the whole document.
		body = append(body, m.body[end:]...)
		e.define(name, &meaning{kind: mMacro, body: body, protected: true}, true)
	}
}

// parseSetfontsize reads a size clo's \@setfontsize\<cmd>\<size>{<leading>} switch
// from the front of a macro body and returns the size and leading in points and the
// index just past the switch (where the clo's display-skip / list tail begins). The
// first token must be \@setfontsize; the three arguments after it are read the way
// TeX reads undelimited arguments (a braced group or a single token), the first —
// the command itself — discarded. A body not beginning with \@setfontsize is not a
// clo size switch: ok is false.
func (e *Engine) parseSetfontsize(body []tok) (sizePt, leadPt float64, end int, ok bool) {
	i := skipTokSpace(body, 0)
	if i >= len(body) || !body[i].cs_ || body[i].cs != "@setfontsize" {
		return 0, 0, 0, false
	}
	i++
	_, i = grabTokArg(body, i)   // arg1: the command itself, discarded
	a2, i := grabTokArg(body, i) // arg2: the size (a \@NNpt macro or a number)
	a3, i := grabTokArg(body, i) // arg3: the leading ({14}, {9.5} or a \@NNpt macro)
	sizePt, ok1 := e.evalNumToks(a2)
	leadPt, ok2 := e.evalNumToks(a3)
	return sizePt, leadPt, i, ok1 && ok2
}

// skipTokSpace advances past space tokens in a stored token slice.
func skipTokSpace(body []tok, i int) int {
	for i < len(body) && !body[i].cs_ && body[i].cat == catSpace {
		i++
	}
	return i
}

// grabTokArg reads one undelimited argument from a stored token slice starting at
// i, mirroring TeX: leading spaces are skipped, a { … } yields the balanced inner
// tokens, anything else yields the single next token. It returns the argument and
// the index just past it.
func grabTokArg(body []tok, i int) ([]tok, int) {
	i = skipTokSpace(body, i)
	if i >= len(body) {
		return nil, i
	}
	if !body[i].cs_ && body[i].cat == catBegin {
		depth := 1
		i++
		start := i
		for i < len(body) {
			if !body[i].cs_ {
				if body[i].cat == catBegin {
					depth++
				} else if body[i].cat == catEnd {
					if depth--; depth == 0 {
						return body[start:i], i + 1
					}
				}
			}
			i++
		}
		return body[start:i], i // unbalanced: take what there is
	}
	return body[i : i+1], i + 1
}

// evalNumToks reads a decimal number from tokens that are either a run of digit and
// '.' characters ({14}, {9.5}) or a single font-size macro that expands to such a
// run (\@xivpt → 14). It follows one level of macro indirection, which is all a
// size clo uses.
func (e *Engine) evalNumToks(ts []tok) (float64, bool) {
	if len(ts) == 1 && ts[0].cs_ {
		m := e.eq[ts[0].cs]
		if m == nil || m.kind != mMacro {
			return 0, false
		}
		return e.evalNumToks(m.body)
	}
	var b strings.Builder
	for _, t := range ts {
		switch {
		case t.cs_:
			return 0, false
		case (t.ch >= '0' && t.ch <= '9') || t.ch == '.':
			b.WriteRune(t.ch)
		case t.cat == catSpace:
			// skip
		default:
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(b.String(), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
