package engine

import "testing"

// \tolerance and \hyphenpenalty govern how the line breaker chooses: the first
// says how bad a line may be before the paragraph is rebroken with hyphenation,
// the second what a hyphen costs when it does. TeX's values are 200 and 50
// (tex.web §240), and LaTeX leaves both alone.
//
// Ours declared them as fresh \newcount registers, so they read 10000 and 0 and
// SHADOWED the engine's own correct defaults (texparams.go, engine.go) — and
// breakSegment reads the register. At 10000 no line is ever too loose or too
// tight to accept, so the breaker packs where TeX would rebreak; at 0 a hyphen is
// free, so it hyphenates where TeX would not.
//
// The expected values are tectonic's, read from the same probe document.
func TestLineBreakingParametersMatchTeX(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\message{[\the\tolerance][\the\pretolerance][\the\hyphenpenalty]` +
		`[\the\exhyphenpenalty][\the\lefthyphenmin][\the\righthyphenmin]}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := trimNL(e.out.String()), "[200][100][50][50][2][3]"; got != want {
		t.Errorf("= %s, want %s", got, want)
	}
}
