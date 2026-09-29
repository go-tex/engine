package engine

import (
	"strings"
	"testing"
)

// foreachText runs a snippet and reads back what it typeset, so a loop can be
// judged by what reached the page rather than by its internals.
func foreachText(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	collectChars(e.parList, &b)
	return b.String()
}

// \foreach runs its body once per value, with the variable bound to it.
func TestForeachRunsItsBodyPerValue(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{"a plain list", `\foreach \x in {a,b,c}{[\x]}`, "[a][b][c]"},
		{"spaces around the values", `\foreach \x in { a , b }{[\x]}`, "[a][b]"},
		// The dots are pgffor's range, and the step comes from what precedes them:
		// {1,...,4} counts by one, {1,3,...,9} by two, because the two values
		// before the dots set it.
		{"a range", `\foreach \x in {1,...,4}{[\x]}`, "[1][2][3][4]"},
		{"a range with a step", `\foreach \x in {1,3,...,9}{[\x]}`, "[1][3][5][7][9]"},
		{"a descending range", `\foreach \x in {3,2,...,0}{[\x]}`, "[3][2][1][0]"},
		{"one value", `\foreach \x in {7}{[\x]}`, "[7]"},
		// The variable is bound inside a group, so it does not leak past the loop.
		{"the body may be more than the variable", `\foreach \x in {1,2}{(\x-\x)}`, "(1-1)(2-2)"},
	} {
		if got := foreachText(t, c.src); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// The loop variable is local to each turn: after the loop it means what it meant
// before, which is what running the body inside a group is for.
func TestForeachDoesNotLeakItsVariable(t *testing.T) {
	got := foreachText(t, `\def\x{Z}\foreach \x in {a,b}{[\x]}[\x]`)
	if want := "[a][b][Z]"; got != want {
		t.Errorf("%q, want %q", got, want)
	}
}

// A \foreach that is not the shape this implements is handed back to the
// undefined-command path untouched, so the census keeps reporting it instead of
// this quietly doing half the loop.
//
// The multi-variable form is the one to watch: \foreach \x/\y in {a/b} binds two
// names per turn, and binding only the first would set the body with the wrong
// values rather than not at all.
func TestForeachLeavesShapesItDoesNotImplementToTheCensus(t *testing.T) {
	for _, src := range []string{
		`\foreach \x/\y in {a/b,c/d}{[\x\y]}`,
		`\foreach [count=\i] \x in {a,b}{[\x]}`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		e.lenient = true // the census only exists in the mode that keeps going
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if e.skippedCS["foreach"] == 0 {
			t.Errorf("%s: \\foreach was not reported as unimplemented", src)
		}
	}
}

// A range whose ends are not plain numbers is not one this understands, and it
// must be reported rather than guessed at.
func TestForeachRefusesARangeItCannotCount(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.lenient = true
	if _, err := e.Run(`\foreach \x in {a,...,e}{[\x]}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if e.skippedCS["foreach"] == 0 {
		t.Error("an alphabetic range is not implemented and must be reported")
	}
}
