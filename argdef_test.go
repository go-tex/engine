package engine

import "testing"

// \@argdef and \@yargdef are the kernel's own definition builders — the machinery
// under \newcommand — and packages call them DIRECTLY. etoolbox routes every
// command it defines through \@argdef (no optional argument) or \@xargdef (with
// one), so with them missing etoolbox defined nothing at all: \mode, on which
// every line of beamer stands, simply did not exist, and beamer's own \mode<all>
// printed "<all>" onto the page.
//
// They are written exactly as the LaTeX kernel writes them (read back from a real
// TeX with \meaning), which is why the "#{" parameter text had to work first.
func TestKernelArgumentDefinitionBuilders(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"argdef-two", `\makeatletter\@argdef\foo[2]{\message{[#1|#2]}}\foo AB`, "[A|B]"},
		{"argdef-none", `\makeatletter\@argdef\foo[0]{\message{[none]}}\foo`, "[none]"},
		{"argdef-braced-arguments", `\makeatletter\@argdef\foo[2]{\message{[#1|#2]}}\foo{xy}{zw}`, "[xy|zw]"},
		// \@yargdef with \tw@ makes the first parameter a bracketed one.
		{"yargdef-bracketed-first", `\makeatletter\@yargdef\foo\tw@{2}{\message{[#1|#2]}}\foo[X]B`, "[X|B]"},
		{"yargdef-plain", `\makeatletter\@yargdef\foo\@ne{2}{\message{[#1|#2]}}\foo AB`, "[A|B]"},
		// \@xargdef builds the optional-argument form, default included.
		{"xargdef-default-used", `\makeatletter\@xargdef\foo[2][D]{\message{[#1|#2]}}\foo{B}`, "[D|B]"},
		{"xargdef-default-overridden", `\makeatletter\@xargdef\foo[2][D]{\message{[#1|#2]}}\foo[X]{B}`, "[X|B]"},
		// \@reargdef redefines without the definability test.
		{"reargdef", `\makeatletter\def\foo{old}\@reargdef\foo[1]{\message{[#1]}}\foo Q`, "[Q]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The other kernel internals a real class asks for by name. Each was missing, and
// each one silently skipped is a definition that never happened.
func TestKernelInternalsPackagesCallByName(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// \in@{a}{b}: is <a> inside <b>?
		{"in@-present", `\makeatletter\in@{cd}{abcde}\message{[\ifin@ Y\else N\fi]}`, "[Y]"},
		{"in@-absent", `\makeatletter\in@{xy}{abcde}\message{[\ifin@ Y\else N\fi]}`, "[N]"},
		{"in@-whole", `\makeatletter\in@{abc}{abc}\message{[\ifin@ Y\else N\fi]}`, "[Y]"},
		// \@makeother makes a character ordinary — how a package reads verbatim text.
		{"makeother", `\makeatletter\@makeother\%\message{[\the\catcode` + "`" + `\%]}`, "[12]"},
		// \dospecials is the list \do is mapped over; here it is counted.
		{"dospecials-is-a-list", `\makeatletter\def\do#1{X}\message{[\dospecials]}`, "[XXXXXXXXXXX]"},
		// \kernel@ifnextchar is the kernel's private copy of \@ifnextchar.
		{"kernel-ifnextchar-yes", `\makeatletter\def\c{\kernel@ifnextchar[{\message{B}}{\message{N}}}\c[x]`, "B"},
		{"kernel-ifnextchar-no", `\makeatletter\def\c{\kernel@ifnextchar[{\message{B}}{\message{N}}}\c y`, "N"},
		// \@onlypreamble records the command; the list is what package code walks.
		{"onlypreamble-records", `\makeatletter\@onlypreamble\foo\def\do#1{[\string#1]}\message{\@preamblecmds}`, `[\foo]`},
		// \@ptionlist reports the options a file was loaded with; empty if unknown.
		{"ptionlist-unknown", `\makeatletter\message{[\@ptionlist{nosuch.sty}]}`, "[]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// More kernel internals a real class asks for by name. Each was missing, and each
// one skipped is an assignment that did not happen.
func TestMoreKernelInternals(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// \maxdimen is plain TeX's largest dimension; package code uses it as "no
		// limit", so its absence means the assignment simply does not happen.
		{"maxdimen", `\newdimen\D\D=\maxdimen\message{[\the\D]}`, "[16383.99998pt]"},
		// \@cons appends an \@elt-separated item to a kernel list macro.
		{"cons", `\makeatletter\def\L{}\@cons\L{A}\@cons\L{B}\def\@elt#1{<#1>}\message{[\L]}`, "[<A><B>]"},
		// \@onelevel@sanitize rewrites a macro's content as ordinary characters.
		{"sanitize", `\makeatletter\def\N{ab}\@onelevel@sanitize\N\message{[\N]}`, "[ab]"},
		{"sanitize-strips-the-prefix", `\makeatletter\def\N{a\relax b}\@onelevel@sanitize\N\message{[\meaning\N]}`, `[macro:->a\relax b]`},
		// xcolor's switches, which xxcolor (beamer's colour layer) toggles.
		{"globalcolors", `\makeatletter\globalcolorstrue\message{[\ifglobalcolors Y\else N\fi]}`, "[Y]"},
		{"globalcolors-off", `\makeatletter\globalcolorsfalse\message{[\ifglobalcolors Y\else N\fi]}`, "[N]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
