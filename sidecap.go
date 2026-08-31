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
	e.scanOptBracketToks() // [relwidth]
	e.scanOptBracketToks() // [pos]
	e.push([]tok{csTok(base)})
}
