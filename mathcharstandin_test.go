// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The engine spells some characters it has no glyph command for as \char:
//
//	\def\cdot{\char183\relax}   \def\bullet{\char8226\relax}   (latex.go)
//
// In a formula those names belong to the maths layer, which renders ⋅ and ∙ itself.
// Flattening a substituted body turned them into \char — a primitive the layer
// refuses, costing the whole formula. A paper's own \expect{}{\cdot} lost every
// occurrence that way.
func TestMathKeepsACharStandIn(t *testing.T) {
	for _, c := range []struct{ nom, def, math string }{
		{"cdot dans une macro", `\newcommand{\expct}[2]{\mathbb{E}_{#1}[#2]}`, `\expct{}{\cdot}`},
		{"bullet dans une macro", `\newcommand{\op}[1]{\mathrm{op}(#1)}`, `\op{x\bullet y}`},
	} {
		e, err := compile([]byte(`\documentclass{article}`+c.def+
			`\begin{document}$`+c.math+`$\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé la formule (%v)", c.nom, e.mathDropped)
		}
	}
}
