// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// This file implements LaTeX's length interface on top of the engine's TeX
// register machinery. In LaTeX a "length" is a rubber length: a \skip register
// that may carry stretch and shrink components. \newlength therefore allocates a
// \skip register (exactly as \newskip would) and binds the given control
// sequence to it, so the name reads as a glue (\the\cmd, \hskip\cmd) and keeps
// its stretch/shrink under \setlength.
//
//   \newlength{\cmd}              allocate a skip register, alias \cmd to it
//   \setlength{\cmd}{glue}        \cmd := glue        (rubber; group-local)
//   \addtolength{\cmd}{glue}      \cmd := \cmd + glue (group-local)
//   \settowidth{\cmd}{content}    \cmd := natural width  of content as an hbox
//   \settoheight{\cmd}{content}   \cmd := natural height of content as an hbox
//   \settodepth{\cmd}{content}    \cmd := natural depth  of content as an hbox
//
// \stretch{n} (a rubber length 0pt plus n fil) is provided as a kernel macro in
// latex.go, so it expands wherever a glue is scanned.
//
// The target of \setlength/\addtolength/\settoX is resolved by control-sequence
// meaning, so besides a \newlength length it also accepts a \newdimen register
// (mDimenRef, width only) and the engine's own dimension/glue parameters
// (\hsize, \vsize, \parindent, \baselineskip, \leftskip, \rightskip). Writes to
// \leftskip/\rightskip are group-scoped through the save stack, matching those
// primitives; writes to \hsize/\vsize/\parindent/\baselineskip set the engine
// field directly (not group-local), consistent with the engine's own \hsize=,
// \parindent= etc. primitives, which are likewise not save-stacked.

// doNewlength implements \newlength{\cmd}: it allocates the next free \skip
// register and aliases \cmd to it (a skip-ref, like \newskip). Re-\newlength of
// an existing name silently rebinds it to a fresh register (LaTeX raises an
// error; we choose the lenient behaviour, which merely consumes a register).
func (e *Engine) doNewlength() {
	target, ok := e.readLengthCS()
	if !ok {
		e.fail("Missing control sequence for \\newlength")
		return
	}
	if e.allocSkp >= 256 {
		e.fail("No room for a new \\skip (\\newlength)")
		return
	}
	e.define(target.cs, &meaning{kind: mSkipRef, code: e.allocSkp}, false)
	e.allocSkp++
}

// recordSkippedLength tallies a length assignment dropped in lenient mode (its
// register was never declared), so the skip census reports it.
func (e *Engine) recordSkippedLength(name string) {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS[name]++
}

// doSetlength implements \setlength (add=false) and \addtolength (add=true):
// \setlength{\cmd}{glue}. The value is a full glue, so rubber lengths keep their
// plus/minus components. Both are group-local (a plain assignment).
func (e *Engine) doSetlength(add bool) {
	target, ok := e.readLengthCS()
	g := e.readBraceGlue() // read the value regardless, to stay token-synchronised
	if !ok {
		// A real document sets a length the engine has no register for (a package's
		// own \newlength that was gobbled, etc.). In lenient mode drop the assignment
		// rather than abort the whole compile; the value is already consumed.
		if e.tolerant() {
			e.recordSkippedLength("setlength")
			return
		}
		e.fail("Missing length register for \\setlength")
		return
	}
	e.assignLength(target, g, add, false)
}

// doSettodim implements \settowidth/\settoheight/\settodepth: it typesets the
// {content} argument into an hbox (in its own group, so nothing leaks to the
// page) and assigns the requested natural dimension to \cmd. which is one of
// 'w', 'h' or 'd'.
func (e *Engine) doSettodim(which byte) {
	target, ok := e.readLengthCS()
	list, _ := e.grabHboxList() // sandboxed group; consumed even on a bad target
	b := hpackSP(list, packNatural, 0)
	if !ok {
		if e.tolerant() {
			e.recordSkippedLength("settowidth")
			return
		}
		e.fail("Missing length register for \\settowidth")
		return
	}
	var d int
	switch which {
	case 'h':
		d = b.height
	case 'd':
		d = b.depth
	default: // 'w'
		d = b.width
	}
	e.assignLength(target, glueSpec{width: d}, false, false)
}

// assignLength writes glue g to the length identified by control sequence target
// (or advances it by g when add is true). It routes by the target's meaning: a
// \newlength length (skip register), a \newdimen register (width only), or one
// of the engine's dimension/glue parameters. An unknown or non-length target is
// an error (no panic).
func (e *Engine) assignLength(target tok, g glueSpec, add, global bool) {
	m := e.eq[target.cs]
	if m == nil {
		if e.tolerant() {
			return // best-effort: ignore a \setlength on a length we don't model
		}
		e.fail("Undefined length \\" + target.cs)
		return
	}
	switch {
	case m.kind == mSkipRef:
		if add {
			e.setSkip(m.code, addGlue(e.skip[m.code], g), global)
		} else {
			e.setSkip(m.code, g, global)
		}
	case m.kind == mDimenRef:
		e.setDimen(m.code, addOrSet(add, e.dimen[m.code], g.width), global)
	// The engine's dimension parameters go through setEngineDimen so \setlength is
	// scoped to its group exactly as \hsize=… is: \setlength\textwidth{…} inside a
	// box must not resize every page that follows.
	case m.kind == mPrim && m.name == "hsize":
		e.setEngineDimen(saveHsize, &e.hsize, addOrSet(add, e.hsize, g.width), global)
	case m.kind == mPrim && m.name == "vsize":
		e.setEngineDimen(saveVsize, &e.vsize, addOrSet(add, e.vsize, g.width), global)
	case m.kind == mPrim && m.name == "parindent":
		e.setEngineDimen(saveParindent, &e.parindent, addOrSet(add, e.parindent, g.width), global)
	case m.kind == mPrim && m.name == "baselineskip":
		e.setEngineDimen(saveBaselineskip, &e.baselineskip, addOrSet(add, e.baselineskip, g.width), global)
	case m.kind == mPrim && m.name == "leftskip":
		if len(e.groups) > 0 {
			e.save = append(e.save, saveItem{kind: 6, oldg: e.leftskip})
		}
		if add {
			e.leftskip = addGlue(e.leftskip, g)
		} else {
			e.leftskip = g
		}
	case m.kind == mPrim && m.name == "rightskip":
		if len(e.groups) > 0 {
			e.save = append(e.save, saveItem{kind: 7, oldg: e.rightskip})
		}
		if add {
			e.rightskip = addGlue(e.rightskip, g)
		} else {
			e.rightskip = g
		}
	default:
		if e.tolerant() {
			return // best-effort: ignore a length op on a non-length target
		}
		e.fail("Not a length \\" + target.cs)
	}
}

// addOrSet returns cur+delta when add is true, otherwise delta.
func addOrSet(add bool, cur, delta int) int {
	if add {
		return cur + delta
	}
	return delta
}

// readLengthCS reads a length target: a control sequence, optionally wrapped in
// braces ({\cmd}, LaTeX's usual spelling, or a bare \cmd). It reads without
// expansion so the length's own name is captured, not its value. ok is false
// when no control sequence is found (the consumed tokens are not restored).
func (e *Engine) readLengthCS() (tok, bool) {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok {
		return tok{}, false
	}
	if t.cat == catBegin && !t.cs_ {
		inner, ok := e.getNext()
		if !ok || !inner.cs_ {
			e.skipToGroupEnd()
			return tok{}, false
		}
		c, ok := e.getNext()
		if !ok {
			return inner, true
		}
		if c.cat == catEnd && !c.cs_ {
			return inner, true
		}
		// More than a single control sequence in the braces: the target is a
		// register reference such as {\skip\footins} — a class sets an insertion's
		// skip that way. Resolve it to the register it names and hand back a
		// handle for it; anything else is consumed whole so the value that follows
		// is still read and nothing leaks onto the page.
		e.back(c)
		if ref, ok := e.registerHandle(inner); ok {
			e.skipToGroupEnd()
			return ref, true
		}
		e.skipToGroupEnd()
		return tok{}, false
	}
	if t.cs_ {
		return t, true
	}
	e.back(t)
	return tok{}, false
}

// registerHandle turns a register primitive plus its number (\skip\footins,
// \dimen4) into a control sequence that stands for that register, so an
// assignment can be made to it through the same path as a named length.
func (e *Engine) registerHandle(prim tok) (tok, bool) {
	m := e.eq[prim.cs]
	if m == nil || m.kind != mPrim {
		return tok{}, false
	}
	var kind mkind
	switch m.name {
	case "skip":
		kind = mSkipRef
	case "dimen":
		kind = mDimenRef
	default:
		return tok{}, false
	}
	i := e.scanInt()
	if i < 0 || i >= 256 {
		return tok{}, false
	}
	name := "gotex@reg@" + prim.cs + strconv.Itoa(i)
	e.define(name, &meaning{kind: kind, code: i}, true)
	return csTok(name), true
}

// skipToGroupEnd consumes tokens up to and including the matching end of the
// group already opened, so a target this engine cannot assign to still leaves the
// input where the caller expects it.
func (e *Engine) skipToGroupEnd() {
	depth := 1
	for {
		t, ok := e.getNext()
		if !ok {
			return
		}
		if t.cs_ {
			continue
		}
		switch t.cat {
		case catBegin:
			depth++
		case catEnd:
			if depth--; depth == 0 {
				return
			}
		}
	}
}

// readBraceGlue reads a {glue} group (the value argument of \setlength) and
// returns the glue, accepting rubber components (plus/minus). It mirrors
// readBraceDimen but scans a full glue instead of a rigid dimen.
func (e *Engine) readBraceGlue() glueSpec {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return glueSpec{}
	}
	g := e.scanGlue()
	if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
		e.back(c)
	}
	return g
}
