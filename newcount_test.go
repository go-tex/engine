package engine

import "testing"

// TestNewcountLoop exercises \newcount, \countdef-style register aliasing,
// assignment (\n=0), \advance, \the on a named register, and \ifnum — the plain
// \loop macro drives the iteration.
func TestNewcountLoop(t *testing.T) {
	e := New()
	got, err := e.Run(
		`\def\loop#1\repeat{\def\body{#1}\iterate}` +
			`\def\iterate{\body \let\next\iterate \else\let\next\relax\fi\next}` +
			`\def\out{}\newcount\n \n=0 ` +
			`\loop \advance\n by 1 \edef\out{\out\the\n}\ifnum\n<4 \repeat ` +
			`\message{\out}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "1234\n" && got != "1234" {
		t.Fatalf("got %q want 1234", got)
	}
}

// TestCountdefDirect checks the \countdef primitive alias path independently.
func TestCountdefDirect(t *testing.T) {
	e := New()
	got, err := e.Run(`\countdef\c=42 \c=7 \advance\c by 5 \multiply\c by 2 \message{\the\c}`)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "24\n" && got != "24" {
		t.Fatalf("got %q want 24", got)
	}
}
