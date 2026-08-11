package engine

import "testing"

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
