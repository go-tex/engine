package engine

import "testing"

// An UNMATCHED } can never be part of a macro argument. Real TeX stops with
// "Argument of \x has an extra }" (checked against tectonic); this engine took the
// brace INTO the argument, where it went on to close a group it had not opened.
//
// That is how a brace counter drifts. The box builder (buildBoxList) raises its
// depth on a { it sees and never gets the } back, so the box's own closing brace is
// spent on the wrong level and the loop reads past the end of the box — leaking a
// group AND eating what follows. beamer hit it on every \mode<…>:
// \beamer@masterdecode leaked THREE groups per call, and \documentclass{beamer}
// ended with 22 groups open (article: 0), which threw away every definition made
// after them.
func TestDelimitedArgumentRefusesAnExtraBrace(t *testing.T) {
	// The argument does not swallow the brace, and the call is abandoned rather
	// than run with a malformed argument.
	out := runExpr(t, `\let\stop\relax\def\eat#1\stop{[#1]}\message{A}\eat }\stop`)
	if out != "A" {
		t.Errorf("an extra } was taken into the argument: %q", out)
	}
	// It is recorded, not silently swallowed.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\let\stop\relax\def\eat#1\stop{}\eat }\stop`); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Named after the macro whose argument was abandoned, so a corpus sweep says
	// WHICH macro to look at rather than only that one of them failed.
	if e.SkippedCommands()[`Argument of \eat has an extra }`] == 0 {
		t.Errorf("the extra } was not reported against \\eat: %v", e.SkippedCommands())
	}
	// A BALANCED brace group inside a delimited argument is still fine.
	if got := runExpr(t, `\let\stop\relax\def\eat#1\stop{\message{[#1]}}\eat {a}b\stop`); got != "[{a}b]" {
		t.Errorf("a balanced group in a delimited argument = %q, want [{a}b]", got)
	}
}

// The box builder keeps its brace counter and the group stack in step: a box body
// hands back exactly the group depth it was given.
func TestBoxBodyLeavesTheGroupStackBalanced(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\let\stop\relax\newbox\bb\def\eat#1\stop{}\setbox\bb=\hbox{{\eat }\stop A}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(e.groups) != 0 {
		t.Errorf("a box body left %d group(s) open", len(e.groups))
	}
}
