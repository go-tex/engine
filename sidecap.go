// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the sidecap package's SCfigure/SCtable environments, which
// set a figure or table with its \caption BESIDE the body rather than below it:
//
//	\begin{SCfigure}[relwidth][pos] … \includegraphics … \caption{…} … \end{SCfigure}
//
// (SCfigure* and SCtable* are the two-column-spanning forms.) The engine has no
// side-caption layout, so — exactly as figure*/table* become plain figure/table —
// these become the ordinary float, with the caption set below. Without a handler
// the environment was undefined and its two leading optional arguments leaked onto
// the page ("[relwidth][pos]") while \caption mis-numbered.
//
// The two optionals are consumed HERE, in Go, rather than in a TeX definition:
// sidecap's \begin{SCfigure}[relwidth][pos] carries TWO optionals, and a leading
// \@discardopt in a macro body cannot eat them (it scans and finds the macro's own
// next token, not the bracket, and two in a row see each other). Reading them
// directly from the input sidesteps that entirely; \figure/\table then run their
// normal setup (their own trailing \@discardopt finds nothing left).

// doSCfloat consumes SCfigure/SCtable's [relwidth] and [pos] optional arguments and
// hands off to the plain float macro base ("figure" or "table").
func (e *Engine) doSCfloat(base string) {
	// ⛔ NO-EXPAND, and the reason is visible on the page. The expanding scan
	// expands the next token to decide whether it is a "[", so with no optional
	// present it EXPANDED THE BODY'S FIRST TOKEN before \figure had run — before
	// \def\@captype{figure}. Witness, base engine:
	//
	//	\begin{SCfigure}\@ifundefined{@captype}{UNDEF}{DEF}\end{SCfigure}   UNDEF
	//	\begin{figure}\@ifundefined{@captype}{UNDEF}{DEF}\end{figure}       DEF
	//	\begin{SCfigure}\relax\@ifundefined{@captype}{UNDEF}{DEF}\end{...}  DEF
	//
	// One token of distance was enough to change the answer. Captions survived it
	// only by accident: \caption expands one step, the scan sees \par instead of
	// "[", pushes the expansion back, and the tokens then EXECUTE after \figure
	// with \@captype in place. Anything that decides something DURING that
	// expansion reads the wrong state — which is what blocks the guard in #558.
	e.scanOptBracketToksNoExpand() // [relwidth]
	e.scanOptBracketToksNoExpand() // [pos]
	e.push([]tok{csTok(base)})
}
