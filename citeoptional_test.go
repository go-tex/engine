// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \cite takes an optional NOTE, and reading its braces first is how its arguments
// end up on the page. latex.ltx:
//
//	\def\cite{\@ifnextchar[{\@tempswatrue\@citex}{\@tempswafalse\@citex[]}}
//	\def\@citex[#1]#2{…\if@tempswa , #1\fi…}
//
// so the note is appended AFTER the key list, separated by ", ". Without the
// optional read, readBraceName met "[" instead of "{", returned nothing, and the
// whole "[etc.]{Zeilberger1990,chyzak98,…}" was typeset as text — on arXiv
// 2311.05246 that is one unbroken run 2665pt wide on a 478pt measure, with the
// words after it run together (go-tex/engine#629). 6621 papers of the pool write
// \cite[, 38 of the measured 154.
func TestCiteTakesItsOptionalNote(t *testing.T) {
	for _, c := range []struct{ name, cite, want string }{
		{"aucun crochet", `\cite{alpha}`, "[1]"},
		{"une note", `\cite[etc.]{alpha}`, "[1, etc.]"},
		{"deux notes", `\cite[see][etc.]{alpha}`, "[see 1, etc.]"},
		{"plusieurs clés", `\cite[etc.]{alpha,bravo}`, "[1, 2, etc.]"},
	} {
		got := citeText(t, c.cite)
		if !strings.Contains(got, strings.ReplaceAll(c.want, " ", "")) {
			t.Errorf("%s: %s donne %q, attendu %q", c.name, c.cite, got, c.want)
		}
	}
}

// ⛔ And the arguments must not reach the page. This is the defect: the keys
// themselves, set as text, in a run that cannot break.
func TestACiteNoteIsNotTypesetAsText(t *testing.T) {
	got := citeText(t, `\cite[etc.]{Zeilberger1990,chyzak98}`)
	for _, leak := range []string{"Zeilberger1990", "chyzak98", "]["} {
		if strings.Contains(got, leak) {
			t.Errorf("%q a fui sur la page: %q", leak, got)
		}
	}
}

// citeText compiles a tiny bibliography and returns what the page carries.
func citeText(t *testing.T, cite string) string {
	t.Helper()
	src := `\documentclass{article}\begin{document}` + cite +
		`\begin{thebibliography}{9}\bibitem{alpha}Alpha.\bibitem{bravo}Bravo.\end{thebibliography}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", cite, err)
	}
	return mvlText(e.mvl)
}
