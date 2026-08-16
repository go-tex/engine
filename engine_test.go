// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

func run(t *testing.T, src string) string {
	t.Helper()
	out, err := New().Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

// TestGullet exercises TeX-faithful expansion via \message (which prints the
// fully-expanded token list).
func TestGullet(t *testing.T) {
	cases := []struct{ src, want string }{
		// basic macro expansion
		{`\def\a{X}\message{\a\a}`, "XX"},
		// undelimited multi-parameter macro
		{`\def\add#1#2{#1+#2}\message{\add xy}`, "x+y"},
		// delimited parameters
		{`\def\pair#1,#2.{(#1|#2)}\message{\pair a,b.}`, "(a|b)"},
		// grouped argument
		{`\def\id#1{[#1]}\message{\id{ab}}`, "[ab]"},
		// \expandafter reorders expansion
		{`\def\a{A}\def\b{B}\message{\expandafter\a\b}`, "AB"},
		// \csname builds a control sequence
		{`\def\xy{Z}\message{\csname xy\endcsname}`, "Z"},
		// integer registers and arithmetic
		{`\count5=40 \advance\count5 by 2 \message{\the\count5}`, "42"},
		{`\count0=6 \multiply\count0 by 7 \message{\the\count0}`, "42"},
		// \number and \romannumeral
		{`\count5=42 \message{\number\count5,\romannumeral 9}`, "42,ix"},
		// conditionals
		{`\message{\ifnum 3>2 yes\else no\fi}`, "yes"},
		{`\message{\ifnum 1>2 T\else F\fi}`, "F"},
		{`\message{\ifodd 7 odd\else even\fi}`, "odd"},
		{`\def\a{x}\def\b{x}\message{\ifx\a\b same\else diff\fi}`, "same"},
		{`\def\a{x}\def\b{y}\message{\ifx\a\b same\else diff\fi}`, "diff"},
		// \ifcase
		{`\message{\ifcase 2 zero\or one\or two\or three\fi}`, "two"},
		// \edef freezes expansion at definition time
		{`\def\a{Q}\edef\b{\a\a}\def\a{R}\message{\b}`, "QQ"},
		// \chardef
		{`\chardef\x=65 \message{\the\x}`, "65"},
		// nested conditionals
		{`\message{\ifnum 1<2 \ifnum 2<3 both\else one\fi\else none\fi}`, "both"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}

// A `%` comment consumes the end-of-line: it leaves NO interword space, and the
// next line starts in state N (leading spaces ignored). The critical consequence
// is that `\def\x#1%⏎   {…}` defines an UNDELIMITED macro — a leaked space would
// make it space-delimited, which is exactly the tokenizer bug that made a package
// like algorithmicx swallow the whole document (its `\ALG@p@main#1%⏎  {…}` parser
// then never terminated on its `]` delimiter).
func TestCommentEatsEndOfLine(t *testing.T) {
	cases := []struct{ src, want string }{
		// `#1` then `%`, newline, indentation, `{` ⇒ undelimited: \x reads one token.
		{"\\def\\x#1%\n   {[#1]}\\message{\\x aZ}", "[a]Z"},
		// text: `%` joins the two lines with no space.
		{"\\message{foo%\nbar}", "foobar"},
		// a blank line after a comment still breaks the paragraph: \par is seen,
		// so \ifvmode is true right after it.
		{"\\message{x}%\n\n\\message{\\ifvmode V\\else H\\fi}", "x V"},
	}
	for _, c := range cases {
		if got := run(t, c.src); got != c.want {
			t.Errorf("src=%q\n got =%q\n want=%q", c.src, got, c.want)
		}
	}
}

// TestScoping verifies that groups save and restore meanings and registers.
func TestScoping(t *testing.T) {
	if got := run(t, `\def\a{out}{\def\a{in}\message{\a}}\message{\a}`); got != "in out" {
		t.Errorf("scoping def = %q, want %q", got, "in out")
	}
	if got := run(t, `\count0=1 {\count0=2 \message{\the\count0}}\message{\the\count0}`); got != "2 1" {
		t.Errorf("scoping count = %q, want %q", got, "2 1")
	}
	// \global escapes the group.
	if got := run(t, `\def\a{out}{\global\def\a{in}}\message{\a}`); got != "in" {
		t.Errorf("global def = %q, want %q", got, "in")
	}
}

func TestUndefined(t *testing.T) {
	if _, err := New().Run(`\nosuchcs`); err == nil {
		t.Error("undefined control sequence should error")
	}
}
