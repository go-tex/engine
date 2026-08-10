package engine

import "testing"

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
