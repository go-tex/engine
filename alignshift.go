// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// amsAlignShifts is amsmath's align tag/shift calculation for the default case
// (tags on the right, not fleqn, \xatlevel@ = 1, which is align and align*). It is
// \measure@'s setup of \eqnshift@ and \alignsep@ (amsmath.sty:1934-1988) followed by
// \calc@shift@align running \x@calc@shift@rc once per tagged row, LAST ROW FIRST
// (:2111-2123, :2125-2188), because \eqnshift@ and \alignsep@ are changed with \global
// and so carry from one row to the next.
//
//	displayWidth     \displaywidth
//	colw             the widest cell of each column (\maxcol@width)
//	fields[r]        the width of each cell of row r (\fieldlengths@)
//	tagw[r]          \tag@width of row r (0 for no tag)
//	minAlignSep      \minalignsep (10pt, :1302)
//	minTagSep        \mintagsep (.5\fontdimen6\textfont2, :1301)
//
// It returns the left shift of the first pair, the gap between pairs, and for each row
// whether its tag has to go on a line of its own (\saveshift@1).
func amsAlignShifts(displayWidth int, colw []int, fields [][]int, tagw []int, minAlignSep, minTagSep int) (eqnshift, alignsep int, ownLine []bool) {
	ownLine = make([]bool, len(fields))
	maxfields := len(colw)
	if maxfields%2 == 1 { // :1910, an odd count is rounded up to complete the last pair
		maxfields++
	}
	if maxfields == 0 {
		return 0, 0, ownLine
	}
	pairs := maxfields / 2
	cntb0 := pairs - 1 // \@tempcntb, :1949-1952
	cnta0 := pairs + 1 // \@tempcnta, :1951 and :1958
	total := 0
	for _, w := range colw {
		total += w
	}
	// \divide truncates toward zero, as Go's / does.
	eqnshift = (displayWidth - total) / cnta0 // :1959-1960
	alignsep = eqnshift                       // :1961
	if alignsep < minAlignSep {               // :1975-1985
		alignsep = minAlignSep
		if eqnshift > 0 {
			eqnshift = (displayWidth - total - cntb0*alignsep) / 2
		}
	}
	if eqnshift < 0 { // :1986-1988
		eqnshift = 0
	}

	for r := len(fields) - 1; r >= 0; r-- {
		tw := tagw[r]
		if tw <= 0 { // :2116-2120, \saveshift@0
			continue
		}
		// \x@rcalc@width over the row's fields (:2191-2205).
		var dimb, dimc int
		for c, a := range fields[r] {
			mw := 0
			if c < len(colw) {
				mw = colw[c]
			}
			if a > 0 {
				dimc += dimb
				if (c+1)%2 == 1 { // \ifodd\column@
					dimc += mw
					dimb = 0
				} else {
					dimc += a
					dimb = mw - a
				}
			} else {
				dimb += mw
			}
		}
		cntb, cnta := cntb0, cnta0
		col := (len(fields[r]) - 1) / 2 // :2135-2136
		if cntb > col {                 // :2137-2141
			cnta = cnta - cntb + col
			cntb = col
		}
		tempdima := dimc + tw // :2143-2144
		// :2145-2152 with \xatlevel@ = 1: \mintagsep is added twice.
		d := minAlignSep*cntb + 2*minTagSep + tempdima
		if d > displayWidth {
			ownLine[r] = true // \saveshift@1
			continue
		}
		// :2156-2185, the inline tag: shrink \eqnshift@/\alignsep@ until it fits.
		d = eqnshift + tempdima + cntb*alignsep + tw
		if d > displayWidth {
			d = (displayWidth - tempdima) / cnta
			if d < minAlignSep {
				alignsep = minAlignSep
				eqnshift = (displayWidth - tempdima - cntb*alignsep) / 2
			} else {
				if d < eqnshift {
					if d < 0 {
						eqnshift = 0
					} else {
						eqnshift = d
					}
				}
				if d < alignsep {
					alignsep = d
				}
			}
		}
	}
	return eqnshift, alignsep, ownLine
}

// mathQuadSP is \fontdimen6\textfont2, the quad of the symbol font. In Computer Modern's
// cmsy that is the design size (1em), and this engine's math face is sized by the same
// mathSize that renders the maths, so the quad is that size in sp. The engine has no
// \textfont register model to read it from (fontchar.go consumes \textfont).
func (e *Engine) mathQuadSP() int {
	return e.mathSize() * unity
}
