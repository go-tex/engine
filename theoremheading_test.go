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
		{"a macro in the title", `\newtheorem{thm}{\textbf{Théorème}}`, "Théorème1.", "{"},
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

// A class may redefine \@begintheorem with amsthm's OWN signature, whose third
// parameter is DELIMITED: \def\@begintheorem#1#2[#3]. Journal classes copy it
// verbatim — oup's oupau.cls does.
//
// Called with no bracket group, such a macro reads forward through the document
// until it finds one. In 2401.17012 a theorem ate the thirty lines that followed
// it, up to the [X_\beta,X_\gamma] inside a formula, and that formula came back
// from the maths layer as one dropped equation the size of the rest of the paper
// ("texmath: unexpected \"}\"").
//
// amsthm never calls it bare: amsthm.sty:143 goes through \@oparg, which supplies
// [] when the document wrote no note. Neither does this now.
func TestATheoremDoesNotHuntForALaterBracket(t *testing.T) {
	const src = `\documentclass{article}
\makeatletter\def\@begintheorem#1#2[#3]{\noindent\textbf{#1 #2}\ }\makeatother
\newtheorem{thm}{Theorem}
\begin{document}
\begin{thm}
BODY
\end{thm}
LATER [BRACKETED] TAIL
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	for _, want := range []string{"BODY", "LATER", "BRACKETED", "TAIL"} {
		if !strings.Contains(got, want) {
			t.Errorf("the page lost %q — the theorem read past it looking for a bracket:\n%s", want, got)
		}
	}
}

// And a theorem with no note still heads "Theorem 1." rather than "Theorem 1 ()":
// the empty bracket group \@oparg supplies is not a note.
func TestAnEmptyNoteIsNoNote(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\newtheorem{thm}{Theorem}`+
		`\begin{document}\begin{thm}body\end{thm}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); strings.Contains(got, "()") {
		t.Errorf("the head carries an empty note: %q", got)
	}
}

// amsgen's \@ifempty, which amsthm's head is built out of: a theorem with no note
// drops the parentheses by \@ifempty{#3}{\let\thmnote\@gobble}{\let\thmnote\@iden}.
// Undefined, the call was skipped and its three arguments were TYPESET — which is
// how a class that styles its own theorems came to print its head twice.
func TestIfEmptyChoosesByEmptiness(t *testing.T) {
	for _, c := range []struct{ arg, want, absent string }{
		{"", "YES", "NO"},
		{"x", "NO", "YES"},
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}\makeatletter`+
			`\@ifempty{`+c.arg+`}{YES}{NO}\makeatother\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%q: %v", c.arg, err)
		}
		got := pageChars(e)
		if !strings.Contains(got, c.want) || strings.Contains(got, c.absent) {
			t.Errorf(`\@ifempty{%s} put %q on the page, want %q and not %q`, c.arg, got, c.want, c.absent)
		}
	}
}
