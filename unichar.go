// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \DeclareUnicodeCharacter{<hex>}{<text>} (utf8.def, latex.ltx) says what a Unicode
// character in the input means. LaTeX does it by making inputenc's reader expand the
// character through \uni@<hex>; a TeX that reads the input itself (this engine, and
// XeTeX with a bundle that carries utf8.def) reaches the same result by making the
// character ACTIVE and giving it the text as its meaning. The declaration is GLOBAL
// in LaTeX (\global\expandafter\let), and so it is here.
//
// ⛔ The character must be addressable past the 256-entry catcode table: see catOf.
//
// Reference: the declaration of 2405.10478 and 2408.07843 (go-tex/engine#602 list).
// Both stay valid under a bare character (U+00A0 "no-break space" declared as a space)
// because the meaning is just the argument's tokens.
func (e *Engine) installUnicodeDeclaration() {
	e.prim("DeclareUnicodeCharacter", func(e *Engine) {
		hex := e.expandedGroup()
		body := e.rawGroup()
		code, ok := parseHexToks(hex)
		if !ok || code < 256 {
			// A code below 256 is a byte of the font encoding in LaTeX (the \DeclareInputText
			// territory), which this table already holds. Declaring one would change the
			// catcode of ASCII, so it is left to the reader.
			return
		}
		r := rune(code)
		e.setCat(r, catActive, true)
		e.define(activeName(r), &meaning{kind: mMacro, body: body}, true)
	})
}

// parseHexToks reads the hexadecimal digits of a declaration's first argument.
func parseHexToks(ts []tok) (int, bool) {
	n, digits := 0, 0
	for _, t := range ts {
		// A control sequence carries no character: it falls to the default case.
		var d int
		switch c := t.ch; {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		case t.cat == catSpace:
			continue
		default:
			return 0, false
		}
		n = n*16 + d
		digits++
		if n > 0x10FFFF {
			return 0, false
		}
	}
	return n, digits > 0
}
