// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// TestConformance is a growing ratchet of TeX snippets with their reference
// (real-TeX) \message output. It is the objective gate for "is it TeX"; new
// primitives extend it, and a regression fails the build.
func TestConformance(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\uppercase{\message{abc}}`, "ABC"},
		{`\lowercase{\message{XyZ}}`, "xyz"},
		{`\message{\ifcat AB yes\else no\fi}`, " yes"}, // both letters (space significant)
		{`\message{\ifcat A1 yes\else no\fi}`, "no"},   // false: \else (a control word) absorbs the following space
		{`\def\x{Y}\message{\meaning\x}`, "macro:->Y"},
		{`\message{\meaning\relax}`, `\relax`},
		{`\message{\empty end}`, "end"},
		{`\def\twice#1{#1#1}\message{\twice{\twice A}}`, "AAAA"}, // nested macro args
		{`\uppercase{\message{hello}}`, "HELLO"},                 // \uppercase re-cases a pushed \message
		{`\count1=10 \count2=3 \multiply\count1 by \count2 \message{\the\count1}`, "30"},
		{`\message{\expandafter\string\csname foo\endcsname}`, `\foo`}, // csname+string
		// \mathchardef names an integer constant, distinct in MEANING from \chardef
		{`\mathchardef\c=500 \message{\the\c}`, "500"},
		{`\mathchardef\c=500 \message{\meaning\c}`, `\mathchar"1F4`},
		{`\mathchardef\a=7 \chardef\b=7 \message{\ifx\a\b same\else diff\fi}`, "diff"},
		// ^^ notation: two lowercase hex digits, else the next character shifted by 64
		{`\message{^^41}`, "A"},
		{`\message{^^4x}`, "tx"},
		{"\\message{\\the\\catcode`\\^^M}", "5"},
		// a parameter text ending in # is delimited by the brace, which stays put
		{`\def\a#1#{\message{[#1]}}\a xy{}`, "[xy]"},
		// \if / \ifcat resolve a control sequence \let to a character
		{`\def\g#1{}\futurelet\n\g p\message{\ifcat\n a LETTER\else OTHER\fi}`, " LETTER"},
		{`\chardef\c=65 \message{\ifcat\c a LETTER\else OTHER\fi}`, "OTHER"},
		// \endlinechar is the character appended to every input line
		{"\\begingroup\\endlinechar=-1\n\\gdef\\A{a\nb}\\endgroup\n\\message{\\meaning\\A}", "macro:->ab"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}
