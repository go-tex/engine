package engine

import "testing"

// \mathchardef\name=<n> is the OTHER way TeX names an integer constant. It exists
// for maths (the number packs a class, a family and a character), but package code
// reaches for it because it costs no \count register: etoolbox builds its
// roman-numeral table that way and the LaTeX kernel's \@M is \mathchardef'd 10000.
// The engine had \chardef and not \mathchardef, so `\mathchardef\etb@rmn@d=500`
// left \mathchardef undefined and typeset "500" into the page — etoolbox, and
// therefore beamer, printed stray numbers before their first line of text.
//
// Read as a number the two are the same; as a MEANING they are not, and TeX makes
// that visible through \ifx and \meaning. The expected strings below are a real
// TeX's (tectonic).
func TestMathchardefNamesAnIntegerConstant(t *testing.T) {
	cases := []struct{ src, want string }{
		// It reads as its number wherever a number is wanted.
		{`\mathchardef\foo=500 \message{\the\foo}`, "500"},
		{`\mathchardef\foo=500 \count0=\foo\message{\the\count0}`, "500"},
		{`\mathchardef\foo=500 \message{\ifnum\foo=500 Y\else N\fi}`, "Y"},
		// …including as the factor of a dimension.
		{`\mathchardef\foo=5 \newdimen\Y\Y=3pt\newdimen\Z\Z=\foo\Y\message{\the\Z}`, "15.0pt"},
		// But it is a different meaning from the \chardef of the same value.
		{`\mathchardef\foo=500 \message{\meaning\foo}`, `\mathchar"1F4`},
		{`\chardef\bar=500 \message{\meaning\bar}`, `\char"1F4`},
		{`\mathchardef\foo=500 \chardef\bar=500 \message{\ifx\foo\bar SAME\else DIFF\fi}`, "DIFF"},
		{`\mathchardef\foo=500 \mathchardef\baz=500 \message{\ifx\foo\baz SAME\else DIFF\fi}`, "SAME"},
		{`\mathchardef\foo=500 \mathchardef\baz=501 \message{\ifx\foo\baz SAME\else DIFF\fi}`, "DIFF"},
		// \global\mathchardef survives the group, like \global\chardef.
		{`{\global\mathchardef\c=90}\message{\the\c}`, "90"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// \@testopt{<cmd>}{<default>} is the kernel's optional-argument dispatcher: it
// hands <cmd> a following [argument], or [<default>] when the caller wrote none.
// It was missing, so etoolbox's \newrobustcmd — which is \@testopt{...}0 — typeset
// its own default, and every etoolbox-based package (beamer's whole base) opened
// with a page of stray 0s and [1]s.
func TestTestoptSuppliesTheDefaultOptionalArgument(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\makeatletter\def\c[#1]{\message{<#1>}}\@testopt\c{D}`, "<D>"},
		{`\makeatletter\def\c[#1]{\message{<#1>}}\@testopt\c{D}[X]`, "<X>"},
		// A multi-token default arrives as ONE argument, because \@testopt braces it.
		{`\makeatletter\def\c[#1]{\message{<#1>}}\@testopt\c{a b}`, "<a b>"},
		// \@protected@testopt drops its first argument (the protect-branch command)
		// and dispatches the rest, which is how \newcommand's optional form calls it.
		{`\makeatletter\def\c[#1]{\message{<#1>}}\@protected@testopt\c\c{D}`, "<D>"},
		{`\makeatletter\def\c[#1]{\message{<#1>}}\@protected@testopt\c\c{D}[X]`, "<X>"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}

// \@star@or@long\cmd consumes a leading star and records, in \l@ngrel@x, the prefix
// the definition it is about to make should carry: \relax after a star, \long
// without one. The engine set \l@ngrel@x to an empty macro instead, which is
// neither — and etoolbox READS that meaning (\ifx\l@ngrel@x\relax) to choose
// between \protected and \protected\long, so a starred \newrobustcmd* silently
// became long.
func TestStarOrLongRecordsTheLongPrefix(t *testing.T) {
	cases := []struct{ src, want string }{
		{`\makeatletter\def\c{\message{\ifx\l@ngrel@x\relax STAR\else PLAIN\fi}}\@star@or@long\c*`, "STAR"},
		{`\makeatletter\def\c{\message{\ifx\l@ngrel@x\relax STAR\else PLAIN\fi}}\@star@or@long\c`, "PLAIN"},
		{`\makeatletter\def\c{\message{\ifx\l@ngrel@x\long LONG\else NO\fi}}\@star@or@long\c`, "LONG"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s\n = %q, want %q", c.src, got, c.want)
		}
	}
}
