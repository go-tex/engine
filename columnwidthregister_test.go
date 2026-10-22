// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \columnwidth is ONE COLUMN and \hsize is the current paragraph measure. They
// coincide in the body of a document, so a \let between them survives almost
// everything — and parts company exactly where a class sets \hsize for a block of
// front matter and then asks how wide a column is:
//
//	\def\@mkauthors{\begingroup
//	  \hsize=\textwidth               % acmart.cls:2530
//	  …
//	  \@mkauthors@iv                  % :2549
//	\def\@mkauthors@iv{%
//	  \author@bx@wd=\columnwidth      % acmart.cls:2722
//
// With the two conflated the author boxes are the full text width and the grid never
// wraps. latex.ltx:15020 makes \columnwidth its own \newdimen, and \hsize is set FROM
// it (6688, 15079, 15113) — never the reverse.
func TestColumnWidthSurvivesAnHsizeSetToTheTextWidth(t *testing.T) {
	e := New()
	e.hsize = 400 * unity
	e.columnsep = 10 * unity
	e.twoColumn = true
	e.applyTwoColumnMeasure()

	col := (400*unity - 10*unity) / 2
	if e.hsize != col {
		t.Fatalf("mesure de colonne: hsize=%d, want %d", e.hsize, col)
	}
	if got := e.columnWidthValue(); got != col {
		t.Fatalf("\\columnwidth=%d, want la colonne %d", got, col)
	}
	// What a class does next: widen the paragraph measure for the front matter.
	// \columnwidth must NOT follow it.
	e.hsize = e.fullWidth()
	if got := e.columnWidthValue(); got != col {
		t.Errorf("après \\hsize=\\textwidth, \\columnwidth=%d, want la colonne %d — "+
			"il a suivi la mesure", got, col)
	}
}

// ⚠ Unset, \columnwidth answers with the paragraph measure. That is what the old
// \let gave, and it is what a document that never reaches \begin{document}'s
// recompute must keep getting: a fragment, a probe, or a class that states widths and
// stops would otherwise read a column width of ZERO and size every box from it.
func TestColumnWidthUnsetReadsTheMeasure(t *testing.T) {
	e := New()
	e.hsize = 345 * unity
	if got := e.columnWidthValue(); got != 345*unity {
		t.Errorf("\\columnwidth non posé = %d, want la mesure %d", got, 345*unity)
	}
}

// And it is a register the document can set and read back, scoped like one.
func TestColumnWidthIsAssignableAndScoped(t *testing.T) {
	e := New()
	out, err := e.Run(`\columnwidth=150pt \message{[\the\columnwidth]}` +
		`{\columnwidth=70pt \message{[\the\columnwidth]}}\message{[\the\columnwidth]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	const want = "[150.0pt] [70.0pt] [150.0pt]"
	if out != want {
		t.Errorf("= %q, want %q", out, want)
	}
}
