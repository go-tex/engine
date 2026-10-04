// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// refOf renders a document and returns what \ref{key} resolved to.
func refOf(t *testing.T, src, key string) string {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return e.labels[key]
}

// \@float's [placement] lookahead must not EXPAND, because LaTeX's does not —
// \@ifnextchar looks at the next token with \futurelet. Expanding it runs a
// \begin{subfigure} that follows immediately: \begin reassigns \@currenvir to
// "subfigure", so \@float resumes believing it is not in a standard float
// environment, takes the inline path and never sets \@captype. The enclosing
// \caption then freezes \csname the\@captype\endcsname over an UNDEFINED
// \@captype and the label becomes the literal text "\the@captype" — which a \ref
// prints on the page: "as in Figure \the@captype".
//
// Nine of the 154 corpus papers did this, 31 times.
func TestFloatPlacementLookaheadDoesNotExpand(t *testing.T) {
	// ⛔ The subject is a TABLE, not a figure. The guard in TestCaptionWithout…
	// defaults a missing \@captype to "figure", so a figure comes out right
	// whether or not this is fixed — a first version of this test passed with the
	// lookahead put back, which is no test at all. A table must stay a table.
	//
	// The sub-panel is the FIRST thing in the float, so the body is still in
	// vertical mode and the lookahead meets \begin, not a letter.
	src := `\documentclass{article}\hsize=300pt
\usepackage{subcaption}
\begin{document}
\begin{figure}\caption{a figure first, to move the figure counter}\end{figure}
\begin{table}
 \begin{subtable}{0.3\linewidth}x\end{subtable}
 \caption{t}\label{t}
\end{table}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.refTypes["t"], "table"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
	if got, want := e.labels["t"], "1"; got != want {
		t.Errorf("number = %q, want %q (the TABLE counter, not the figure's)", got, want)
	}
	// An explicit [placement] is still found — the scan only stopped EXPANDING.
	src2 := `\documentclass{article}\hsize=300pt
\usepackage{subcaption}
\begin{document}
\begin{table}[t]
 \begin{subtable}{0.3\linewidth}x\end{subtable}
 \caption{t}\label{t}
\end{table}
\end{document}`
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(src2); err != nil {
		t.Fatal(err)
	}
	if got, want := e2.refTypes["t"], "table"; got != want {
		t.Errorf("with [t]: type = %q, want %q", got, want)
	}
}

// Every float path keeps its OWN reference type, on each placement and on both
// kinds. This pins existing behaviour rather than new: it is here because the
// leak above was first "fixed" by a guard in \caption that defaulted a missing
// \@captype to "figure", and that guard turned sidecap's SCtable into
// "Figure 3" — trading a visible macro name for a silently wrong number. The
// repo's own TestSCfigureFloatsAndDropsOptionals caught it; this says the same
// thing about the ordinary paths, so the next attempt at a floor has to pass
// both.
func TestCaptypeSetOnEveryFloatPath(t *testing.T) {
	// ⛔ Tables again, for the same reason: the guard's default is "figure", so
	// only a table can tell whether \@float set the type or the guard invented it.
	for _, place := range []string{"", "[t]", "[h]", "[ht]", "[H]", "[!htbp]"} {
		src := `\documentclass{article}\hsize=300pt
\begin{document}
\begin{table}` + place + `\caption{t}\label{t}\end{table}
\end{document}`
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("table%s: %v", place, err)
		}
		if got, want := e.refTypes["t"], "table"; got != want {
			t.Errorf("table%s: type = %q, want %q", place, got, want)
		}
		if got, want := e.labels["t"], "1"; got != want {
			t.Errorf("table%s: number = %q, want %q", place, got, want)
		}
	}
	// Both kinds in one document, each on the inline path, each keeping its own
	// counter: the failure this guards against numbered a table as a figure.
	src := `\documentclass{article}\hsize=300pt
\begin{document}
\begin{figure}[h]\caption{f}\label{f}\end{figure}
\begin{table}[h]\caption{t}\label{t}\end{table}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"f": "figure", "t": "table"} {
		if got := e.refTypes[k]; got != want {
			t.Errorf("type[%q] = %q, want %q", k, got, want)
		}
	}
	for _, k := range []string{"f", "t"} {
		if got := e.labels[k]; got != "1" {
			t.Errorf("label[%q] = %q, want \"1\"", k, got)
		}
	}
}

// The STARRED float has its own [placement] scan (\@dblfloat, twocolumn.go) and
// it needed the same treatment. The token after \begin{figure*} is routinely
// \begin{center}, and expanding it runs the centring environment there: its
// \begingroup lands before the float opens its own, so the matching
// \end{center} closes the FLOAT's group and takes \@captype with it — the
// restore trace shows it defined at group depth 3 and DELETED on the way back to
// 2, before the \caption.
//
// Every one of the corpus's remaining "\the@captype" leaks after the unstarred
// fix was a figure* in a two-column region: 31 occurrences over 9 papers became
// 24, then 12, and four papers came fully clean.
func TestStarredFloatPlacementLookaheadDoesNotExpand(t *testing.T) {
	// A table*, so the type cannot be right by accident.
	src := `\documentclass[twocolumn]{article}\hsize=300pt
\begin{document}
\begin{figure}\caption{moves the figure counter}\end{figure}
\begin{table*}
\begin{center}x\end{center}
\caption{wide}\label{t}
\end{table*}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := e.refTypes["t"], "table"; got != want {
		t.Errorf("type = %q, want %q", got, want)
	}
	if got, want := e.labels["t"], "1"; got != want {
		t.Errorf("number = %q, want %q (the TABLE counter)", got, want)
	}
	// A starred float's own [placement] is still read, and does not leak onto the
	// page — the reason doDblFloat consumes it at all.
	src2 := `\documentclass[twocolumn]{article}\hsize=300pt
\begin{document}
\begin{table*}[t]
\begin{center}x\end{center}
\caption{wide}\label{t}
\end{table*}
\end{document}`
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(src2); err != nil {
		t.Fatal(err)
	}
	if got, want := e2.refTypes["t"], "table"; got != want {
		t.Errorf("with [t]: type = %q, want %q", got, want)
	}
	var b strings.Builder
	collectChars(e2.mvl, &b)
	if strings.Contains(b.String(), "[t]") {
		t.Errorf("the [placement] leaked onto the page: %q", b.String())
	}
}
