// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// macroText returns the characters of a plain macro's body.
func macroText(e *Engine, name string) string {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return ""
	}
	var b []rune
	for _, t := range m.body {
		if !t.cs_ {
			b = append(b, t.ch)
		}
	}
	return string(b)
}

// \begin{subfigure}{w} is a minipage: latex.ltx \@iiiminipage sets \textwidth and
// \columnwidth to the panel width, so \includegraphics[width=\textwidth] and
// \def\svgwidth{\textwidth} scale to the panel, not the page. Witness c1
// (unblock-outliers-2.md): tectonic reads \textwidth = 112.12395pt inside
// \begin{subfigure}{0.325\textwidth}, we read 345.0pt.
func TestSubfigureLocalisesTextwidth(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}
\usepackage{subcaption}
\begin{document}
\begin{figure}
\begin{subfigure}{0.325\textwidth}
\xdef\sfTW{\the\textwidth}\xdef\sfCW{\the\columnwidth}
\caption{a}
\end{subfigure}
\begin{minipage}{0.325\textwidth}
\xdef\mpTW{\the\textwidth}
\end{minipage}
\caption{Fig}
\end{figure}
\xdef\outTW{\the\textwidth}
\end{document}
`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := macroText(e, "sfTW"), macroText(e, "mpTW"); got == "" || got != want {
		t.Errorf("\\textwidth in subfigure = %q, minipage of the same width = %q", got, want)
	}
	if got := macroText(e, "sfTW"); got == macroText(e, "outTW") {
		t.Errorf("\\textwidth in subfigure = %q, still the page's", got)
	}
	if got, want := macroText(e, "sfCW"), macroText(e, "mpTW"); got != want {
		t.Errorf("\\columnwidth in subfigure = %q, want %q", got, want)
	}
	if got := macroText(e, "outTW"); got != "345.0pt" {
		t.Errorf("\\textwidth after the figure = %q, want 345.0pt (not restored)", got)
	}
}
