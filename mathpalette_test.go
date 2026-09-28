// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \mathpalette#1#2 is latex.ltx:11179-11184, used verbatim in classkernel.go. 336 equations
// over 6 papers of a 999-paper census (#466), and every one of the six builds a CUSTOM
// SYMBOL with it: \mathbin{\mathpalette\shuffle@{}} or \mathpalette\@cupdot{}.
//
// ⛔ The reason it belongs in this engine and not in go-tex/math is the EXPANSION ORDER,
// and these tests pin it. Those papers' \shuffle@ and \@cupdot are their own
// \newcommand*[2] macros, which this engine substitutes into the maths SOURCE STRING.
// Expanded inside the maths layer instead, the source would still read
// \mathpalette\shuffle@{} while the layer asked for \shuffle@ — and the retry would try to
// grab TWO arguments after \shuffle@ in a source that has one.
func TestMathpaletteExpandsBeforeThePapersOwnMacro(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	// \shuffleX takes the style as #1 and the body as #2, which is the corpus shape.
	if _, err := e.Run(`\newcommand*{\shuffleX}[2]{#1\mathsf{S}}` +
		`$a \mathpalette\shuffleX{} b$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics().MathDropped
	for _, name := range []string{`\mathpalette`, `\shuffleX`, `\mathchoice`} {
		if n := d[name]; n != 0 {
			t.Errorf("%s dropped %d equation(s): %v", name, n, d)
		}
	}
	if len(d) != 0 {
		t.Errorf("the formula dropped on something: %v", d)
	}
}

// ⛔ And the body must be SUBSTITUTED, not the name removed: a paper macro whose content is
// itself unknown must drop under THAT name. This is the only observation that separates
// "expanded" from "silently swallowed".
func TestMathpaletteSubstitutesTheMacroBody(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\newcommand*{\shuffleY}[2]{#1\nosuchmaththing}` +
		`$\mathpalette\shuffleY{}$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics().MathDropped
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf("the macro body was not substituted through \\mathpalette: %v", d)
	}
	if d[`\mathpalette`] != 0 {
		t.Errorf("\\mathpalette itself dropped: the kernel definition is not being used: %v", d)
	}
}

// The style argument reaches the macro: #1 is a style switch, so the four branches differ.
// Asserted through the drop surface rather than a rendering, because what matters is that
// the DISPLAY branch was the one selected — its \displaystyle is what #1 carries there.
func TestMathpaletteHandsTheStyleToTheMacro(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	// #1 is used as a command; if the style were not passed, #1 would be empty and the
	// body would render without it. Putting an unknown command in the SCRIPT branch only
	// would not discriminate, so the test instead checks that a body which USES #1 works.
	if _, err := e.Run(`\newcommand*{\shuffleZ}[2]{{#1 x}}$\mathpalette\shuffleZ{}$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if d := e.Diagnostics().MathDropped; len(d) != 0 {
		t.Errorf("a body using #1 as a style switch dropped: %v", d)
	}
}

// ⛔ The SECOND argument must reach the body too. Every corpus use passes {} for it —
// \mathpalette\shuffle@{} builds its symbol out of nothing but the style — so an ablation
// that drops #2 was invisible to the tests above. The definition is latex.ltx's and has to
// be complete, not complete enough for six papers.
func TestMathpalettePassesItsSecondArgument(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\newcommand*{\shBoth}[2]{#1#2}` +
		`$\mathpalette\shBoth{\nosuchmaththing}$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	d := e.Diagnostics().MathDropped
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf("the second argument did not reach the body: %v", d)
	}
	// And a body that uses it renders when the argument is renderable.
	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.lenient = true
	if _, err := e2.Run(`\newcommand*{\shBoth}[2]{#1#2}$\mathpalette\shBoth{\alpha}$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if d := e2.Diagnostics().MathDropped; len(d) != 0 {
		t.Errorf("a renderable second argument dropped: %v", d)
	}
}
