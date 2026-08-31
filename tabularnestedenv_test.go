// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A nested environment carries its OWN & and \\: $\begin{matrix} a & b \end{matrix}$
// in a table cell is one matrix, not two cells. TeX protects it with the brace
// \@array opens (latex.ltx:12101, reached through amsmath's \env@matrix), counted in
// align_state (tex.web §6738-6742, §7259-7264). This engine never expands \matrix,
// so the environment is counted instead — the same rule the maths alignment scanner
// has followed since #153.
//
// Split, the formula reached the maths layer as fragments and was refused:
//
//	texmath: unexpected "}"   "\begin {matrix} d_{\ell _2}}"
//
// 224 of the corpus's dropped formulas are that, in tables of matrices.
func TestTabularKeepsNestedEnvironmentsWhole(t *testing.T) {
	src := `\documentclass{article}\usepackage{amsmath}\begin{document}
\begin{tabular}{ c c }
 $\begin{matrix} a & b \end{matrix}$ & $\begin{bmatrix} c & d \end{bmatrix}$ \\
 x & y
\end{tabular}
APRÈS\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("la couche maths a refusé une formule (%v)", e.mathDropped)
	}
	if got := pageChars(e); !strings.Contains(got, "APRÈS") {
		t.Errorf("la page porte %q: le tableau a mangé ce qui le suit", got)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "<path"); n < 12 {
		t.Errorf("%d tracés: les deux matrices ne sont pas composées", n)
	}
}

// The table's own & and \\ must still separate, and a nested tabular must still be
// able to close itself: the guard is nesting, not blindness.
func TestTabularStillSplitsItsOwnCells(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\begin{tabular}{cc}a & b \\ c & \begin{tabular}{c}d\end{tabular}\end{tabular}`+
		`APRÈS\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); !strings.Contains(got, "APRÈS") || !strings.Contains(got, "d") {
		t.Errorf("la page porte %q", got)
	}
}
