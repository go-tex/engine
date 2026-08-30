// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A class may build its display out of $$ and close it inside \end<env>:
// iopart.cls (Institute of Physics, and every paper written to it) opens \equation
// with $$\displaystyle\hskip\mathindent and defines \endequation as $$. The maths
// scanner reads RAW tokens, so \end reached it literally, the closing $$ never
// arrived, and the "equation" ran on through the paragraphs that followed until the
// next $ — one paper rendered 5 pages against tectonic's 26.
//
// \end<env> is run when it carries a math shift, since that is the only way the
// display can close; \end{array}, whose \endarray holds none, stays literal for the
// maths layer to read.
func TestDisplayClosedByAnEnvironmentsOwnEnd(t *testing.T) {
	src := `\documentclass{article}
\makeatletter
\def\equation{$$\displaystyle\hskip\parindent}
\def\endequation{$$}
\makeatother
\begin{document}
\begin{equation}
E = mc^2
\end{equation}
Après.
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); got != "Après." {
		t.Errorf("la page porte %q, want %q — la formule a débordé sur le texte", got, "Après.")
	}
	if n := e.mathDropped["$math$"] + e.mathDropped["\\csname"] + e.mathDropped["\\hskip"]; n != 0 {
		t.Errorf("%d formule(s) abandonnée(s) par la couche maths, want 0", n)
	}
}

// \hskip and its <glue> carry no maths and must not cost the equation.
func TestMathGlueIsStripped(t *testing.T) {
	for _, c := range []struct{ nom, math string }{
		{"registre", `\hskip\parindent x`},
		{"dimen", `\hskip 10pt x`},
		{"dimen élastique", `\kern -3.5pt plus 2pt minus 1pt x`},
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}$`+c.math+`$\end{document}`),
			Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if n := len(e.mathDropped); n != 0 {
			t.Errorf("%s: la couche maths a abandonné la formule (%v)", c.nom, e.mathDropped)
		}
	}
}
