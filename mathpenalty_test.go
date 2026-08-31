// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \penalty is append_penalty — scan_int, then a penalty node (tex.web §21242-21250).
// It is a break cost carrying no maths at all, but inside a formula it reached the
// maths layer as an unknown command and took the formula with it, in 15 of the 200
// arXiv papers.
func TestMathPenaltyIsStripped(t *testing.T) {
	for _, c := range []struct{ nom, math string }{
		{"entier", `a\penalty 100 b`},
		{"entier négatif", `a\penalty-100 b`},
		{"registre", `a\penalty\@M b`},
		{"avant/après display", `a\postdisplaypenalty 0 b`},
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

// \dag and \ddag mean a different symbol in maths than in text:
//
//	\DeclareRobustCommand{\dag}{\ifmmode{\dagger}\else\textdagger\fi}  (latex.ltx:7195)
//
// and the engine's own \dag is plain.tex's \mathhexbox form, which reaches the maths
// layer as \char and costs the formula.
func TestMathDaggerAliases(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}$x^\ddag+y^\dag$\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("la couche maths a refusé la formule (%v)", e.mathDropped)
	}
}
