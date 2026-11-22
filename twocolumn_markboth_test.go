// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A \markboth before \twocolumn[...] must not ship the empty one-column region as a page.
// sn-jnl.cls issues \markboth from \title, ahead of \twocolumn[\@maketitle]; a mark is
// invisible, so the region has no ink and the document stays on one page. Witness b2
// (unblock-outliers-2.md): tectonic 1 page.
func TestMarkbothBeforeTwocolumnKeepsOnePage(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	src := `\documentclass[twocolumn]{article}
\begin{document}
\markboth{T}{T}
\twocolumn[\centering SPANTITLE\par\vskip 20pt]
\section{Introduction}
BODYSTART text.
\end{document}
`
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := len(e.Pages()); got != 1 {
		t.Errorf("\\markboth then \\twocolumn[...]: %d pages, tectonic gives 1", got)
	}
}
