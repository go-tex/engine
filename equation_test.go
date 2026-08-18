package engine

import (
	"strings"
	"testing"
)

// collectChars walks a node tree gathering every charNode rune, so a test can read
// back literal text (like an equation number "(1)") that was typeset into boxes.
func collectChars(nodes []node, b *strings.Builder) {
	for _, n := range nodes {
		switch v := n.(type) {
		case charNode:
			b.WriteRune(rune(v.ch))
		case *charNode:
			b.WriteRune(rune(v.ch))
		case *boxNode:
			collectChars(v.list, b)
		case frameNode:
			if v.inner != nil {
				collectChars(v.inner.list, b)
			}
		}
	}
}

// hasMathNode reports whether a math box was placed anywhere in the tree.
func hasMathNode(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case mathNode:
			return true
		case *boxNode:
			if hasMathNode(v.list) {
				return true
			}
		}
	}
	return false
}

// The equation environment steps \c@equation, records the number as \@currentlabel
// (so \label captures it), renders the body as display math and sets a right-flushed
// "(N)". Two equations number 1 then 2.
func TestEquationNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{equation} x^2 \label{eq:a} \end{equation}
\begin{equation} y=1 \label{eq:b} \end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["eq:a"]; got != "1" {
		t.Errorf("eq:a label = %q, want \"1\"", got)
	}
	if got := e.labels["eq:b"]; got != "2" {
		t.Errorf("eq:b label = %q, want \"2\"", got)
	}
	if !hasMathNode(e.mvl) {
		t.Fatal("no math box placed for the equation")
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "(1)") || !strings.Contains(text, "(2)") {
		t.Errorf("equation numbers not typeset; got chars %q, want to contain (1) and (2)", text)
	}
}

// \nonumber inside an equation suppresses the printed number (the counter is still
// stepped by \begin{equation}, matching LaTeX).
func TestEquationNonumber(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n\\begin{equation} x \\nonumber \\end{equation}"); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.Contains(b.String(), "(1)") {
		t.Errorf("\\nonumber should suppress the number, but (1) was typeset: %q", b.String())
	}
}

// \eqref reproduces an equation's number, parenthesised, resolving through the label
// table just like \ref.
func TestEqrefResolves(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// two-pass would be needed for a forward ref; here the ref follows the label.
	src := `\hsize=300pt
\begin{equation} x \label{eq:a} \end{equation}
see \eqref{eq:a}.`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.refText("eq:a"); got != "1" {
		t.Fatalf("refText(eq:a) = %q, want \"1\"", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "(1)") {
		t.Errorf("\\eqref did not typeset (1); got %q", b.String())
	}
}

// \begin{equation*} renders the body as an UNNUMBERED display: a math box is placed,
// the environment is not skipped, and no "(N)" is printed. Undefined, the whole
// environment was skipped and the \frac/\sum inside dropped as unknown text.
func TestEquationStarUnnumbered(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt \begin{equation*} \frac{x}{y} + a \end{equation*}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.skippedCS["equation*"] != 0 {
		t.Fatal("equation* was skipped as an undefined environment")
	}
	if !hasMathNode(e.mvl) {
		t.Fatal("no math box placed for equation*")
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.ContainsAny(b.String(), "()") {
		t.Errorf("equation* printed a number %q; a starred equation is unnumbered", b.String())
	}
}
