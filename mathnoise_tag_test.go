// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// go-tex/math refuses an unknown command by refusing the WHOLE equation, so a
// command that carries no maths still costs the formula it stands in. These four
// carry none: \tag is the equation's number, \intertext is prose between the rows
// of an alignment, and the style switches change a size the maths layer sets itself.
func TestMathNoiseKeepsTheEquation(t *testing.T) {
	for _, src := range []string{
		`\[ a = b \tag{1}\]`,
		`\[ a = b \tag*{$\star$}\]`,
		`$\scriptscriptstyle a$`,
		`$\textstyle a$`,
		`$a \bigtimes b$`,
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src + `\par`); err != nil {
			t.Fatalf("Run(%q): %v", src, err)
		}
		if d := e.Diagnostics().MathDropped; len(d) != 0 {
			t.Errorf("%s dropped the equation: %v", src, d)
		}
	}
}

// \tag inside an equation environment is still READ as the number, not stripped:
// that path predates this and must keep working.
func TestEquationTagStillNumbers(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\begin{equation} a = b \tag{7}\end{equation}`); err != nil {
		t.Fatal(err)
	}
	if d := e.Diagnostics().MathDropped; len(d) != 0 {
		t.Errorf("the equation was dropped: %v", d)
	}
}
