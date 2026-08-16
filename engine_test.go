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

// \futurelet\cs<t1><t2> lets \cs take t2's meaning WITHOUT consuming t1 or t2 —
// the one-token lookahead that generic LaTeX scanners depend on. Its absence made
// elsarticle-style classes (ifacconf) loop forever: their \thanksref scanner ends
// each element with \futurelet and never sees its \relax terminator.
func TestFuturelet(t *testing.T) {
	// \futurelet\pk\chk<t2> lets \pk take t2's meaning and runs \chk (t1) with
	// both tokens still in the stream. Here t2 is \relax, so \chk sees \pk\ifx\relax.
	// (Run in the token stream, not \message: \futurelet is an assignment.)
	if got := run(t, `\def\chk{\ifx\pk\relax\def\R{sawrelax}\else\def\R{other}\fi}`+
		`\futurelet\pk\chk\relax\message{\R}`); got != "sawrelax" {
		t.Errorf("futurelet lookahead = %q, want %q", got, "sawrelax")
	}
	// A delimited-argument scanner that ends by peeking for its \relax terminator
	// must terminate rather than consume off the end.
	got := run(t, `\def\scan#1\stop{\futurelet\p\chk}`+
		`\def\chk{\ifx\p\relax\def\R{done}\else\expandafter\eat\fi}\def\eat#1{\scan}`+
		`\scan A\stop\relax\message{\R}`)
	if got != "done" {
		t.Errorf("futurelet scanner terminate = %q, want %q", got, "done")
	}
}

// Inside \message/\typeout, \protect shields the next control sequence from
// expansion (LaTeX \let\protect\string). Without it, a self-referential warning
// like ieeeconf's \typeout{… \protect\section …} — where \section expands to a
// macro that re-issues that very warning — loops forever.
func TestProtectInMessage(t *testing.T) {
	// \protect keeps \foo literal; the bare \foo still expands.
	if got := run(t, `\def\foo{EXPANDED}\message{a\protect\foo b}`); got != `a\foob` {
		t.Errorf("protected \\foo = %q, want %q", got, `a\foob`)
	}
	if got := run(t, `\def\foo{EXPANDED}\message{\foo}`); got != "EXPANDED" {
		t.Errorf("unprotected \\foo = %q, want %q", got, "EXPANDED")
	}
	// A self-referential \protect\sec inside its own warning must not recurse.
	if got := run(t, `\def\sec{\@ifstar{\x}{\destroy}}`+
		`\def\destroy#1{\message{use \protect\sec}}\destroy{arg}\message{ok}`); got != `use \sec ok` {
		t.Errorf("self-referential protect = %q, want %q", got, `use \sec ok`)
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
