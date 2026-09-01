// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A substituted macro body is flattened through the gullet (#179) — but \begin and
// \end are macros here:
//
//	\def\begin#1{\gotex@checkenv{#1}\csname #1\endcsname}
//
// so expanding them turns \begin{bmatrix} into \bmatrix, a control sequence the
// maths layer has never heard of, where the environment it wrote is one it renders.
// A paper's own \newcommand{\mymat}[2]{\begin{bmatrix}#1\\#2\end{bmatrix}} lost every
// formula that used it; over the corpus, bmatrix, pmatrix, cases, aligned, split and
// array accounted for 676 fresh drops in 42 papers.
func TestMathKeepsAnEnvironmentInsideAMacro(t *testing.T) {
	for _, c := range []struct{ nom, def, math string }{
		{"bmatrix", `\newcommand{\mymat}[2]{\begin{bmatrix}#1\\#2\end{bmatrix}}`, `A=\mymat{a}{b}`},
		{"cases", `\newcommand{\mycase}[1]{\begin{cases}#1 & x>0\end{cases}}`, `f=\mycase{1}`},
		{"array", `\newcommand{\myarr}[1]{\begin{array}{c}#1\end{array}}`, `M=\myarr{z}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+c.def+
			`\begin{document}$`+c.math+`$\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé la formule (%v)", c.nom, e.mathDropped)
		}
	}
}
