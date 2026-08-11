// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements TeX's text ligatures and the quote/dash forms its fonts
// produce through their ligature programs: the f-ligatures (ff, fi, fl, ffi, ffl),
// en/em dashes (-- and ---) and curly quotes (` ' `` ''). Each substitution fires
// only when the current font actually has the combined glyph, so a font missing,
// say, U+FB01 keeps the separate f and i rather than dropping a glyph — the safe
// fallback that lets any font drive the engine.
//
// Ligatures are applied incrementally as characters join a paragraph (appendChar):
// the trailing character and the newcomer are folded into one glyph where the font
// allows, so "office" sets with a single ﬃ and "don't" closes with a ’.

// Ligature target codepoints.
const (
	ligFF  = 0xFB00 // ﬀ
	ligFI  = 0xFB01 // ﬁ
	ligFL  = 0xFB02 // ﬂ
	ligFFI = 0xFB03 // ﬃ
	ligFFL = 0xFB04 // ﬄ

	enDash = 0x2013 // –
	emDash = 0x2014 // —

	lsQuote = 0x2018 // ‘
	rsQuote = 0x2019 // ’
	ldQuote = 0x201C // “
	rdQuote = 0x201D // ”
)

// fontHas reports whether the current font can draw a glyph for r.
func (e *Engine) fontHas(r rune) bool {
	return e.curFont != nil && e.curFont.glyphPathAt(r) != ""
}

// ligature folds a trailing character prev and the newcomer cur into a single
// glyph (returning it and true) when the pair forms a TeX text ligature and the
// current font has that glyph. It is cheap for the common case: only a handful of
// prev characters can start a ligature.
func (e *Engine) ligature(prev, cur rune) (rune, bool) {
	switch prev {
	case 'f':
		switch cur {
		case 'f':
			return ligFF, e.fontHas(ligFF)
		case 'i':
			return ligFI, e.fontHas(ligFI)
		case 'l':
			return ligFL, e.fontHas(ligFL)
		}
	case ligFF:
		switch cur {
		case 'i':
			return ligFFI, e.fontHas(ligFFI)
		case 'l':
			return ligFFL, e.fontHas(ligFFL)
		}
	case '-':
		if cur == '-' {
			return enDash, e.fontHas(enDash)
		}
	case enDash:
		if cur == '-' {
			return emDash, e.fontHas(emDash)
		}
	case lsQuote:
		if cur == '`' {
			return ldQuote, e.fontHas(ldQuote)
		}
	case rsQuote:
		if cur == '\'' {
			return rdQuote, e.fontHas(rdQuote)
		}
	}
	return 0, false
}

// singleForm maps a lone ` or ' to its curly single-quote glyph (when the font has
// it); ligature() upgrades a doubled quote to the matching double afterwards. All
// other characters pass through unchanged.
func (e *Engine) singleForm(ch rune) rune {
	switch ch {
	case '`':
		if e.fontHas(lsQuote) {
			return lsQuote
		}
	case '\'':
		if e.fontHas(rsQuote) {
			return rsQuote
		}
	}
	return ch
}

// trailingChar returns the rune and index of the list's final node when that node
// is a character (so a ligature can fold into it), else ok=false.
func trailingChar(list []node) (r rune, idx int, ok bool) {
	if len(list) == 0 {
		return 0, 0, false
	}
	if c, isChar := list[len(list)-1].(charNode); isChar {
		return c.ch, len(list) - 1, true
	}
	return 0, 0, false
}
