// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A longtable carries its own \caption, and it is a TABLE's. The real package is
// a float-like environment that sets \@captype (longtable.sty's \LT@array), but
// here the body is collected and typeset as cells, so nothing set it:
// \csname the\@captype\endcsname froze over an UNDEFINED \@captype and the
// literal text "\the@captype" reached the page. Every \ref to such a table
// printed it too — 2303.18017 reads "more details in Table \the@captype" six
// times. Five corpus papers write a longtable, eight in all.
//
// tectonic renders the witness below "A: 1".
func TestLongtableCaptionIsATable(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass{article}\hsize=300pt
\begin{document}
\begin{longtable}{ll}
\caption{a long table}
\label{t}
\\
a & b \\
\end{longtable}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.labels["t"], "1"; got != want {
		t.Errorf("\\ref = %q, want %q", got, want)
	}
	if got, want := e.refTypes["t"], "table"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
}

// The binding is SCOPED: a longtable inside a figure must not leave "table"
// behind for the figure's own caption.
func TestLongtableCaptypeIsScoped(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass{article}\hsize=300pt
\begin{document}
\begin{figure}
\begin{longtable}{ll}
\caption{inner}\label{t}
\\
a & b \\
\end{longtable}
\caption{outer}\label{f}
\end{figure}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.refTypes["t"], "table"; got != want {
		t.Errorf("inner type = %q, want %q", got, want)
	}
	if got, want := e.refTypes["f"], "figure"; got != want {
		t.Errorf("outer type = %q, want %q — the longtable's binding leaked", got, want)
	}
	// And the two counters are separate.
	if e.labels["t"] != "1" || e.labels["f"] != "1" {
		t.Errorf("numbers %q / %q, want 1 / 1 (a table counter and a figure counter)",
			e.labels["t"], e.labels["f"])
	}
}
