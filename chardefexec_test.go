// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \chardef\x=65 makes \x behave exactly as \char65 does — in horizontal mode it
// typesets that character. The engine read such a token as a number and nothing
// else, so a \chardef token put nothing on the page; \char65 as a control tells
// the two apart.
func TestChardefTokenTypesetsItsCharacter(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"jeton \\chardef", `\chardef\aa=65 \setbox0=\hbox{\aa}`, "A"},
		{"\\char, le contrôle", `\setbox0=\hbox{\char65}`, "A"},
		{"deux jetons à la suite", `\chardef\aa=65 \chardef\bb=66 \setbox0=\hbox{\aa\bb}`, "AB"},
		{"mêlé à du texte", `\chardef\aa=65 \setbox0=\hbox{x\aa y}`, "xAy"},
		{"toujours lisible comme nombre", `\chardef\aa=65 \count3=\aa \setbox0=\hbox{\number\count3}`, "65"},
		// A \mathchardef token names a class and a family, not a glyph. Outside
		// math mode TeX refuses it rather than typesetting anything, so the
		// engine deliberately leaves it alone.
		{"jeton \\mathchardef, laissé tel quel", `\mathchardef\alphaa="010B \setbox0=\hbox{x\alphaa y}`, "xy"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := New()
			e.SetFont(spMock{})
			if _, err := e.Run(c.src); err != nil {
				t.Fatal(err)
			}
			if got := boxChars(e.box[0]); got != c.want {
				t.Errorf("%s\n  obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}
