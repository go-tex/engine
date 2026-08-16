// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// nullFont is TeX's \nullfont: a font with no characters at all. A package
// selects it around material it wants measured but not set — pgf does exactly
// that while it works out a picture's bounding box — so every character set in it
// has zero size and draws no glyph.
type nullFont struct{}

func (nullFont) charDimsSP(rune) (int, int, int) { return 0, 0, 0 }
func (nullFont) spaceSP() glueSpec               { return glueSpec{} }
func (nullFont) glyphPathAt(rune) string         { return "" }
func (nullFont) kernSP(rune, rune) int           { return 0 }
func (nullFont) sizePt() int                     { return 0 }
