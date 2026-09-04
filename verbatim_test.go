package engine

import (
	"strings"
	"testing"
)

// \verb sets its delimited text literally inline: backslashes, braces and other
// specials are ordinary characters, no macro expands.
func TestVerbInline(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\verb|a\b{}$|`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != `a\b{}$` {
		t.Errorf("\\verb typeset %q, want %q", got, `a\b{}$`)
	}
}

// The verbatim environment sets each raw line literally — %, \, $, {, } are all
// ordinary — and the material is copied verbatim up to \end{verbatim}.
func TestVerbatimBlock(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{verbatim}\nx$y%z\\w\nsecond()\n\\end{verbatim}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// mvlText concatenates glyphs across lines (spaces are kerns, dropped).
	if got := mvlText(e.mvl); got != `x$y%z\wsecond()` {
		t.Errorf("verbatim typeset %q, want %q", got, `x$y%z\wsecond()`)
	}
	// Two content lines → two line boxes on the main vertical list.
	lines := 0
	for _, n := range e.mvl {
		if _, ok := n.(*boxNode); ok {
			lines++
		}
	}
	if lines != 2 {
		t.Errorf("verbatim produced %d line boxes, want 2", lines)
	}
}

// Verbatim glyphs keep their source line, so click-to-source works inside a
// verbatim block too. \begin is line 1, the two code lines are 2 and 3.
func TestVerbatimSourceLines(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := "\\begin{verbatim}\nalpha\nbeta\n\\end{verbatim}"
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := charLine(e.mvl)
	if got['a'] != 2 { // 'a' first appears in "alpha" on line 2
		t.Errorf("verbatim glyph 'a' source line = %d, want 2", got['a'])
	}
	if got['b'] != 3 { // 'b' first appears in "beta" on line 3
		t.Errorf("verbatim glyph 'b' source line = %d, want 3", got['b'])
	}
}

// fancyvrb's \Verb takes its text braced as well as delimited. The braced form is
// the one that must be read from TOKENS: a paper writes
// \newcommand{\jl}[1]{\small\Verb{#1}} and uses it a hundred times, so by the time
// \Verb runs its argument is a macro parameter and the file is long past.
func TestVerbFancyReadsBracedAndDelimitedForms(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{"braced", `\Verb{code_ici()}`, "code_ici()"},
		{"delimited", `\Verb|code_ici()|`, "code_ici()"},
		{"through a macro", `\newcommand{\jl}[1]{\Verb{#1}}\jl{code_ici()}`, "code_ici()"},
		{"with options", `\Verb[fontsize=\small]{code_ici()}`, "code_ici()"},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(tc.src + `\par`); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		got := strings.ReplaceAll(mvlText(e.mvl), " ", "")
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: got %q, want it to carry %q", tc.name, got, tc.want)
		}
	}
}

// The delimited form must not lose its first character: peeking with a TOKEN read
// moves the buffer cursor past the delimiter, and \Verb|autre_code()| then came out
// as "utre_code()||" — the character after the bar had become the delimiter.
func TestVerbFancyDelimitedKeepsItsFirstCharacter(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`avant \Verb|autre_code()| apres\par`); err != nil {
		t.Fatal(err)
	}
	got := strings.ReplaceAll(mvlText(e.mvl), " ", "")
	if !strings.Contains(got, "autre_code()") || strings.Contains(got, "|") {
		t.Errorf("delimited \\Verb mangled: %q", got)
	}
}
