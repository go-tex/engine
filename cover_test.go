// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

func TestMorePrimitives(t *testing.T) {
	cases := []struct{ src, want string }{
		// \noexpand keeps a token unexpanded inside \edef
		{`\def\b{B}\edef\a{\noexpand\b}\message{\meaning\a}`, `macro:->\b `},
		// \catcode makes @ a letter, so \a@b is one control word
		{`\catcode64=11 \def\a@b{X}\message{\a@b}`, "X"},
		// \let to a primitive and to an undefined cs
		{`\let\x=\relax\message{\meaning\x}`, `\relax`},
		{`\let\u=\undef\message{\meaning\u}`, `undefined`},
		// \let to a character token
		{`\let\lb=!\message{\meaning\lb}`, `the character !`},
		// \global escapes a group (count and def)
		{`{\global\count3=9}\message{\the\count3}`, "9"},
		{`{\gdef\a{G}}\message{\a}`, "G"},
		{`\def\a{o}{\global\edef\a{X}}\message{\a}`, "X"},
		// delimited parameters, including a braced argument that hides the delimiter
		{`\def\p#1:#2;{[#1,#2]}\message{\p x:y;}`, "[x,y]"},
		// the braced argument hides the '.' delimiter; because the whole argument is a
		// single enclosing group, TeX strips its outer braces (#1 = a.b, not {a.b}).
		{`\def\q#1.{(#1)}\message{\q {a.b}.}`, "(a.b)"},
		// \string of a single character
		{`\message{\string a}`, "a"},
		// \meaning of a macro with parameters and a control-symbol body
		{`\def\m#1{\! #1}\message{\meaning\m}`, `macro:#1->\! #1`},
		// \multiply / \advance via \global
		{`\count4=5 {\global\advance\count4 by 3}\message{\the\count4}`, "8"},
		// \romannumeral and \number
		{`\message{\romannumeral\number 2024}`, "mmxxiv"},
		// nested \expandafter chain
		{`\def\a{X}\def\b{\a}\message{\expandafter\expandafter\expandafter Q\expandafter\b\b}`, "QXX"},
		// \if compares character codes after expansion
		{`\def\a{z}\message{\if\a z eq\else ne\fi}`, " eq"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}

func TestWhiteboxHelpers(t *testing.T) {
	if !csTok("x").isCS() || chTok('a', catLetter).isCS() {
		t.Error("isCS")
	}
	if csTok("foo").String() != `\foo` || chTok('a', catLetter).String() != "a" {
		t.Error("String")
	}
	if !isWord("abc") || isWord("a1") || isWord("") {
		t.Error("isWord")
	}
	// matchLiteral: a delimiter that does not match is left in the input.
	e := New()
	if _, err := e.Run(`\def\d x{Y}\d z`); err != nil { // 'x' delimiter vs 'z' input
		t.Fatalf("matchLiteral run: %v", err)
	}
}

func TestDelimitedAndGlobal(t *testing.T) {
	cases := []struct{ src, want string }{
		// multi-token delimiter, and a braced group that hides an inner delimiter
		{`\def\a#1->#2.{[#1|#2]}\message{\a x->y.}`, "[x|y]"},
		{`\def\a#1;{(#1)}\message{\a a{;}b;}`, "(a{;}b)"},   // inner ; is protected by braces
		{`\def\a#1XY#2Z{<#1#2>}\message{\a pXYqZ}`, "<pq>"}, // partial-match backtracking on X
		{`\def\a#1XX{[#1]}\message{\a abXX}`, "[ab]"},
		// \global variants
		{`\count7=1 {\global\multiply\count7 by 6}\message{\the\count7}`, "6"},
		{`{\global\chardef\c=90}\message{\the\c}`, "90"},
		{`{\global\let\g=\relax}\message{\meaning\g}`, `\relax`},
		// \count reading a \chardef'd value and a register
		{`\chardef\n=5 \count0=\n \message{\the\count0}`, "5"},
		{`\count1=9 \count2=\count1 \message{\the\count2}`, "9"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}
