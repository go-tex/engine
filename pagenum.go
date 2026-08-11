// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements page styles and page numbering: \pagestyle{plain|empty}
// (and \thispagestyle, treated the same), \pagenumbering{arabic|roman|Roman|alph|
// Alph} which selects the page-number format, and \today (the document date, taken
// from Options.Date since a pure-Go wasm build has no ambient clock). When the page
// style is not "empty", the page builder draws a centred page number at the foot of
// every page (see assemblePage in footnote.go).
//
// LaTeX's article class defaults to the "plain" style (a bottom-centred number). The
// engine defaults to "empty" so existing single-pass renders are unchanged; a
// document opts in with \pagestyle{plain}. \thepage inside running text expands to
// the arabic page counter, which this single-pass engine cannot know in advance, so
// it is best-effort (1) — the foot number, computed per page at break time, is
// always correct. Both are honest, documented limitations.

import (
	"strconv"
	"strings"
)

// doPagestyle implements \pagestyle{name} and \thispagestyle{name}: only the number
// vs no-number distinction is modelled (plain/headings/myheadings all show the
// bottom-centred number; empty shows none).
func (e *Engine) doPagestyle() {
	name := e.readBraceName()
	if name == "empty" {
		e.pageStyle = "empty"
	} else {
		e.pageStyle = "plain"
	}
}

// doPagenumbering implements \pagenumbering{style}: select the page-number format.
// It also resets the page counter in LaTeX; here the foot number is a running
// ordinal, so only the format is recorded (documented simplification).
func (e *Engine) doPagenumbering() {
	switch e.readBraceName() {
	case "roman":
		e.pageNumStyle = 'r'
	case "Roman":
		e.pageNumStyle = 'R'
	case "alph":
		e.pageNumStyle = 'l'
	case "Alph":
		e.pageNumStyle = 'L'
	default: // "arabic" and anything else
		e.pageNumStyle = 'a'
	}
}

// formatPageNumber renders a 1-based page ordinal in the current page-number style.
func formatPageNumber(n int, style byte) string {
	if n < 1 {
		n = 1
	}
	switch style {
	case 'r':
		return roman(n)
	case 'R':
		return strings.ToUpper(roman(n))
	case 'l':
		return alphaLabel(n, 'a')
	case 'L':
		return alphaLabel(n, 'A')
	default:
		return strconv.Itoa(n)
	}
}

// alphaLabel renders n (>=1) as a, b, …, z, aa, ab, … in the alphabet based at
// base ('a' or 'A'), the LaTeX \alph/\Alph convention (bijective base-26).
func alphaLabel(n int, base byte) string {
	var b []byte
	for n > 0 {
		n--
		b = append([]byte{base + byte(n%26)}, b...)
		n /= 26
	}
	return string(b)
}

// pageFooter builds the centred page-number line placed at the foot of a page.
func (e *Engine) pageFooter(num int) *boxNode {
	numBox := e.textToHbox(formatPageNumber(num, e.pageNumStyle))
	return hpackSP([]node{e.hfil(), numBox, e.hfil()}, packTo, e.hsize)
}
