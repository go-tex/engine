// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// newTypedRefEngine builds a LaTeX engine with the mock font, ready to Run source.
func newTypedRefEngine() *Engine {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	return e
}

// Running each numbered construct records its reference type (and, for the ones
// with a title, its name) under the following \label — the data \autoref / \cref /
// \nameref read back.
func TestRefMetaRecorded(t *testing.T) {
	e := newTypedRefEngine()
	src := `\hsize=300pt
\part{Foundations}\label{p}
\section{Intro}\label{s}
\subsection{Details}\label{ss}
\begin{equation} x \label{eq}\end{equation}
\begin{figure}\caption{A plot}\label{fig}\end{figure}
\begin{table}\caption{Some data}\label{tab}\end{table}
\newtheorem{theorem}{Theorem}
\begin{theorem}\label{thm} Body.\end{theorem}
\begin{enumerate}\item\label{it}\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	wantType := map[string]string{
		"p": "part", "s": "section", "ss": "subsection", "eq": "equation",
		"fig": "figure", "tab": "table", "thm": "theorem", "it": "item",
	}
	for k, want := range wantType {
		if got := e.refTypes[k]; got != want {
			t.Errorf("refType[%q] = %q, want %q", k, got, want)
		}
	}
	wantName := map[string]string{
		"p": "Foundations", "s": "Intro", "ss": "Details",
		"fig": "A plot", "tab": "Some data",
	}
	for k, want := range wantName {
		if got := e.refNames[k]; got != want {
			t.Errorf("refName[%q] = %q, want %q", k, got, want)
		}
	}
	// Constructs without a title record an empty name.
	for _, k := range []string{"eq", "thm", "it"} {
		if got := e.refNames[k]; got != "" {
			t.Errorf("refName[%q] = %q, want empty", k, got)
		}
	}
}

// autorefText prints "<Type> <number>", parenthesising equation numbers, and "??"
// for an unknown key.
func TestAutorefText(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "ss": "1.1", "eq": "2", "fig": "3", "tab": "4", "thm": "5", "p": "II", "it": "6", "x": "7"}
	e.refTypes = map[string]string{"s": "section", "ss": "subsection", "eq": "equation", "fig": "figure", "tab": "table", "thm": "theorem", "p": "part", "it": "item"}
	cases := map[string]string{
		"s": "Section 1", "ss": "Subsection 1.1", "eq": "Equation (2)",
		"fig": "Figure 3", "tab": "Table 4", "thm": "Theorem 5",
		"p": "Part II", "it": "item 6",
		"x":       "7",  // known number, untyped: bare number
		"missing": "??", // unknown key
	}
	for key, want := range cases {
		if got := e.autorefText(key); got != want {
			t.Errorf("autorefText(%q) = %q, want %q", key, got, want)
		}
	}
}

// crefText / crefOne print cleveref's lowercase (\cref) and capitalised (\Cref)
// abbreviations, parenthesising equation numbers.
func TestCrefSingle(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "ss": "1.1", "eq": "2", "fig": "3", "tab": "4", "thm": "5", "p": "II", "it": "6", "x": "7"}
	e.refTypes = map[string]string{"s": "section", "ss": "subsection", "eq": "equation", "fig": "figure", "tab": "table", "thm": "theorem", "p": "part", "it": "item"}
	lower := map[string]string{
		"s": "section 1", "ss": "subsection 1.1", "eq": "eq. (2)",
		"fig": "fig. 3", "tab": "tab. 4", "thm": "thm. 5", "p": "part II", "it": "item 6",
		"x": "7", "missing": "??",
	}
	upper := map[string]string{
		"s": "Section 1", "ss": "Subsection 1.1", "eq": "Eq. (2)",
		"fig": "Fig. 3", "tab": "Tab. 4", "thm": "Thm. 5", "p": "Part II", "it": "Item 6",
		"x": "7", "missing": "??",
	}
	for key, want := range lower {
		if got := e.crefText([]string{key}, false); got != want {
			t.Errorf("\\cref{%s} = %q, want %q", key, got, want)
		}
	}
	for key, want := range upper {
		if got := e.crefText([]string{key}, true); got != want {
			t.Errorf("\\Cref{%s} = %q, want %q", key, got, want)
		}
	}
}

// A multi-key \cref names the type once (plural) and joins the numbers "a and b" /
// "a, b and c"; equations parenthesise each number.
func TestCrefMultiKey(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"a": "1", "b": "2", "c": "3", "e1": "1", "e2": "2", "u": "9"}
	e.refTypes = map[string]string{"a": "section", "b": "section", "c": "section", "e1": "equation", "e2": "equation"}
	cases := []struct {
		keys    []string
		capital bool
		want    string
	}{
		{[]string{"a", "b"}, false, "sections 1 and 2"},
		{[]string{"a", "b"}, true, "Sections 1 and 2"},
		{[]string{"a", "b", "c"}, false, "sections 1, 2 and 3"},
		{[]string{"e1", "e2"}, false, "eqs. (1) and (2)"},
		{[]string{"e1", "e2"}, true, "Eqs. (1) and (2)"},
		{[]string{"u", "u"}, false, "9 and 9"}, // untyped: numbers only
	}
	for _, c := range cases {
		if got := e.crefText(c.keys, c.capital); got != c.want {
			t.Errorf("crefText(%v, %v) = %q, want %q", c.keys, c.capital, got, c.want)
		}
	}
	// Empty key list resolves to "??".
	if got := e.crefText(nil, false); got != "??" {
		t.Errorf("crefText(nil) = %q, want %q", got, "??")
	}
}

// \nameref prints the recorded title; an unknown key or a nameless target yields "??".
func TestNamerefText(t *testing.T) {
	e := newTypedRefEngine()
	e.refNames = map[string]string{"s": "Introduction", "eq": ""}
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\nameref{s}|\nameref{eq}|\nameref{missing}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "Introduction|??|??" {
		t.Errorf("nameref typeset %q, want %q", got, "Introduction|??|??")
	}
}

// End-to-end: the typed references typeset the expected characters into the main
// vertical list (spaces are glue, so they do not appear among the charNodes).
func TestTypedRefsTypeset(t *testing.T) {
	e := newTypedRefEngine()
	e.labels = map[string]string{"s": "1", "eq": "2", "thm": "3"}
	e.refTypes = map[string]string{"s": "section", "eq": "equation", "thm": "theorem"}
	e.refNames = map[string]string{"s": "Intro"}
	if _, err := e.Run(`\noindent\autoref{s}|\cref{eq}|\Cref{thm}|\nameref{s}`); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	// "Section 1" | "eq. (2)" | "Thm. 3" | "Intro" — inter-word spaces are glue.
	if got, want := b.String(), "Section1|eq.(2)|Thm.3|Intro"; got != want {
		t.Errorf("typeset %q, want %q", got, want)
	}
}

// A forward \autoref (before its \label) resolves on the second pass, exactly as
// the two-pass compile carries labels/refTypes/refNames from the aux run.
func TestForwardTypedRefTwoPass(t *testing.T) {
	src := `\hsize=300pt\noindent\autoref{s} then \nameref{s}.\section{Preliminaries}\label{s}`
	aux := newTypedRefEngine()
	if _, err := aux.Run(src); err != nil {
		t.Fatal(err)
	}
	if aux.refTypes["s"] != "section" || aux.refNames["s"] != "Preliminaries" {
		t.Fatalf("aux meta = %q/%q, want section/Preliminaries", aux.refTypes["s"], aux.refNames["s"])
	}
	// Second pass with the carried maps: the forward \autoref/\nameref resolve.
	e := newTypedRefEngine()
	e.labels = aux.labels
	e.refTypes = aux.refTypes
	e.refNames = aux.refNames
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	// \autoref{s}→"Section 1", \nameref{s}→"Preliminaries", then the heading "1Preliminaries".
	if got, want := b.String(), "Section1thenPreliminaries.1Preliminaries"; got != want {
		t.Errorf("second pass typeset %q, want %q", got, want)
	}
}

// The reference primitives must not panic on a missing brace or an empty key; each
// then yields "??" (or the untyped fallback), as \ref does.
func TestTypedRefErrorBranches(t *testing.T) {
	e := newTypedRefEngine()
	// Missing brace group: readBraceName returns "" and the next token is left alone.
	if _, err := e.Run(`\noindent\autoref x\cref y\Cref z\nameref w`); err != nil {
		t.Fatal(err)
	}
	// Empty key group.
	e2 := newTypedRefEngine()
	if _, err := e2.Run(`\noindent\autoref{}\cref{}\Cref{}\nameref{}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e2.mvl); got != "????????" {
		t.Errorf("empty-key typeset %q, want %q", got, "????????")
	}
	// recordRefMeta lazily allocates its maps.
	e3 := newTypedRefEngine()
	e3.recordRefMeta("k")
	if e3.refTypes == nil || e3.refNames == nil {
		t.Error("recordRefMeta did not allocate maps")
	}
}

// joinAnd formats 0, 1, 2 and 3+ elements without an Oxford comma.
func TestJoinAnd(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"1"}, "1"},
		{[]string{"1", "2"}, "1 and 2"},
		{[]string{"1", "2", "3"}, "1, 2 and 3"},
		{[]string{"1", "2", "3", "4"}, "1, 2, 3 and 4"},
	}
	for _, c := range cases {
		if got := joinAnd(c.in); got != c.want {
			t.Errorf("joinAnd(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
