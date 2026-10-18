// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// exHeightSP is one `ex` in the current font: its X-HEIGHT, which TeX takes from
// \fontdimen5 (tex.web §455, "The x-height for cur_font" → x_height(cur_font);
// §10675: "param[5]=x_height is the size of one ex in the font, it is also the
// height of letters for which accents don't have to be raised or lowered").
//
// ⛔⛔ This replaces half the font size, which is 16% too big. Measured against
// the reference at 10pt in Latin Modern: 1ex read 5.0pt here against 4.31pt
// there, so 3.5ex was 17.5 against 15.085 and 2.3ex was 11.5 against 9.913.
// Every dimension a class states in ex inherited that, and classes state a great
// many: article's \section alone is
//
//	\@startsection{section}{1}{\z@}{-3.5ex plus -1ex minus -.2ex}{2.3ex plus .2ex}
//
// which made every heading take 4.00pt more vertical space than the reference
// gives it.
//
// THE SOURCES, in TeX's own order of authority:
//
//   - \fontdimen5 if the document ASSIGNED it. A document that writes the
//     parameter means it, exactly as extraSpaceOf honours an assigned
//     \fontdimen7, and no measurement of the face may override it.
//   - otherwise the height of the glyph 'x' in that face. For a real font this IS
//     the x-height — that is what the name means, and what the parameter holds in
//     every TFM: cmr10's \fontdimen5 is 4.30554pt and Latin Modern's 'x' is
//     4.31pt tall at 10pt. Using the glyph keeps this to the fontFace interface
//     the engine already has (charDimsSP) rather than widening it for one number.
//   - and only with no font at all, half the size. That is the old approximation,
//     kept for the case it was always right for: there is nothing to measure.
//
// ⚠ A face whose 'x' has zero height — a symbol or dingbat font made current —
// falls back rather than returning 0, because a 0pt ex would silently collapse
// every ex-stated skip in the document to nothing.
func (e *Engine) exHeightSP() int {
	f := e.curFont
	if f == nil {
		return 5 * unity // no font: half of the 10pt default, as before
	}
	if v, ok := e.fontDimens[f][5]; ok {
		return v
	}
	if _, h, _ := f.charDimsSP('x'); h > 0 {
		return h
	}
	return f.sizePt() * unity / 2
}
