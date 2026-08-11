package engine

import "testing"

func TestNewcommand(t *testing.T) {
	cases := []struct{ src, want string }{
		// zero-arg
		{`\newcommand{\RR}{REALS}\message{\RR}`, "REALS"},
		// one-arg
		{`\newcommand{\emphx}[1]{<#1>}\message{\emphx{hi}}`, "<hi>"},
		// two-arg
		{`\newcommand{\pair}[2]{(#1,#2)}\message{\pair{a}{b}}`, "(a,b)"},
		// unbraced name form
		{`\newcommand\greet[1]{Hi #1!}\message{\greet{Bob}}`, "Hi Bob!"},
		// \renewcommand overrides
		{`\newcommand{\X}{one}\renewcommand{\X}{two}\message{\X}`, "two"},
		// first argument optional: no bracket → #1 is the default; the {z} group
		// is leftover text (shown with its braces in \message serialization)
		{`\newcommand{\opt}[1][d]{[#1]}\message{\opt{z}}`, "[d]{z}"},
		// first argument optional: bracket supplies #1
		{`\newcommand{\opt}[1][d]{[#1]}\message{\opt[z]}`, "[z]"},
		// nested use of a 2-arg command
		{`\newcommand{\add}[2]{#1 plus #2}\message{\add{x}{\add{y}{z}}}`, "x plus y plus z"},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		got, err := e.Run(c.src)
		if err != nil {
			t.Errorf("%q: err %v", c.src, err)
			continue
		}
		if trimNL(got) != c.want {
			t.Errorf("%q: got %q want %q", c.src, trimNL(got), c.want)
		}
	}
}
