// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// Reading a font's own quantities back: \the\fontdimen<n><font>,
// \the\hyphenchar<font> and \fontname<font>.
//
// ⛔ THE ASSIGNMENT SIDE EXISTED AND THE READING SIDE DID NOT, which is the
// shape #762 records. \fontdimen2\font=8pt has worked since #365 and
// \hyphenchar since #767; `\the` of either produced the LITERAL TOKENS. A
// document asking its own engine for a metric got "\fontdimen 2\font" where
// tectonic answers "3.33pt".
//
// ⭐ AND IT IS WHAT BLOCKED THE MEASUREMENT THAT MATTERS. #449 and #612 turn on
// WHICH FACE was selected, and \fontname is the command that says so. Without
// it the only way to compare our font choice with the reference's was to render
// to SVG and subtract glyph positions — which works, and is a poor substitute
// for asking.
//
// tex.web places them in two different machines, and they are implemented
// accordingly:
//
//	\the\fontdimen   scan_something_internal §413 -> §425 find_font_dimen(false),
//	                 which is §578: scan_int THEN scan_font_ident. Result is a
//	                 dimen.
//	\fontname        a CONVERT primitive: §468 (primitive), §470 (conv_toks),
//	                 §471 (scan_font_ident), §472 — it prints font_name[f], and
//	                 appends " at <size>pt" when font_size differs from
//	                 font_dsize.

// fontDimenOf answers \fontdimen<n> for a face: the value a document assigned
// if there is one, else the face's own.
//
// ⚠ THE NUMBERS THIS ENGINE CAN ANSWER FOR ARE 2 TO 7, and the rest report 0
// rather than a guess. TeX's parameter 1 is the slant per point, which this
// engine does not model at all; answering anything for it would be inventing a
// metric. 0 is also what TeX returns for a parameter a font does not define
// (§578 extends the array with zeros), so it is the honest answer rather than a
// placeholder.
func (e *Engine) fontDimenOf(f fontFace, n int) int {
	if f == nil {
		return 0
	}
	if v, ok := e.fontDimens[f][n]; ok {
		return v
	}
	g := f.spaceSP()
	switch n {
	case 2:
		return g.width
	case 3:
		return g.stretch
	case 4:
		return g.shrink
	case 5:
		// The x-height, by the same rule exHeightSP uses for the current face: a
		// symbol font whose 'x' has no height falls back to half the size rather
		// than reporting 0, which would collapse every ex-stated skip.
		if _, h, _ := f.charDimsSP('x'); h > 0 {
			return h
		}
		return ptToSP(f.sizePt() / 2)
	case 6:
		// The quad: one em, which for these faces is the design size.
		return ptToSP(f.sizePt())
	case 7:
		// \extraspace, the glue added after a sentence. XeTeX sets it to the
		// space shrink (xetex.web:16838-16855: extra_space := s div 3), which is
		// what the reference reports: \fontdimen4 and \fontdimen7 both read
		// 1.11pt where \fontdimen2 reads 3.33pt.
		return g.shrink
	}
	return 0
}

// doTheFontDimen handles `\the\fontdimen<n><font>`.
//
// It scans the number and then the font, in that order, because that is the
// order §578 scans them — and a reader who swaps them gets a number where a
// font should be and silently measures the wrong thing.
func (e *Engine) doTheFontDimen() string {
	n := e.scanInt()
	f, ok := e.scanFontIdent()
	if !ok {
		f = e.curFont
	}
	return formatPt(e.fontDimenOf(f, n))
}

// doTheHyphenChar handles `\the\hyphenchar<font>`, which is an integer.
func (e *Engine) doTheHyphenChar() string {
	f, ok := e.scanFontIdent()
	if !ok {
		f = e.curFont
	}
	return strconv.Itoa(e.hyphenCharOf(f))
}

// doFontName implements \fontname<font>, a convert primitive (tex.web §468-472).
//
// ⚠ WHAT IT PRINTS IS OUR OWN NAME AND NOT THE REFERENCE'S STRING. tectonic,
// being XeTeX, answers
//
//	"[lmroman10-regular]:mapping=tex-text;" at 10.0pt
//
// where the brackets and the mapping are XeTeX's own file-request syntax
// (tuenc.def:101,104) rather than anything about the font. This engine does not
// resolve faces through .fd files and has no such request string, so it prints
// the file it actually opened. The part that carries the information — WHICH
// optical size was chosen — is the same in both, and that is the question #449
// and #612 need answered.
//
// The " at <size>pt" suffix follows §472: TeX appends it only when the face's
// size differs from the font's design size, so lmroman10 at 10pt prints bare
// and at 10.95pt prints the size.
func (e *Engine) doFontName() string {
	f, ok := e.scanFontIdent()
	if !ok {
		f = e.curFont
	}
	if f == nil {
		return "nullfont"
	}
	name := faceFileName(f)
	if o, isOT := f.(*OpenTypeFont); isOT && o.designPt() != o.pt {
		name += " at " + formatPt(ptToSP(o.pt))
	}
	return name
}

// faceFileName is the name of the file a face was opened from, or a plain
// description when there is none (a host's own font, or a test mock).
func faceFileName(f fontFace) string {
	if o, ok := f.(*OpenTypeFont); ok && o.fileName != "" {
		return o.fileName
	}
	return "unknown"
}

// designPt is the size the font program was drawn for, which for the Latin
// Modern optical sizes is the number in the file's name.
func (o *OpenTypeFont) designPt() float64 {
	if o.design > 0 {
		return o.design
	}
	return o.pt
}
