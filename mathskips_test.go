// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Assignments and stretch carry no maths, and an unknown command costs the whole
// formula. Three families reached the maths layer that way:
//
//   - \setlength{\len}{dim} is \def\setlength#1#2{#1 #2\relax} (latex.ltx:7347) —
//     an assignment with two arguments;
//   - \hfil and its kin are \hskip with a fixed glue —
//     primitive("hfil",hskip,fil_code), fil_glue = 0pt plus 1fil (tex.web:20547,
//     §3318) — so they take no argument, and the maths layer cannot stretch anyway;
//   - \leftskip and the other glue PARAMETERS are assignments that read a <glue>
//     (tex.web §224), with TeX's optional equals before it.
func TestMathAssignmentsAndStretchAreStripped(t *testing.T) {
	for _, c := range []struct{ nom, math string }{
		{"hfill", `a\hfill b`},
		{"hss", `a\hss b`},
		{"vfil", `a\vfil b`},
		{"setlength", `\setlength{\fboxsep}{2pt}x`},
		{"addtolength", `\addtolength{\fboxsep}{2pt}x`},
		{"leftskip avec =", `\leftskip=0pt y`},
		{"leftskip sans =", `\leftskip 0pt y`},
		{"parskip registre", `\parskip\medskipamount y`},
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}$`+c.math+`$\end{document}`),
			Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé la formule (%v)", c.nom, e.mathDropped)
		}
	}
}
