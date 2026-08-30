// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The name serves two packages. subcaption/subfig spell it as an ENVIRONMENT,
// \begin{subfigure}[pos]{width}…\end{subfigure}; the older subfigure package spells
// it as a COMMAND, \subfigure[caption]{content} — 5 of the 200 papers in the arXiv
// reference corpus still do. Read as an environment, the command form sends
// collectEnvBody looking for an \end{subfigure} that is not there and it swallows
// the rest of the document: one paper fell from 18 pages to 4.
// (pageChars collects glyphs, not the spaces between them.)
func TestSubfigureCommandFormDoesNotSwallowTheDocument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{subfigure}\begin{document}`+
		`\begin{figure}\subfigure[Gauche]{A}\subfigure[Droite]{B}\caption{Toutes deux}\end{figure}`+
		`APRÈS.\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "APRÈS.") {
		t.Fatalf("le document a été avalé: la page porte %q", got)
	}
	for _, want := range []string{"A", "B", "Gauche", "Droite", "(a)", "(b)", "Toutesdeux"} {
		if !strings.Contains(got, want) {
			t.Errorf("la page porte %q, il y manque %q", got, want)
		}
	}
}

// The environment form must be untouched: it is what 44 of those 200 papers use.
func TestSubfigureEnvironmentFormStillPanels(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{subcaption}\begin{document}`+
		`\begin{figure}\begin{subfigure}{100pt}A\caption{Gauche}\end{subfigure}`+
		`\begin{subfigure}{100pt}B\caption{Droite}\end{subfigure}\caption{Toutes deux}\end{figure}`+
		`APRÈS.\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "APRÈS.") {
		t.Fatalf("le document a été avalé: la page porte %q", got)
	}
	if !strings.Contains(got, "(a)") || !strings.Contains(got, "(b)") {
		t.Errorf("les sous-légendes manquent: %q", got)
	}
}
