// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the numprint package's \numprint / \np: a formatted number.
//
//	\numprint{1234567.89}  ->  1 234 567.89        \numprint{1.5e3}  ->  1.5 ×10^3
//	\numprint[km]{5}       ->  5 km
//
// numprint groups the digits in threes (a thin space), keeps the decimal sign,
// renders an e/E exponent as ×10^{…} and prints an optional [unit] after a thin
// space. Without a handler \numprint was an undefined command whose whole argument
// was DROPPED — the number vanished from the page (143 occurrences in the corpus).
//
// The number is formatted by the same formatNumber the siunitx \num uses (see
// siunitx.go): identical grouping/exponent handling, so the two number packages
// render consistently. numprint's only addition is the optional [unit], set upright
// after a thin space.

// doNumprint implements \numprint[unit]{number} and its \np alias.
func (e *Engine) doNumprint() {
	unit, hasUnit := e.scanOptBracketToks()
	num, err := formatNumber(e.grabRawArg())
	if err != nil {
		return // empty/malformed argument: typeset nothing (still no leak)
	}
	// The formatted number, then the optional unit a thin space later. The unit's
	// own tokens are emitted verbatim (a plain "km"/"kg"/"%" sets cleanly as text)
	// rather than round-tripped through a string, so a control sequence in it
	// survives; the thin space matches \SI's number–unit gap.
	toks := tokenizeTeX(num)
	if hasUnit && len(unit) > 0 {
		toks = append(toks, csTok("thinspace"))
		toks = append(toks, unit...)
	}
	e.push(toks)
}
