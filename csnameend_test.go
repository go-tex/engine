// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A body scanner that reads RAW must run \csname: it can only produce a control
// sequence, and that sequence may be the \end the scanner is hunting.
//
// beamer reaches every one of its templates that way — \usebeamertemplate{X} is
// \csname beamer@@tmpl@X\endcsname — and the rounded block's closing template is
// \end{beamerboxesrounded}, whose own first move is \end{minipage}. Stored raw, that
// \end never surfaced and the minipage scanner ran to the end of the document.

func TestCsnameCanCarryAnEnvironmentsEnd(t *testing.T) {
	for _, c := range []struct{ nom, ferme string }{
		{"écrit en clair", `\end{minipage}`},
		{"par une macro", `\mf`},
		{"par \\csname", `\csname mf\endcsname`},
	} {
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("%s: buildEngine: %v", c.nom, err)
		}
		out, err := e.Run(`\hsize=200pt\def\mf{\end{minipage}}` +
			`\setbox0=\hbox{\begin{minipage}{100pt}X` + c.ferme + `}` +
			`\message{[largeur \the\wd0]}`)
		if err != nil {
			t.Fatalf("%s: Run: %v", c.nom, err)
		}
		if got := trimNL(out); !strings.Contains(got, "[largeur 100.0pt]") {
			t.Errorf("%s: = %q, want the minipage to end there (100.0pt)", c.nom, got)
		}
	}
}

// The same for the other environments the engine collects raw.
func TestCsnameEndClosesTheCollectedEnvironments(t *testing.T) {
	for _, c := range []struct{ nom, src string }{
		{"tabular", `\def\f{\end{tabular}}\begin{tabular}{ll}A & B\\\csname f\endcsname`},
		{"equation", `\def\f{\end{equation}}\begin{equation}x\csname f\endcsname`},
		{"align", `\def\f{\end{align}}\begin{align}x&=y\csname f\endcsname`},
		{"minipage", `\def\f{\end{minipage}}\begin{minipage}{100pt}X\csname f\endcsname`},
	} {
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("%s: buildEngine: %v", c.nom, err)
		}
		out, err := e.Run(`\hsize=300pt` + c.src + `\message{[suite]}`)
		if err != nil {
			t.Fatalf("%s: Run: %v", c.nom, err)
		}
		if !strings.Contains(out, "[suite]") {
			t.Errorf("%s: the environment swallowed what followed its \\csname end", c.nom)
		}
	}
}
