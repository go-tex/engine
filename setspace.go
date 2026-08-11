// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the setspace package (line spacing): \singlespacing,
// \onehalfspacing and \doublespacing, plus \setstretch{factor} and LaTeX's
// \linespread{factor}, and the spacing environment \begin{spacing}{factor}. Each
// multiplies the single-spaced baseline skip (captured at start-up as
// baseBaselineskip) by the factor and sets \baselineskip accordingly, so subsequent
// paragraphs are set with wider (or tighter) leading. The named commands use
// factors 1 / 1.5 / 2, a documented approximation of setspace's exact values.

import "strconv"

// setLineStretch sets \baselineskip to the single-spaced reference times f.
func (e *Engine) setLineStretch(f float64) {
	if f <= 0 {
		f = 1
	}
	e.baselineskip = int(float64(e.baseBaselineskip)*f + 0.5)
}

// doSetstretch implements \setstretch{f} and \linespread{f}: read the factor and
// apply it. A malformed factor leaves the spacing unchanged.
func (e *Engine) doSetstretch() {
	if f, ok := parseFloatArg(e.readBraceName()); ok {
		e.setLineStretch(f)
	}
}

// doSpacing implements \begin{spacing}{f}: save the current \baselineskip so
// \end{spacing} can restore it, then apply the factor.
func (e *Engine) doSpacing() {
	e.spacingSaved = append(e.spacingSaved, e.baselineskip)
	if f, ok := parseFloatArg(e.readBraceName()); ok {
		e.setLineStretch(f)
	}
}

// endSpacing restores the \baselineskip saved by the matching \begin{spacing}.
func (e *Engine) endSpacing() {
	if n := len(e.spacingSaved); n > 0 {
		e.baselineskip = e.spacingSaved[n-1]
		e.spacingSaved = e.spacingSaved[:n-1]
	}
}

// parseFloatArg parses a decimal factor (e.g. "1.5"); ok is false when malformed.
func parseFloatArg(s string) (float64, bool) {
	f, err := strconv.ParseFloat(trimSpaces(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}
