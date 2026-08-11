package engine

import (
	"strings"
	"testing"
)

// \tag replaces an equation's number with a custom parenthesised label and does not
// consume an automatic number.
func TestEquationTag(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// Tags are plain text (read via readBraceName); a symbol like "$\star$" would
	// need a token-preserving read, which is out of scope.
	src := `\hsize=300pt
\begin{equation} a=b \tag{A} \label{eq:s} \end{equation}
\begin{equation} c=d \end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// The tagged equation used no number, so the next equation is (1), not (2).
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	if !strings.Contains(txt, "(1)") {
		t.Errorf("second equation should be (1); got %q", txt)
	}
	if strings.Contains(txt, "(2)") {
		t.Errorf("\\tag must not consume a number; got %q", txt)
	}
	if !strings.Contains(txt, "(A)") {
		t.Errorf("tagged equation should show (A); got %q", txt)
	}
	// \label on the tagged equation records the tag text.
	if got := e.labels["eq:s"]; got != "A" {
		t.Errorf("tagged label = %q, want \"A\"", got)
	}
}

// \tag* sets a bare number (no parentheses).
func TestEquationTagStar(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n\\begin{equation} x \\tag*{alt} \\end{equation}"); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	if !strings.Contains(txt, "alt") {
		t.Errorf("\\tag* text missing; got %q", txt)
	}
	if strings.Contains(txt, "(alt)") {
		t.Errorf("\\tag* must not parenthesise; got %q", txt)
	}
}

// \notag suppresses the number and does not consume one.
func TestEquationNotag(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{equation} a \notag \end{equation}
\begin{equation} b \end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	if !strings.Contains(txt, "(1)") || strings.Contains(txt, "(2)") {
		t.Errorf("\\notag should not consume a number; got %q", txt)
	}
}

// \tag inside align applies per row.
func TestAlignTag(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{align}
a &= b \\
c &= d \tag{*}
\end{align}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	// Row 1 numbered (1); row 2 tagged (*), consuming no number.
	if !strings.Contains(txt, "(1)") || !strings.Contains(txt, "(*)") {
		t.Errorf("align tag: got %q, want (1) and (*)", txt)
	}
	if strings.Contains(txt, "(2)") {
		t.Errorf("tagged align row must not consume a number; got %q", txt)
	}
}

// subequations numbers inner equations Na, Nb and restores the counter after.
func TestSubequations(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{equation} pre \end{equation}
\begin{subequations}
\begin{equation} a \label{eq:a} \end{equation}
\begin{equation} b \end{equation}
\end{subequations}
\begin{equation} post \end{equation}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	// pre=(1); sub=(2a),(2b); post=(3).
	for _, want := range []string{"(1)", "(2a)", "(2b)", "(3)"} {
		if !strings.Contains(txt, want) {
			t.Errorf("subequations numbering missing %q; got %q", want, txt)
		}
	}
	if got := e.labels["eq:a"]; got != "2a" {
		t.Errorf("sub-equation label = %q, want \"2a\"", got)
	}
}
