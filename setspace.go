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

// setLineStretch sets \baselineskip to the single-spaced reference times f. It also
// records that an explicit spacing command has run, so applyBaselineStretch does not
// re-apply the \baselinestretch macro over a setspace-chosen (size-adjusted) skip.
func (e *Engine) setLineStretch(f float64) {
	if f <= 0 {
		f = 1
	}
	e.baselineskip = int(float64(e.baseBaselineskip)*f + 0.5)
	e.explicitStretch = true
}

// applyBaselineStretch honors LaTeX's NATIVE line-spacing mechanism at
// \begin{document}: a document that sets its spacing with a plain
// \renewcommand{\baselinestretch}{f} (rather than setspace's \onehalfspacing /
// \setstretch / \linespread, which set the baseline skip themselves) had that
// setting ignored — real LaTeX applies \baselinestretch inside \@setfontsize, which
// the engine stubs to a no-op. 2683 corpus papers set it this way (most a stretch >1,
// so the engine was under-spacing them). Skipped when an explicit spacing command
// already ran, so setspace's size-adjusted value is not clobbered, and a value of 1
// (the default, or an explicit single-spacing) changes nothing.
func (e *Engine) applyBaselineStretch() {
	if e.explicitStretch {
		return
	}
	m := e.eq["baselinestretch"]
	if m == nil || m.kind != mMacro {
		return
	}
	f, ok := parseFloatArg(e.toksToString(m.body))
	if !ok || f <= 0 || f == 1 {
		return
	}
	e.setLineStretch(f)
}

// ptsizeCode returns the class base-size code \@ptsize — 0/1/2 for 10/11/12pt,
// defaulting to 0 — read from the macro setPtsize defines. setspace's named
// spacing commands tune their stretch to this size.
func (e *Engine) ptsizeCode() int {
	m := e.eq["@ptsize"]
	if m == nil || m.kind != mMacro {
		return 0
	}
	switch trimSpaces(e.toksToString(m.body)) {
	case "1":
		return 1
	case "2":
		return 2
	}
	return 0
}

// onehalfStretch / doubleStretch give the \baselinestretch setspace.sty selects
// for \onehalfspacing and \doublespacing. The factor is size-dependent — setspace
// tunes it per \@ptsize so the visual leading, not the raw multiple, is one-and-a-
// half / double the single-spaced body. A flat 1.5 / 2.0 overstated the leading (at
// 12pt real \onehalfspacing is 1.241, not 1.5), which inflated the page count of
// every setspace document by roughly a fifth.
func onehalfStretch(ptsize int) float64 {
	switch ptsize {
	case 1:
		return 1.213
	case 2:
		return 1.241
	}
	return 1.25
}

func doubleStretch(ptsize int) float64 {
	switch ptsize {
	case 1:
		return 1.618
	case 2:
		return 1.655
	}
	return 1.667
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

// doSetfontsize is what \@setfontsize reports to: the command being defined and
// the baseline skip it asks for. The
// engine has no NFSS and leaves the SIZE to the font system (setPtsize rescales
// the bound faces from the class's base option), but it does take the leading
// when the command being defined is \normalsize: a conference style states its
// body leading there and nowhere else, and the arguments must be consumed in any
// case or the redefinition recurses.
//
// neurips_2024.sty is the pattern: \@setfontsize\normalsize\@xpt\@xipt — 10pt on
// an 11pt skip, where the article default is 12pt. Ignoring it set 24 of the
// corpus's 157 papers 10% loose, worth 12 pages on a 26-page paper.
func (e *Engine) doSetfontsize() {
	skip := e.grabUndelimited()
	f, ok := parseFloatArg(e.toksToString(e.expandList(skip)))
	if !ok || f <= 0 {
		return
	}
	e.setEngineDimen(saveBaselineskip, &e.baselineskip, ptToSP(f), false)
}
