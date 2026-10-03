// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// ε-TeX's glyph-metric primitives, and XeTeX's link box.
//
//	\fontcharht<font><charcode>   the height of that character in that font
//	\fontcharwd / \fontchardp     its width / depth
//	\fontcharic                   its italic correction
//	\XeTeXLinkBox{<material>}     a hyperlink wrapper around <material>
//
// ⛔ Undefined, each was SKIPPED — and skipping releases what follows, so the harm is not a
// missing value but a mangled stream. lmcs.cls:817 sizes an ORCID logo from the height of a
// capital X,
//
//	\setlength{\@lmcscurXheight}{\fontcharht\font`X}%
//
// and it runs from \author, so 2603.18955 lost that \setlength's argument and leaked a group
// there. Measured on 999 papers: \fontcharht in 14 papers, \XeTeXLinkBox in 11.
//
// ⭐ The heights are EXACT rather than approximated: fontFace already exposes
// charDimsSP(rune), the same measurement that sets every character on the page, so
// \fontcharht\font`X answers with the X the document will actually show. The engine's \ex
// unit is a design-size fraction; this is not.
func (e *Engine) initFontCharPrims() {
	for _, name := range fontCharDimNames {
		which := name
		// Executed directly — \the\fontcharht\font`X, or a bare use — it contributes its
		// value as text. Read where a <dimen> is wanted it goes through scanDimenValue
		// instead, which is what \setlength needs and what registering it in
		// isInternalDimen arranges.
		e.prim(which, func(e *Engine) { e.pushString(formatPt(e.fontCharDim(which))) })
	}
	// \XeTeXLinkBox{<material>} wraps material in a hyperlink. This engine writes no PDF
	// link annotations, so the faithful reduction is the material itself — which is what
	// skipping happened to leave behind too. What changes is that the braces are now
	// CONSUMED as an argument instead of being left to the surrounding group.
	e.prim("XeTeXLinkBox", func(e *Engine) {
		e.skipOptSpace()
		t, ok := e.getNext()
		if !ok {
			return
		}
		if !(t.cat == catBegin && !t.cs_) {
			e.back(t)
			return
		}
		e.push(e.scanBody())
	})
}

// fontCharDimNames are the glyph-metric primitives, in one place so the scanner and the
// registration cannot disagree about which names are internal dimensions.
var fontCharDimNames = []string{"fontcharht", "fontcharwd", "fontchardp", "fontcharic"}

// fontCharDim reads <font><charcode> and answers the metric the primitive names.
//
// ⭐ EXACT, not approximated: fontFace.charDimsSP is the same measurement that sets every
// character on the page, so \fontcharht\font`X answers with the X the document will show.
// The engine's \ex unit is a design-size fraction; this is not.
//
// \fontcharic is zero because this engine's faces carry no italic correction — and zero is
// also what \/ contributes here, so the two agree rather than one of them lying.
func (e *Engine) fontCharDim(which string) int {
	e.scanFontSpec()
	r := rune(e.scanInt())
	if e.curFont == nil || r <= 0 || which == "fontcharic" {
		return 0
	}
	w, h, d := e.curFont.charDimsSP(r)
	switch which {
	case "fontcharwd":
		return w
	case "fontchardp":
		return d
	}
	return h
}

// scanFontSpec consumes the <font> of a glyph-metric primitive. Only \font — the current
// font — is honoured, which is what every corpus use writes (\fontcharht\font`X); a
// \textfont/\scriptfont register or a font identifier needs a font-register model this
// engine does not have, and is CONSUMED rather than left to leak onto the page.
func (e *Engine) scanFontSpec() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if t.cs_ {
		return // \font, \textfont…: consumed either way
	}
	e.back(t)
}

// isFontCharDim reports whether a primitive name is one of the glyph-metric dimensions.
func isFontCharDim(name string) bool {
	for _, n := range fontCharDimNames {
		if name == n {
			return true
		}
	}
	return false
}
