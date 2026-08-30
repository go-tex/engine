// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A nested math environment carries its OWN & and \\. \begin{bmatrix}…&…\\…
// \end{bmatrix} inside an align cell is one matrix, not four cells over two rows —
// but brace depth does not see it, a matrix being unbraced, so the collector cut
// every matrix into fragments and the maths layer refused each one:
//
//	texmath: \begin{bmatrix} without \end   " = \begin {bmatrix} \mathrm {mat}…"
//
// 622 of the 4172 formulas the 200-paper arXiv corpus drops are that fault.
func TestAlignKeepsNestedEnvironmentsWhole(t *testing.T) {
	for _, c := range []struct{ nom, body string }{
		{"bmatrix", `S &= \begin{bmatrix} a & b \\ c & d \end{bmatrix} \\ &= X`},
		{"cases", `f(x) &= \begin{cases} 1 & x>0 \\ 0 & x\le 0 \end{cases} \\ &= g(x)`},
		{"array imbriqué", `A &= \left(\begin{array}{cc} 1 & 2 \\ 3 & 4 \end{array}\right)`},
	} {
		e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}\begin{document}`+
			`\begin{align*}`+c.body+`\end{align*}\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé %v", c.nom, e.mathDropped)
		}
	}
}

// The align's own \\ and & must still separate: the guard is nesting, not blindness.
func TestAlignStillSeparatesItsOwnRows(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}\begin{document}`+
		`\begin{align}a &= b \\ c &= d\end{align}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if got != "(1)(2)" {
		t.Errorf("les numéros d'équation sont %q, want %q (deux lignes numérotées)", got, "(1)(2)")
	}
}
