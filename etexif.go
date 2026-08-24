// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file adds e-TeX's conditionals: \ifdefined, \ifcsname and \unless. They
// matter out of proportion to their size, because a package guards its own
// loading with them —
//
//	\ifdefined\pgfmathloaded\else \input{pgfmath.code.tex}\fi
//
// and when the conditional is not a conditional the guard collapses: the test is
// not merely wrong, the \else/\fi structure is mis-read and a whole component
// silently never loads. Every failure downstream then looks like a missing
// command in that component.
//
// \ifdefined and \ifcsname are the two ways to ask whether a control sequence has
// a meaning: \ifdefined takes the token (and does not expand it), \ifcsname builds
// the name from expanded text and — unlike \csname — does not create the control
// sequence as a side effect. \unless negates any conditional that follows.

// loadETeXConditionals installs them.
func (e *Engine) loadETeXConditionals() {
	e.prim("ifdefined", func(e *Engine) { e.doIf(e.evalIfdefined()) })
	e.prim("ifcsname", func(e *Engine) { e.doIf(e.evalIfcsname()) })
	e.prim("unless", func(e *Engine) { e.doUnless() })
	for _, n := range []string{"ifdefined", "ifcsname", "unless"} {
		expandableSet[n] = true
	}
	for _, n := range []string{"ifdefined", "ifcsname"} {
		etexIfPrims[n] = true
	}
}

// etexIfPrims are the conditionals added here, so the skipping machinery that
// walks over a false branch counts their \fi like any other's.
var etexIfPrims = map[string]bool{}

// evalIfdefined reads one token *without expanding it* and reports whether it has
// a meaning. A character token is always defined, as in e-TeX.
func (e *Engine) evalIfdefined() bool {
	t, ok := e.getNext()
	if !ok {
		return false
	}
	if !t.cs_ {
		return true
	}
	m := e.eq[t.cs]
	return m != nil && m.kind != mUndef
}

// evalIfcsname reads expanded text up to \endcsname and reports whether a control
// sequence of that name has a meaning — without bringing one into existence, which
// is the whole point of the primitive.
func (e *Engine) evalIfcsname() bool {
	var name []rune
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		if t.cs_ {
			if e.isPrim(t.cs, "endcsname") {
				break
			}
			name = append(name, []rune(t.cs)...)
			continue
		}
		name = append(name, t.ch)
	}
	m := e.eq[string(name)]
	return m != nil && m.kind != mUndef
}

// doUnless implements \unless<conditional>: the conditional is evaluated and its
// sense reversed. It works by running the conditional with the true and false
// branches exchanged, which is what e-TeX's \unless does.
func (e *Engine) doUnless() {
	t, ok := e.getNext()
	if !ok {
		return
	}
	m := e.meaningOf(t)
	if m == nil || m.kind != mPrim || !(isIfPrim(m.name) || etexIfPrims[m.name]) {
		e.back(t) // not a conditional: \unless does nothing to it
		return
	}
	e.negateNextIf++
	m.prim(e)
}

// loadETeXExpansion installs the expansion primitives every current TeX engine
// provides and modern package code takes for granted. pgfkeys, for one, refuses
// to load without \expanded ("PGF requires the \expanded primitive"): it uses it
// to expand a key's value once, in one step, inside a larger expansion.
func (e *Engine) loadETeXExpansion() {
	// \expanded{<text>} expands the text completely and leaves the result in the
	// input, without the assignment \edef would need.
	e.prim("expanded", func(e *Engine) { e.push(e.expandedGroup()) })
	// \unexpanded{<text>} leaves the text alone where expansion would otherwise
	// reach it: each control sequence is protected on the way back.
	e.prim("unexpanded", func(e *Engine) { e.push(protectToks(e.rawGroup())) })
	// \detokenize{<text>} turns the text into the characters that spell it.
	e.prim("detokenize", func(e *Engine) { e.pushString(e.toksToString(e.rawGroup())) })
	// \scantokens{<text>} re-reads the text as if it were a file: the characters
	// are re-tokenized under the catcodes in force now.
	e.prim("scantokens", func(e *Engine) { e.scanTokensGroup() })
	for _, n := range []string{"expanded", "unexpanded", "detokenize", "scantokens"} {
		expandableSet[n] = true
	}
	// \protected\def makes a macro that survives expansion in an \edef. The engine
	// expands macros only where a value is wanted, so the prefix is accepted and
	// the definition made as usual.
	e.prim("protected", func(e *Engine) {})
	// \eTeXversion / \eTeXrevision identify the extended engine a package tests
	// for. The version is an internal integer, so both \the\eTeXversion and
	// \ifnum\eTeXversion>0 read it.
	e.eq["eTeXversion"] = &meaning{kind: mCharDef, code: 2}
	e.prim("eTeXrevision", func(e *Engine) { e.pushString(".6") })
	expandableSet["eTeXrevision"] = true
	// \currentgrouplevel and \currentgrouptype say where the file stands in the
	// grouping stack; see etexgroup.go.
	e.installGroupQueries()
}

// rawGroup reads a braced group without expanding it. An argument that is not a
// group yields nothing, as a mis-called primitive should.
func (e *Engine) rawGroup() []tok {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return nil
	}
	if t.cs_ || t.cat != catBegin {
		e.back(t)
		return nil
	}
	return e.grabGroup()
}

// expandedGroup reads a braced group and expands it completely.
func (e *Engine) expandedGroup() []tok { return e.expandList(e.rawGroup()) }

// protectToks marks every control sequence in a list \noexpand, so the list
// survives an expansion pass unchanged.
func protectToks(ts []tok) []tok {
	out := make([]tok, 0, len(ts))
	for _, t := range ts {
		if t.cs_ {
			out = append(out, csTok("noexpand"))
		}
		out = append(out, t)
	}
	return out
}

// scanTokensGroup re-reads a group's text as input: it is written out as the
// characters that spell it and pushed back to be tokenized afresh, which is what
// \scantokens does with its pseudo-file.
func (e *Engine) scanTokensGroup() {
	e.pushString(e.toksToString(e.expandedGroup()))
}
