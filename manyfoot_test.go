// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⭐ manyfoot's \DeclareNewFootnote used to be swallowed, and so was everything it
// declares: \footnoteA, \footnotemarkA and \footnotetextA were undefined, and the notes
// of every extra class vanished (12 corpus papers).
//
// Measured against tectonic on \DeclareNewFootnote{A}, \DeclareNewFootnote{B}[roman] and
// \footnote{one}\footnoteA{two}\footnoteB{three}: marks 1, 1, i. Each class has its OWN
// counter, in its own numbering style, and that is what the cases below pin: with a
// shared counter "two" would be 2, and with the style ignored "three" would be 3.
func TestManyfootClassesCountApartInTheirOwnStyle(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\usepackage{manyfoot}
\DeclareNewFootnote{A}
\DeclareNewFootnote{B}[roman]
\SelectFootnoteRule{A}
\begin{document}
Text\footnote{NOTEONE}\footnote{NOTETWO}\footnoteA{NOTEAONE}\footnoteB{NOTEBONE}\footnoteB{NOTEBTWO}.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)
	for _, c := range []struct{ mark, word string }{
		{"1", "NOTEONE"}, {"2", "NOTETWO"},
		{"1", "NOTEAONE"}, // not 3: class A has its own counter
		{"i", "NOTEBONE"}, // not 1: class B numbers in roman
		{"ii", "NOTEBTWO"},
	} {
		if !labelled(txt, c.mark, c.word) {
			t.Errorf("note %s should carry mark %q: %.400q", c.word, c.mark, txt)
		}
	}
}

// \footnotemarkA steps the counter and \footnotetextA does not (its mark already did),
// and an explicit [n] names a number without disturbing the counter — latex.ltx
// :13202-13222, the same contract as the kernel pair.
func TestManyfootMarkTextPairAndExplicitNumber(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\usepackage{manyfoot}
\DeclareNewFootnote{A}
\begin{document}
First\footnotemarkA{} then\footnotetextA{NOTEPAIR} and\footnoteA[7]{NOTESEVEN} and\footnoteA{NOTENEXT}.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	txt := pageText(e)
	for _, c := range []struct{ mark, word string }{
		{"1", "NOTEPAIR"},  // text took the mark the \footnotemarkA stepped to
		{"7", "NOTESEVEN"}, // the explicit number
		{"2", "NOTENEXT"},  // and 7 did not move the counter: 1 -> 2
	} {
		if !labelled(txt, c.mark, c.word) {
			t.Errorf("note %s should carry mark %q: %.400q", c.word, c.mark, txt)
		}
	}
}
