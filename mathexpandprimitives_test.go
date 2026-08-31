// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A macro substituted into a maths source brings TeX with it, not maths: a class's
// \the@inst is \number\c@inst, an author's mark is an \@for loop over \expandafter
// and \edef. Those primitives are the ENGINE's to run — the maths layer can only
// refuse them, and refusing costs the whole formula.
//
// The substituted material is run through the gullet, bounded to what was spliced:
// an isolated expansion stops at its own sentinel (#174), so it cannot read past it.
func TestMathExpandsThePrimitivesAMacroBringsWithIt(t *testing.T) {
	for _, c := range []struct{ nom, preamble, math string }{
		{"number", `\newcount\c@inst \c@inst=3 \def\theinst{\number\c@inst}`, `x^{\theinst}`},
		{"the", `\newcount\c@sec \c@sec=7 \def\thesec{\the\c@sec}`, `y_{\thesec}`},
		{"expandafter", `\def\aa{2}\def\bb{\expandafter\aa}`, `z^{\bb}`},
		{"romannumeral", `\def\rn{\romannumeral 4}`, `w_{\rn}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\makeatletter`+c.preamble+
			`\makeatother\begin{document}$`+c.math+`$\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé la formule (%v)", c.nom, e.mathDropped)
		}
		if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
			t.Errorf("%s: aucun tracé — la formule n'est pas composée", c.nom)
		}
	}
}
