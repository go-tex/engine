// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// siunitx's quantities are composed for text (siunitx.go) but reached the maths
// layer as unknown commands, and an unknown command costs the whole formula: a
// capacitance written $C_m = \SI{1}{\micro\farad\per\square\cm}$ left the page with
// nothing on it. 158 of the 4172 formulas the arXiv corpus drops are \SI.
func TestSIUnitxInMath(t *testing.T) {
	for _, c := range []struct{ nom, math string }{
		{"SI", `C = \SI{1}{\micro\farad\per\square\cm}`},
		{"qty", `v = \qty{9.81}{\meter\per\second\squared}`},
		{"num", `n = \num{12345}`},
		{"si seul", `u = \si{\newton\meter}`},
		{"ang", `\theta = \ang{30}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\usepackage{siunitx}\begin{document}$`+
			c.math+`$\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: la couche maths a refusé la formule (%v)", c.nom, e.mathDropped)
		}
	}
}

// A unit symbol is upright, never italic — the SI rule, and siunitx's own default
// (unit-font-command = \mathrm, siunitx.sty:6209). The composed source must say so.
func TestSIUnitxMathPutsUnitsUpright(t *testing.T) {
	got := mathQuantityText("SI", []string{"9.81", `\meter \per \second \squared `})
	if !strings.Contains(got, `\mathrm {`) {
		t.Errorf("composé %q, il manque le \\mathrm des unités", got)
	}
	if !strings.HasPrefix(got, "9.81") {
		t.Errorf("composé %q, la valeur doit venir en tête", got)
	}
}
