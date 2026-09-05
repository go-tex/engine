package engine

import (
	"strings"
	"testing"
)

// mvlText collects every character typeset in a node tree, ignoring glue/kern/
// rules — enough to read back the letters and digits a run produced.
func mvlText(nodes []node) string {
	var b []rune
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				b = append(b, c.ch)
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(nodes)
	return string(b)
}

// \label freezes the current \@currentlabel: a section's number, a subsection's
// dotted number, an enumerate item's counter — captured at \label time.
func TestLabelValues(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\section{A}\label{a}` +
			`\subsection{b}\label{b}` +
			`\section{C}\label{c}`); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{"a": "1", "b": "1.1", "c": "2"} {
		if got := e.labels[k]; got != want {
			t.Errorf("label %q = %q, want %q", k, got, want)
		}
	}
}

// \ref reproduces a stored label's text; an unknown key yields "??".
func TestRefResolves(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "1", "z": "2"}
	if _, err := e.Run(`\noindent\ref{a} and \ref{z} and \ref{missing}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "1and2and??" {
		t.Errorf("refs typeset %q, want %q", got, "1and2and??")
	}
}

// \eqref parenthesises the reference text.
func TestEqref(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = map[string]string{"eq": "3"}
	if _, err := e.Run(`\noindent\eqref{eq}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "(3)" {
		t.Errorf("eqref typeset %q, want %q", got, "(3)")
	}
}

// A forward \ref (used before its \label) resolves on the second pass, exactly as
// the two-pass compile carries the label table from the aux run into the render.
func TestForwardRefTwoPass(t *testing.T) {
	src := `\noindent\ref{r}\section{X}\label{r}`
	// Pass 1: labels start empty, so \ref{r} would be "??" here (discarded).
	aux := New()
	aux.LoadLaTeX()
	aux.SetFont(spMock{})
	if _, err := aux.Run(src); err != nil {
		t.Fatal(err)
	}
	if aux.labels["r"] != "1" {
		t.Fatalf("aux pass label r = %q, want 1", aux.labels["r"])
	}
	// Pass 2: with the aux labels, the forward \ref resolves to the section number.
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = aux.labels
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "11X" { // \ref→1, section number 1, title X
		t.Errorf("second pass typeset %q, want %q", got, "11X")
	}
}

// needsTwoPass detects when forward references (\label or \bibitem) require it.
func TestNeedsTwoPass(t *testing.T) {
	if !needsTwoPass([]byte(`x \label{a} y`)) {
		t.Error("needsTwoPass should detect \\label")
	}
	if !needsTwoPass([]byte(`\bibitem{a} An entry`)) {
		t.Error("needsTwoPass should detect \\bibitem")
	}
	if needsTwoPass([]byte(`\ref{a} only, no definitions`)) {
		t.Error("needsTwoPass should be false without \\label/\\bibitem")
	}
}

// \bibitem numbers entries and stores each number under its key, so a \cite (even
// one before the bibliography) resolves to the entry's number.
func TestBibitemLabels(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(
		`\begin{thebibliography}{9}` +
			`\bibitem{a}First reference.` +
			`\bibitem{b}Second reference.` +
			`\end{thebibliography}`); err != nil {
		t.Fatal(err)
	}
	if e.labels["a"] != "1" || e.labels["b"] != "2" {
		t.Errorf("bib labels = %q/%q, want 1/2", e.labels["a"], e.labels["b"])
	}
}

// \cite brackets and comma-joins the numbers of the cited entries; an unknown key
// resolves to "??".
func TestCite(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "1", "b": "2"}
	if _, err := e.Run(`\noindent\cite{a} \cite{a,b} \cite{x}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "[1][1,2][??]" { // spaces are glue, not chars
		t.Errorf("cites typeset %q, want %q", got, "[1][1,2][??]")
	}
}

// splitComma trims and drops empty keys.
func TestSplitComma(t *testing.T) {
	got := splitComma(" a , b ,,c ")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitComma = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("splitComma[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// \pageref names the page its \label fell on — not the label's own reference
// text. The two are easy to confuse because they agree on a one-page document,
// so this test puts the target three pages away from the reference to it.
func TestPagerefResolvesToPage(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
Target on page \pageref{far}, section \ref{far}.
\newpage x \newpage \section{Far}\label{far} here.
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.labelPages["far"]; got != 3 {
		t.Fatalf("label far resolved to page %d, want 3", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if text := b.String(); !strings.Contains(text, "page3") {
		t.Errorf("\\pageref did not typeset page 3: %q", text)
	}
}

// A \pageref whose label is unknown — or whose page never resolved — yields
// LaTeX's "??" rather than a wrong number.
func TestPagerefUnresolved(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labelPages = map[string]int{"a": 4, "unset": 0}
	if _, err := e.Run(`\noindent\pageref{a} and \pageref{unset} and \pageref{gone}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "4and??and??" {
		t.Errorf("pagerefs typeset %q, want %q", got, "4and??and??")
	}
}

// finalizeLabelPages leaves the table alone when the run declared no \label.
func TestFinalizeLabelPagesWithoutLabels(t *testing.T) {
	e := New()
	e.finalizeLabelPages()
	if e.labelPages != nil {
		t.Errorf("labelPages = %v, want nil when no label was declared", e.labelPages)
	}
}

// crossRefsAgree is what decides whether another pass is worth paying for: it
// must report disagreement for every table that can move, and only then.
func TestCrossRefsAgree(t *testing.T) {
	was := map[string]int{"a": 2}
	base := func() *Engine {
		return &Engine{
			labelPages:   map[string]int{"a": 2},
			tocSource:    []tocEntry{{page: 1}},
			tocEntries:   []tocEntry{{page: 1}},
			indexSource:  []indexEntry{{page: 5}},
			indexEntries: []indexEntry{{page: 5}},
		}
	}
	if !base().crossRefsAgree(was) {
		t.Error("identical tables should agree")
	}
	for _, c := range []struct {
		name string
		mut  func(*Engine)
	}{
		{"a label appearing", func(e *Engine) { e.labelPages["b"] = 3 }},
		{"a label moving", func(e *Engine) { e.labelPages["a"] = 9 }},
		{"a contents entry appearing", func(e *Engine) { e.tocEntries = nil }},
		{"a contents entry moving", func(e *Engine) { e.tocEntries[0].page = 7 }},
		{"an index entry appearing", func(e *Engine) { e.indexEntries = nil }},
		{"an index entry moving", func(e *Engine) { e.indexEntries[0].page = 7 }},
	} {
		e := base()
		c.mut(e)
		if e.crossRefsAgree(was) {
			t.Errorf("%s should force a rerun", c.name)
		}
	}
}

// \pageref{LastPage} — the "page 1 of N" a report or CV asks the lastpage
// package for — names the final page. The label is planted by \enddocument, so
// it works whether the package was loaded, inherited from a class, or absent.
func TestLastPageLabel(t *testing.T) {
	src := []byte(`\documentclass{article}
\usepackage{lastpage}
\begin{document}Page 1 of \pageref{LastPage}.
\newpage a \newpage b \newpage c\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.labelPages["LastPage"]; got != 4 {
		t.Errorf("LastPage resolved to page %d, want 4", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if text := b.String(); !strings.Contains(text, "of4") {
		t.Errorf("\\pageref{LastPage} did not typeset 4: %q", text)
	}
	// The page is what LastPage is for, and the only half this plants meaningfully:
	// real lastpage leaves \ref{LastPage} empty, so nothing should read ours.
	if _, ok := e.labelPages["LastPage"]; !ok {
		t.Error("LastPage has no page entry")
	}
}
