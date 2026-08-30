// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A theorem's heading is not a name. beamer writes
// \newtheorem{theorem}{\translate{Theorem}} so the word comes from the reader's
// language, and a paper writes \newtheorem{thm}{\bfseries Théorème}. Read as a
// NAME, a heading loses every control sequence and keeps the braces around them
// as characters — which is how every beamer talk came to be headed "{Theorem} 2."
// (pageChars collects glyphs, not the spaces between them, hence "Theorem1.")
func TestTheoremHeadingKeepsItsCommands(t *testing.T) {
	for _, c := range []struct{ nom, decl, want, absent string }{
		{"macro dans le titre", `\newtheorem{thm}{\textbf{Théorème}}`, "Théorème1.", "{"},
		{"translate sans le paquet", `\newtheorem{thm}{\translate{Theorem}}`, "Theorem1.", "{"},
		{"titre nu", `\newtheorem{thm}{Lemme}`, "Lemme1.", "{"},
	} {
		e, err := compile([]byte(`\documentclass{article}`+c.decl+
			`\begin{document}\begin{thm}énoncé\end{thm}\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		got := pageChars(e)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: la page porte %q, elle doit porter %q", c.nom, got, c.want)
		}
		if strings.Contains(got, c.absent) {
			t.Errorf("%s: la page porte %q, qui contient %q", c.nom, got, c.absent)
		}
	}
}
