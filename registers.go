// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// registerPoolSize is how many registers of each kind this engine has: eTeX's
// 32768 (etex.tex, \count255 … \count32767), not TeX82's 256.
//
// ⛔ IT WAS TeX82's 256 WHILE THIS ENGINE PRESENTED AS eTeX. The eTeX extensions
// real package code depends on are all implemented here — \numexpr, \dimexpr,
// \ifdefined, \ifcsname, \expanded, \unexpanded, \detokenize, \scantokens,
// \protected, \eTeXversion — and pgfkeys refuses to load without \expanded. So
// packages written against eTeX allocated accordingly and ran out: measured
// 2026-11-15, loading the real tikz and calc left 86 of 256 \count registers,
// and a package-heavy paper spends that — arXiv 2408.08199 declares fourteen
// \newtheorem counters before amsmath, mathtools, enumitem and hyperref have
// had their turn. go-tex/engine#830 made that refusal audible; this is the
// other half of go-tex/engine#788, which makes it stop happening.
//
// ⛔ AND THE OLD CEILING WAS NOT A CEILING, IT WAS A CRASH. Measured at 39da8fe,
// before this change:
//
//	\the\count300  →  panic: index out of range [300] with length 256
//	\the\dimen300  →  panic: index out of range [300] with length 256
//	\the\skip300   →  panic: index out of range [300] with length 256
//
// Three one-line documents, each a legal eTeX document, each taking the process
// down. The bound existed at eight write sites as the literal 256 — unevenly:
// \count300=5 was refused, \the\count300 was not checked at all, and
// \advance\dimen300 read the register before setDimen's own guard could refuse
// the write. Boxes had accessors and so never panicked. That asymmetry is the
// whole defect: a bound duplicated per site is a bound that is missing at some
// of them, which is why every access now goes through the accessors below and
// the number lives in exactly one place.
const registerPoolSize = 32768

// registerBaseSize is how many registers of each kind exist from the start.
//
// The pools are grown, not allocated: a dense [32768] of each kind costs about
// 1.5 MB per Engine (a glueSpec is five ints), paid by every New() whether the
// document names a high register or not, and the test suite builds thousands of
// engines. A document that stays inside TeX82's range therefore pays exactly
// what it paid before, and one that allocates past it pays for the range it
// actually reaches. An unwritten register reads as its zero value — 0, 0pt,
// zero glue, void — which is what an untouched register holds anyway, so a read
// above the current length needs no storage to answer correctly.
const registerBaseSize = 256

// growPool returns p extended far enough to index i, or false if i is outside
// the pool entirely. It doubles from registerBaseSize so a document allocating
// upward pays a logarithmic number of copies rather than one per register.
func growPool[T any](p []T, i int) ([]T, bool) {
	if i < 0 || i >= registerPoolSize {
		return p, false
	}
	if i < len(p) {
		return p, true
	}
	n := len(p)
	if n < registerBaseSize {
		n = registerBaseSize
	}
	for n <= i {
		n *= 2
	}
	if n > registerPoolSize {
		n = registerPoolSize
	}
	grown := make([]T, n)
	copy(grown, p)
	return grown, true
}

// getCount reads \count<i>. An index the pools cannot hold, and one inside the
// pool that nothing has written, both read 0.
func (e *Engine) getCount(i int) int {
	if i < 0 || i >= len(e.count) {
		e.noteBadRegisterIndex(i, `\count`)
		return 0
	}
	return e.count[i]
}

// getDimen reads \dimen<i>, in scaled points.
func (e *Engine) getDimen(i int) int {
	if i < 0 || i >= len(e.dimen) {
		e.noteBadRegisterIndex(i, `\dimen`)
		return 0
	}
	return e.dimen[i]
}

// getSkip reads \skip<i>.
func (e *Engine) getSkip(i int) glueSpec {
	if i < 0 || i >= len(e.skip) {
		e.noteBadRegisterIndex(i, `\skip`)
		return glueSpec{}
	}
	return e.skip[i]
}

// storeCount, storeDimen and storeSkip write a register with NO save-stack
// effect at all — neither scoped like an ordinary assignment nor global.
//
// ⛔ THEY ARE NOT setCount WITH global=true. The global branch calls forgetSaved,
// which drops an enclosing group's pending restore for that register; these are
// the engine installing a value of its own (a class default in texparams.go, or
// \c@footnote set and put back around one expansion in footnote.go), where the
// old code wrote through the array directly and the save stack was untouched.
// They exist so that those writes get the pool's bound and growth without
// acquiring scope semantics they never had.
func (e *Engine) storeCount(i, v int) {
	grown, ok := growPool(e.count, i)
	if !ok {
		e.noteBadRegisterIndex(i, `\count`)
		return
	}
	e.count = grown
	e.count[i] = v
}

func (e *Engine) storeDimen(i, v int) {
	grown, ok := growPool(e.dimen, i)
	if !ok {
		e.noteBadRegisterIndex(i, `\dimen`)
		return
	}
	e.dimen = grown
	e.dimen[i] = v
}

func (e *Engine) storeSkip(i int, v glueSpec) {
	grown, ok := growPool(e.skip, i)
	if !ok {
		e.noteBadRegisterIndex(i, `\skip`)
		return
	}
	e.skip = grown
	e.skip[i] = v
}

// noteBadRegisterIndex records a register index outside the pool. An index that
// is merely unwritten is not one: it is inside the pool, holds its zero value
// legitimately, and saying so would report every \the\count of a fresh register.
//
// ⛔ THIS IS A DIFFERENT FACT FROM A POOL THAT IS FULL, and it gets its own
// tally for that reason. "No room for a new \count" is this engine declining to
// allocate a name (noteRegisterExhausted, go-tex/engine#830); this is a document
// naming a register number that cannot exist, which TeX answers with "Bad
// register code" and refuses. Folding the two together would have the printed
// line blame a package for exhausting a pool that was never touched.
func (e *Engine) noteBadRegisterIndex(i int, kind string) {
	if i >= 0 && i < registerPoolSize {
		return // in the pool, merely never written: its zero value is the answer
	}
	if e.regBadIndex == nil {
		e.regBadIndex = map[string]int{}
	}
	e.regBadIndex[kind]++
}

// noteRegisterExhausted records an allocation refused because the pool of that
// kind is full.
//
// ⛔ WHAT WAS MISSING WAS NEVER THE REFUSAL, IT WAS THE SILENCE. Running out is
// a decision this engine already made and TestNewcounterExhaustion documents:
// \newcounter past the limit allocates nothing and its \arabic then reads 0.
// TeX itself stops with "No room for a new \count". Here the name simply stayed
// undefined, the document carried on, and whatever used that register read 0 or
// vanished — with nothing in the output to say a register had been refused.
//
// The engine already reports font substitutions, dropped equations, abandoned
// alignments and skipped commands this way, so this is that mechanism applied
// to the one recovery that had none.
func (e *Engine) noteRegisterExhausted(kind string) {
	if e.regExhausted == nil {
		e.regExhausted = map[string]int{}
	}
	e.regExhausted[kind]++
}
