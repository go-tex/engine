// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// registerPoolSize is how many registers of each kind this engine has.
//
// ⛔ IT IS TeX82's NUMBER, AND THIS ENGINE PRESENTS AS eTeX. The eTeX extensions
// real package code depends on are all implemented here — \numexpr, \dimexpr,
// \ifdefined, \ifcsname, \expanded, \unexpanded, \detokenize, \scantokens,
// \protected, \eTeXversion — and pgfkeys refuses to load without \expanded. eTeX
// carries 32768 registers of each kind and packages written against it allocate
// accordingly. Raising this is a separate change from reporting the failure
// (go-tex/engine#788), and a larger one: the backing arrays are sized from it and
// the bound is checked at every register ACCESS, not only at allocation.
//
// It exists as a name so that there is ONE place to raise, and so the report
// below can say what the ceiling was rather than repeat a literal. Measured
// 2026-11-15: loading the real tikz and calc leaves 86 of these free, and a
// package-heavy paper spends that — arXiv 2408.08199 declares fourteen
// \newtheorem counters before amsmath, mathtools, enumitem and hyperref.
const registerPoolSize = 256

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
