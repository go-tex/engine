// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The subfigure environment renders as a lettered sub-panel — not dropped as an
// undefined environment — its \caption prints "(a)", "(b)", and (crucially) the
// panel's \@captype does NOT leak, so the ENCLOSING figure keeps its own number.
func TestSubfigureEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\hsize=400pt\begin{figure}
\begin{subfigure}{0.4\hsize}A\caption{one}\label{sf:a}\end{subfigure}
\begin{subfigure}{0.4\hsize}B\caption{two}\label{sf:b}\end{subfigure}
\caption{main}\label{fig:m}\end{figure}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if n := e.undefinedEnvs["subfigure"]; n != 0 {
		t.Fatalf("subfigure treated as undefined (%d) — dropped, not rendered", n)
	}
	// Sub-captions are lettered.
	if e.labels["sf:a"] != "a" || e.labels["sf:b"] != "b" {
		t.Errorf("sub-caption labels = %q,%q; want a,b", e.labels["sf:a"], e.labels["sf:b"])
	}
	// The panel's \@captype must not leak: the enclosing figure is number 1, NOT a
	// third sub-letter "c".
	if got := e.labels["fig:m"]; got != "1" {
		t.Errorf("enclosing figure label = %q; want \"1\" (panel \\@captype leaked?)", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if got := b.String(); !strings.Contains(got, "(a)") || !strings.Contains(got, "(b)") {
		t.Errorf("rendered sub-captions missing (a)/(b): %s", got)
	}
}
