// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's counter interface: the counter-defining and
// counter-mutating commands \newcounter, \setcounter, \addtocounter,
// \stepcounter and \refstepcounter. The value-reading command \value and the
// counter-formatting commands \arabic / \roman / \Roman / \alph / \Alph /
// \fnsymbol are pure kernel macros (see the counter block at the end of
// MiniLaTeXKernel in latex.go), because they are one-liners that reuse existing
// expandable machinery (\number, \romannumeral, \@Roman, \@alph, \@Alph and
// \ifcase). The mutating commands are Go primitives, matching the precedent set
// by \newcount (doNewcount) and \newtheorem (doNewtheorem): a counter is a plain
// \count register aliased as \c@<name>, so these commands are thin wrappers over
// the same register machinery (allocCnt / setCount / e.count).
//
// Reset-hook design. \newcounter{foo}[within] must zero \c@foo whenever \c@within
// steps. Rather than invent a second mechanism, this reuses addToReset/hookReset
// from theorem.go (LaTeX's own \@addtoreset / \cl@<within> scheme): addToReset
// appends "\global\count<foo>=0" to the reset-list macro \cl@within and, for a
// sectioning parent (section/subsection), hooks that parent's macro to run its
// reset list after stepping. \stepcounter{within} independently executes the same
// \cl@within list, so a counter registered within another resets whether that
// parent advances through its sectioning macro or through \stepcounter.

import "strings"

// doNewcounter implements \newcounter{name} and \newcounter{name}[within]: it
// allocates a \count register aliased as \c@<name>, defines \the<name> to print
// it as \arabic{name}, and creates an (initially empty) reset list \cl@<name>.
// With [within] the new counter is registered to reset when \c@within steps.
func (e *Engine) doNewcounter() {
	name := e.readBraceName()
	withinToks, hasWithin := e.scanOptBracketToks()
	if name == "" {
		return
	}
	ctr := "c@" + name
	code := -1
	if m := e.eq[ctr]; m != nil && m.kind == mCountRef {
		code = m.code // already allocated (\newcounter of an existing counter): reuse
	} else if e.allocCnt < 256 {
		code = e.allocCnt
		e.define(ctr, &meaning{kind: mCountRef, code: code}, true)
		e.allocCnt++
	}
	// \the<name> := \arabic{name}, the LaTeX default representation.
	body := append([]tok{csTok("arabic"), chTok('{', catBegin)}, stringToToks(name)...)
	body = append(body, chTok('}', catEnd))
	e.define("the"+name, &meaning{kind: mMacro, body: body}, true)
	// An empty reset list, so \stepcounter{name} always has one to run.
	if e.eq["cl@"+name] == nil {
		e.define("cl@"+name, &meaning{kind: mMacro}, true)
	}
	if hasWithin && code >= 0 {
		if within := strings.TrimSpace(e.toksToString(withinToks)); within != "" {
			e.addToReset(code, within)
		}
	}
}

// doSetcounter implements \setcounter{name}{value}: it sets \c@<name> to value.
// LaTeX's \setcounter is global. The value is scanned as a <number>, so
// \setcounter{x}{\value{y}} works. The argument is always consumed (so an unknown
// counter leaves no stray tokens), and applied only when the counter exists.
func (e *Engine) doSetcounter() {
	name := e.readBraceName()
	v := e.readBraceInt()
	if m := e.eq["c@"+name]; m != nil && m.kind == mCountRef {
		e.setCount(m.code, v, true)
	}
}

// doAddtocounter implements \addtocounter{name}{value}: \c@<name> += value,
// globally, mirroring \setcounter.
func (e *Engine) doAddtocounter() {
	name := e.readBraceName()
	v := e.readBraceInt()
	if m := e.eq["c@"+name]; m != nil && m.kind == mCountRef {
		e.setCount(m.code, e.count[m.code]+v, true)
	}
}

// doStepcounter implements \stepcounter{name}: advance \c@<name> by 1 (global) and
// run its reset list \cl@<name>, zeroing every counter registered within it.
func (e *Engine) doStepcounter() {
	e.stepCounter(e.readBraceName())
}

// stepCounter is the shared step-and-reset used by \stepcounter and
// \refstepcounter. The reset list is executed by pushing \cl@<name> back into the
// input, so its \global\count…=0 assignments run in the ordinary execution loop
// (as they do when a sectioning macro reaches its appended \cl@… — see hookReset).
func (e *Engine) stepCounter(name string) {
	if m := e.eq["c@"+name]; m != nil && m.kind == mCountRef {
		e.setCount(m.code, e.count[m.code]+1, true)
	}
	if cl := e.eq["cl@"+name]; cl != nil && cl.kind == mMacro {
		e.push([]tok{csTok("cl@" + name)})
	}
}

// doRefstepcounter implements \refstepcounter{name}: \stepcounter{name} followed by
// freezing the counter's representation \the<name> into \@currentlabel (fully
// expanded, like \edef), so a following \label/\ref resolves to this number —
// exactly as equation.go and theorem.go do for their counters.
func (e *Engine) doRefstepcounter() {
	name := e.readBraceName()
	e.stepCounter(name)
	if e.eq["the"+name] != nil {
		expanded := e.expandList([]tok{csTok("the" + name)})
		e.define("@currentlabel", &meaning{kind: mMacro, body: expanded}, false)
	}
}

// readBraceInt reads a {…} group whose content is scanned as a <number> and
// returns it. A missing group yields 0 (the token, if any, is pushed back).
func (e *Engine) readBraceInt() int {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return 0
	}
	v := e.scanInt()
	if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
		e.back(c)
	}
	return v
}
