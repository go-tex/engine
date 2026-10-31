// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ \\ IN VERTICAL MODE CONTRIBUTES NOTHING — AND IT USED TO EJECT A PAGE.
//
// ltspace.dtx:517-522, \@gnewline:
//
//	\def\@gnewline #1{%
//	  \ifvmode \@nolnerr
//	  \else \unskip \reserved@e {\reserved@f#1}\nobreak \hfil \break \fi}
//
// \@nolnerr is "There's no line here to end" — an error, after which LaTeX has
// contributed nothing and carries on. There is no line to end, so there is nothing to
// do.
//
// As a bare \penalty-10000 it reached the MAIN VERTICAL list outside a paragraph, and
// −10000 there is a FORCED PAGE BREAK. Measured against tectonic on three paragraphs
// separated by a bare \\: the reference reports the error twice and sets all three on
// ONE page; this engine emitted THREE, one per paragraph.
func TestBackslashBreakInVerticalModeContributesNothing(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`Un.\par \\ Deux.\par \\ Trois.\par`); err != nil {
		t.Fatal(err)
	}
	// ⛔ A −10000 penalty in the MAIN vertical list is a forced page break, which is
	// what the three pages were. None may be there.
	for _, n := range e.mvl {
		if p, ok := n.(penaltyNode); ok && p.penalty <= -10000 {
			t.Fatalf("pénalité %d dans la liste verticale: en mode vertical, \\\\ "+
				"ne contribue RIEN (ltspace.dtx:518)", p.penalty)
		}
	}
	if got := len(e.Pages()); got != 1 {
		t.Errorf("%d page(s), want 1: un \\\\ entre deux paragraphes ne doit pas "+
			"éjecter de page", got)
	}
}

// ⛔ The control: in HORIZONTAL mode the forced break must still happen, or the fix
// would have turned \\ into a no-op everywhere. Two lines, one paragraph.
func TestBackslashBreakStillBreaksALine(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\noindent Un\\ Deux\par`); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, x := range e.mvl {
		if b, ok := x.(*boxNode); ok && b != nil && len(b.list) > 0 {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d ligne(s), want 2: en mode horizontal le \\\\ coupe la ligne", n)
	}
}
