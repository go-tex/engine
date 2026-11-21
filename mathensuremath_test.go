// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A macro built on \ensuremath, used inside a formula, carries what its body says
// into the formula's text layer and nothing more. \ensuremath{X} is
// \ifmmode X\else $X$\fi, and \ifmmode was false while flattenMathBody expanded
// the macro for the maths layer, so
//
//	\newcommand{\Data}{\ensuremath{\boldsymbol{D}}}   then   $\Data = 1$
//
// was carried as "$D$ = 1". The ink was the same — the maths layer drew no
// dollar — but the invisible text a reader copies, searches and extracts had two
// dollar signs in every such formula. 2405.18549 defines 136 of these macros
// (macros.tex), and its extracted text lost 6523 characters when this was fixed.
func TestAnEnsuremathMacroInsideAFormulaCarriesNoDollarSigns(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+
		`\newcommand{\Data}{\ensuremath{\boldsymbol{D}}}`+
		`\begin{document}$\Data = 1$\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	text := textLayerContent(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "D = 1") || strings.Contains(text, "$") {
		t.Errorf("the formula's text layer is %q, want \"D = 1\" and no dollar sign", text)
	}
}
