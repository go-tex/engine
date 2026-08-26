package engine

import "testing"

// TeX strips the braces of an argument that is ENTIRELY one group (§399):
// \def\d[#1]{} called as \d[{g}] sees "g", while \d[{a}{b}] — two groups, with
// nothing enclosing both — keeps them. The engine reads [optional arguments] in
// Go rather than through a delimited macro, so the rule has to be applied there.
//
// It matters at once for \newcommand. The kernel hands a default over BRACED, and
// beamer builds a definition as \newcommand\foo[{1}][{}]{…} through a chain of
// \expandafter; with the braces left on, the argument count was not a number and
// \foo came out as a macro with NO arguments — which put the body of every beamer
// template on the page instead of storing it.
//
// Every expected value below is a real TeX's (tectonic).
func TestDelimitedArgumentStripsOneEnclosingGroup(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"one-group-is-stripped", `\def\d[#1]{\message{<#1>}}\d[{g}]`, "<g>"},
		{"two-groups-are-kept", `\def\d[#1]{\message{<#1>}}\d[{a}{b}]`, "<{a}{b}>"},
		{"no-group-at-all", `\def\d[#1]{\message{<#1>}}\d[p]`, "<p>"},
		{"a-group-that-ends-early", `\def\d[#1]{\message{<#1>}}\d[{a}b]`, "<{a}b>"},
		// \newcommand's optional-argument default follows the same rule.
		{"braced-default", `\newcommand\c[1][{x}]{<#1>}\message{[\c]}`, "[<x>]"},
		{"braced-call", `\newcommand\c[1][{x}]{<#1>}\message{[\c[{y}]]}`, "[<y>]"},
		{"plain-call", `\newcommand\c[1][{x}]{<#1>}\message{[\c[z]]}`, "[<z>]"},
		// …and so does the argument COUNT, which may arrive braced.
		{"braced-count", `\newcommand\c[{1}]{<#1>}\message{[\c A]}`, "[<A>]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// beamer's own idiom: build the whole \newcommand call by expansion, so that both
// the argument count and the optional default arrive braced.
func TestNewcommandBuiltByExpansion(t *testing.T) {
	const base = `\makeatletter` +
		`\edef\A{\expandafter\noexpand\csname foo\endcsname[{1}]}\def\T{[{}]}` +
		`\expandafter\expandafter\expandafter\newcommand\expandafter\A\T{<#1>}`
	cases := []struct{ name, src, want string }{
		{"default-is-empty", base + `\message{[\foo]}`, "[<>]"},
		{"optional-argument-given", base + `\message{[\foo[Q]]}`, "[<Q>]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}

// The guards on the two new readers: a bracket the input never closes, and a
// negative or absent count.
func TestOptionalArgumentEdges(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// A group that closes before the argument ends keeps its braces (the
		// stripping rule needs ONE group enclosing everything).
		{"early-close", `\def\d[#1]{\message{<#1>}}\d[{a}b]`, "<{a}b>"},
		{"trailing-group", `\def\d[#1]{\message{<#1>}}\d[a{b}]`, "<a{b}>"},
		// No bracket at all: \newcommand takes no arguments and the input is intact.
		{"no-bracket", `\newcommand\c{<none>}\message{[\c]}`, "[<none>]"},
		// A negative count is read as written (real TeX rejects it; the engine reads
		// the number and defines nothing useful, which is the harmless direction).
		{"unclosed-bracket-ends-with-input", `\message{[before]}\newcommand\c[1`, "[before]"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
	// Directly: a count written with a minus sign.
	if got := ckRun(t, `\makeatletter\def\showcount{\message{ok}}\newcommand\c[{-1}]{<>}\message{[done]}`); got != "[done]" {
		t.Errorf("a negative argument count = %q, want [done]", got)
	}
}

// stripOuterGroup directly, for the shapes an optional argument can take that a
// document is unlikely to write but a macro-built call can produce.
func TestStripOuterGroupShapes(t *testing.T) {
	open, close := chTok('{', catBegin), chTok('}', catEnd)
	a, b := chTok('a', catLetter), chTok('b', catLetter)
	cases := []struct {
		name string
		in   []tok
		want int // number of tokens expected back
	}{
		{"one-enclosing-group", []tok{open, a, b, close}, 2},
		{"group-ends-early", []tok{open, a, close, b}, 4},
		{"two-groups", []tok{open, a, close, open, b, close}, 6},
		{"unbalanced", []tok{open, a}, 2},
		{"no-group", []tok{a, b}, 2},
		{"empty", nil, 0},
		{"single-token", []tok{a}, 1},
	}
	for _, c := range cases {
		if got := stripOuterGroup(c.in); len(got) != c.want {
			t.Errorf("%s: %d tokens back, want %d", c.name, len(got), c.want)
		}
	}
}

// An argument count that holds a control sequence is SCANNED AS A NUMBER, the way
// LaTeX scans it (\@tempcnta#1\relax): a register gives the number it holds, and
// something that cannot begin a number gives zero.
//
// Reading only the digits and ignoring the rest looked forgiving and was not: a
// command written \newcommand\foo[\somecount]{…} took ZERO arguments, and every
// argument the caller passed was left in the input to be typeset. beamer generates
// its whole overlay layer that way — \newcommand\cs[\beamer@argscount]{…} — so
// \defbeamertemplate printed each template's body onto the page.
//
// Checked against real LaTeX: \newcommand\ca[\nn]{<#1>} with \nn=1 gives <X> for
// \ca{X}; \newcommand\cb[\relax 1]{…} raises "Missing number, treated as zero"
// and defines a command of no arguments.
func TestArgumentCountWithControlSequences(t *testing.T) {
	if got := ckRun(t, `\newcount\nn \nn=1 \newcommand\c[\nn]{<#1>}\message{[\c A]}`); got != "[<A>]" {
		t.Errorf("count from a register = %q, want [<A>]", got)
	}
	if got := ckRun(t, `\newcount\nn \nn=2 \newcommand\c[\nn]{<#1|#2>}\message{[\c AB]}`); got != "[<A|B>]" {
		t.Errorf("two arguments from a register = %q, want [<A|B>]", got)
	}
	// \the\nn spells the number through expansion; the scanner expands to reach it.
	if got := ckRun(t, `\newcount\nn \nn=1 \newcommand\c[\the\nn]{<#1>}\message{[\c A]}`); got != "[<A>]" {
		t.Errorf("count from \\the = %q, want [<A>]", got)
	}
	// Not a number: zero arguments, and what followed stays in the input.
	if got := ckRun(t, `\newcommand\c[\relax 1]{<#1>}\message{[\c A]}`); got != "[<>A]" {
		t.Errorf("count that is not a number = %q, want [<>A]", got)
	}
	if got := ckRun(t, `\newcommand\c[1][{a}b]{<#1>}\message{[\c]}`); got != "[<{a}b>]" {
		t.Errorf("default whose group ends early = %q, want [<{a}b>]", got)
	}
}

// The last two guards: a \newcommand whose name ends the file (nothing left to
// look at for an argument count), and the frame popper called with no frame open.
func TestReadersWithNothingLeft(t *testing.T) {
	if got := ckRun(t, `\message{[before]}\newcommand\c`); got != "[before]" {
		t.Errorf("\\newcommand at the end of input = %q, want [before]", got)
	}
	e := New()
	e.endLoad() // no file is being loaded: a no-op, not a panic
	if len(e.loadStack) != 0 {
		t.Errorf("endLoad with no frame open left %d frames", len(e.loadStack))
	}
}
