// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// The ifthen package. \ifthenelse{TEST}{THEN}{ELSE} was undefined, and an
// undefined command in the DOCUMENT keeps its arguments (skipUndefined), so all
// three groups were typeset: the source of the test, and BOTH branches. Measured
// against tectonic on the three corpus forms:
//
//	reference   Avant. ALPHA GAMMA EPSILON Apres.
//	ours        Avant. xxALPHABETA > 2GAMMADELTA EPSILONZETA Apres.
//
// 226 occurrences over 7 of the 200 corpus papers.
//
// Only the tests the corpus actually writes are evaluated — \equal, \boolean, and
// a numeric comparison of \value against a number. \isodd, \lengthtest and the
// \AND/\OR/\NOT combinators are NOT here: implementing a grammar nothing exercises
// buys untested paths, and the census names any test that turns up (see
// evalIfthenTest's default). The same reasoning as \discretionary in #384.
func (e *Engine) loadIfthen() {
	e.prim("ifthenelse", func(e *Engine) {
		test, yes, no := e.grabUndelimited(), e.grabUndelimited(), e.grabUndelimited()
		if e.evalIfthenTest(test) {
			e.push(yes)
		} else {
			e.push(no)
		}
	})
	// \equal and \boolean are only ever READ by evalIfthenTest. Outside a test they
	// are meaningless in LaTeX too, so they stay undefined rather than becoming
	// commands that typeset something.
}

// splitGroup peels a leading {...} off a token list, returning its contents and
// what follows. Nesting is tracked, so {a{b}c} comes back whole.
func splitGroup(ts []tok) (inner, rest []tok, ok bool) {
	i := 0
	for i < len(ts) && !ts[i].cs_ && ts[i].cat == catSpace {
		i++
	}
	if i >= len(ts) || ts[i].cs_ || ts[i].cat != catBegin {
		return nil, ts, false
	}
	depth := 0
	for j := i; j < len(ts); j++ {
		if !ts[j].cs_ && ts[j].cat == catBegin {
			depth++
		} else if !ts[j].cs_ && ts[j].cat == catEnd {
			depth--
			if depth == 0 {
				return ts[i+1 : j], ts[j+1:], true
			}
		}
	}
	return nil, ts, false
}

// leadingCS returns the first token of a test when it is a control sequence.
func leadingCS(ts []tok) (string, []tok) {
	i := 0
	for i < len(ts) && !ts[i].cs_ && ts[i].cat == catSpace {
		i++
	}
	if i < len(ts) && ts[i].cs_ {
		return ts[i].cs, ts[i+1:]
	}
	return "", ts
}

// evalIfthenTest evaluates one ifthen test. An unrecognised test reports itself
// through the skipped-command census under a name that says what happened, and
// takes the THEN branch: the alternative is to guess silently, and a wrong guess
// that drops a branch looks exactly like a correct one.
func (e *Engine) evalIfthenTest(ts []tok) bool {
	switch name, rest := leadingCS(ts); name {
	case "equal":
		a, rest2, ok1 := splitGroup(rest)
		b, _, ok2 := splitGroup(rest2)
		if !ok1 || !ok2 {
			return e.ifthenUnknown("equal")
		}
		return e.expandToString(a) == e.expandToString(b)
	case "boolean":
		n, _, ok := splitGroup(rest)
		if !ok {
			return e.ifthenUnknown("boolean")
		}
		// \newboolean{b} is \newif\ifb, which \let's \ifb to \iftrue or \iffalse
		// (kernelhelpers.go). The switch's MEANING is therefore the value.
		m := e.eq["if"+e.expandToString(n)]
		return m != nil && m.kind == mPrim && m.name == "iftrue"
	default:
		// Everything else is read as a numeric comparison: the left side is
		// usually \value{counter}, so the test OPENS with a control sequence and
		// cannot be recognised by its first token. evalIfthenNumeric reports the
		// test itself if it does not parse as one.
		return e.evalIfthenNumeric(ts)
	}
}

// expandToString expands a token list and renders it as the characters it stands
// for — what \ifthenelse compares, since \equal is a comparison of the EXPANDED
// texts (ifthen.sty uses \edef on both sides).
func (e *Engine) expandToString(ts []tok) string {
	return strings.TrimSpace(e.toksToString(e.expandList(ts)))
}

// evalIfthenNumeric evaluates "<number> <rel> <number>", where either side may be
// \value{counter}. The engine's own scanner reads the numbers, so \value, a
// register, arithmetic and \the all work exactly as they do elsewhere.
func (e *Engine) evalIfthenNumeric(ts []tok) bool {
	// The test list is the ONLY input while it is read, and the base string is
	// fenced off (noBase) — the same shape as evalDimenTokens (image.go), so a
	// malformed test cannot run on into the document.
	savedLists, savedNoBase := e.lists, e.noBase
	e.lists = [][]tok{append([]tok(nil), ts...)}
	e.noBase = true
	a := e.scanInt()
	e.skipOptSpace()
	rel, ok := e.getNext()
	var b int
	if ok && !rel.cs_ {
		b = e.scanInt()
	}
	e.lists, e.noBase = savedLists, savedNoBase
	if !ok || rel.cs_ {
		return e.ifthenUnknown(e.toksToString(ts))
	}
	switch rel.ch {
	case '<':
		return a < b
	case '>':
		return a > b
	case '=':
		return a == b
	}
	return e.ifthenUnknown(e.toksToString(ts))
}

// ifthenUnknown records a test shape this engine does not evaluate and takes the
// THEN branch.
func (e *Engine) ifthenUnknown(what string) bool {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS[`\ifthenelse test not evaluated: `+what]++
	return true
}
