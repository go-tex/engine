package engine

import "testing"

// A macro whose parameter text ends with "#" is delimited by the opening BRACE
// that follows, and that brace is not consumed — it opens the group that comes
// next (TeX §399). \def\a#1#{…} then reads everything up to the "{" as #1.
//
// The LaTeX kernel builds every \newcommand-style definition this way
// (\@yargd@f matches against a ready-made run of nine parameters and stops at the
// requested count with a "#{"), so without it \@argdef defined nothing at all —
// and etoolbox, whose \newrobustcmd goes through \@argdef, silently defined none
// of the commands beamer stands on.
//
// The expected values are a real TeX's (tectonic).
func TestParameterTextEndingInHash(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"text-then-brace", `\def\a#1#{\message{[#1]}}\a xy{Z}`, "[xy]"},
		{"empty-argument", `\def\a#1#{\message{[#1]}}\a{Z}`, "[]"},
		{"after-a-delimiter", `\def\a#1x#2#{\message{[#1|#2]}}\a PxQ{Z}`, "[P|Q]"},
		{"no-parameters", `\def\a#{\message{[plain]}}\a{Z}`, "[plain]"},
	}
	for _, c := range cases {
		if got := runExpr(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The brace really is LEFT for the group that follows: the body below sees it as
// the start of its own group, so what is inside still reaches the message.
func TestParameterTextEndingInHashLeavesTheBrace(t *testing.T) {
	const src = `\def\a#1#{\message{[#1]}\gotexshow}\def\gotexshow#1{\message{{#1}}}\a xy{Z}`
	if got := runExpr(t, src); got != "[xy] {Z}" {
		t.Errorf("%s\n = %q, want %q", src, got, "[xy] {Z}")
	}
}

// TeX's "insert \relax" rule (§510): while a conditional is still SCANNING its
// operands, an \else / \fi belongs to that conditional and not to the number
// being read, so TeX puts a \relax in front of it rather than expanding it.
//
// The idiom is the LaTeX kernel's own date comparison, which writes
// \expandafter\@secondoftwo directly after the second number. Without the rule
// the scan expanded \expandafter, which expanded \else, and \@ifl@t@r answered
// NOTHING for two equal dates — so every package asking \IfFormatAtLeastTF took
// neither branch.
//
// The three expected values below are a real TeX's (tectonic), INCLUDING the
// second, which is not what a reader would guess: with the number ending exactly
// at \expandafter, TeX really does typeset a literal "\relax {Y}{N}".
func TestConditionalScanInsertsRelax(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"equal-takes-else", `\makeatletter\message{[\ifnum 1<1\expandafter\@secondoftwo\else\expandafter\@firstoftwo\fi{Y}{N}]}`, "[Y]"},
		{"true-branch-unravels-in-real-tex", `\makeatletter\message{[\ifnum 1<2\expandafter\@secondoftwo\else\expandafter\@firstoftwo\fi{Y}{N}]}`, `[\relax {Y}{N}]`},
		{"a-space-ends-the-number", `\makeatletter\message{[\ifnum 1<2 \expandafter\@secondoftwo\else\expandafter\@firstoftwo\fi{Y}{N}]}`, "[N]"},
		// A dimension comparison scans operands too, and the rule is armed there.
		{"ifdim-equal-takes-else", `\makeatletter\message{[\ifdim 1pt<1pt\expandafter\@secondoftwo\else\expandafter\@firstoftwo\fi{Y}{N}]}`, "[Y]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The rule must NOT fire for a conditional opened INSIDE the scan: the \else of
// an \if in the operand belongs to that \if. \@parse@version's own
// \if\relax#2\relax\else#1\fi runs inside \ifnum's operand scan, and an
// over-eager rule cut the number short — the comparison then read "1001" instead
// of "20201001".
func TestConditionalScanRelaxRuleSkipsNestedConditionals(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"nested-if-true", `\makeatletter\message{[\ifnum\if TT20\else 30\fi<25 LT\else GE\fi]}`, "[LT]"},
		{"nested-if-false", `\makeatletter\message{[\ifnum\if TX20\else 30\fi<25 LT\else GE\fi]}`, "[GE]"},
		// The kernel's date parse, which is exactly this shape.
		{"date-equal", `\makeatletter\message{[\@ifl@t@r{2020-10-01}{2020-10-01}{Y}{N}]}`, "[Y]"},
		{"date-earlier", `\makeatletter\message{[\@ifl@t@r{2020-10-01}{2021-10-01}{Y}{N}]}`, "[N]"},
		{"date-later", `\makeatletter\message{[\@ifl@t@r{2021-10-01}{2020-10-01}{Y}{N}]}`, "[Y]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
