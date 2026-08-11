package engine

import (
	"strings"
	"testing"
)

// align numbers each row independently; two rows → (1) and (2), and \label on a row
// captures that row's number so \eqref resolves.
func TestAlignNumbering(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{align}
a &= b \label{eq:one} \\
c &= d
\end{align}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["eq:one"]; got != "1" {
		t.Errorf("eq:one = %q, want \"1\"", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "(1)") || !strings.Contains(text, "(2)") {
		t.Errorf("align numbers missing; chars %q want (1) and (2)", text)
	}
	if !hasMathNode(e.mvl) {
		t.Fatal("no math boxes placed for align")
	}
}

// align* suppresses all numbers; \nonumber suppresses one row's number in align.
func TestAlignStarAndNonumber(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n\\begin{align*}\nx &= y \\\\\nz &= w\n\\end{align*}"); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.Contains(b.String(), "(1)") {
		t.Errorf("align* must not number; got %q", b.String())
	}

	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run("\\hsize=300pt\n\\begin{align}\nx &= y \\nonumber \\\\\nz &= w\n\\end{align}"); err != nil {
		t.Fatal(err)
	}
	var b2 strings.Builder
	collectChars(e2.mvl, &b2)
	txt := b2.String()
	// \nonumber does not step the counter, so the first (suppressed) row has no
	// number and the second numbered row is (1), not (2).
	if !strings.Contains(txt, "(1)") {
		t.Errorf("numbered row should be (1); got %q", txt)
	}
	if strings.Contains(txt, "(2)") {
		t.Errorf("only one number expected (\\nonumber must not step); got %q", txt)
	}
}

// gather centres each row and numbers it; two rows → (1),(2).
func TestGather(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n\\begin{gather}\na=b \\\\\nc=d\n\\end{gather}"); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "(1)") || !strings.Contains(b.String(), "(2)") {
		t.Errorf("gather numbering; got %q", b.String())
	}
}

// eqnarray numbers each {rcl} line; the middle column is centred, and numbers run
// down the right.
func TestEqnarray(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{eqnarray}
a &=& b \label{eq:x} \\
c &<& d
\end{eqnarray}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["eq:x"]; got != "1" {
		t.Errorf("eq:x = %q, want \"1\"", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "(1)") || !strings.Contains(b.String(), "(2)") {
		t.Errorf("eqnarray numbering; got %q", b.String())
	}
}

// eqnarrayCols maps columns to the {rcl} pattern.
func TestEqnarrayCols(t *testing.T) {
	for j, want := range []byte{'r', 'c', 'l', 'r', 'c', 'l'} {
		if got := eqnarrayCols(j); got != want {
			t.Errorf("eqnarrayCols(%d) = %q, want %q", j, got, want)
		}
	}
}

// multline gives one number to the whole block (on the last line); intermediate
// lines carry none.
func TestMultline(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\begin{multline}
a+b+c+d \\
+ e + f + g \\
= h \label{eq:m}
\end{multline}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := e.labels["eq:m"]; got != "1" {
		t.Errorf("eq:m = %q, want \"1\"", got)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	txt := b.String()
	if !strings.Contains(txt, "(1)") {
		t.Errorf("multline should number the block (1); got %q", txt)
	}
	if strings.Contains(txt, "(2)") {
		t.Errorf("multline is one equation: no (2) expected; got %q", txt)
	}
	// Three lines contributed.
	if n := countHboxes(e.mvl); n < 3 {
		t.Errorf("multline should place >=3 lines, got %d", n)
	}
}

// multline* carries no number at all.
func TestMultlineStar(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\hsize=300pt\n\\begin{multline*}\nx \\\\ = y\n\\end{multline*}"); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if strings.Contains(b.String(), "(1)") {
		t.Errorf("multline* must not number; got %q", b.String())
	}
}

// countHboxes counts hbox line boxes directly in the main vertical list.
func countHboxes(nodes []node) int {
	n := 0
	for _, x := range nodes {
		if b, ok := x.(*boxNode); ok && b.kind == hbox {
			n++
		}
	}
	return n
}

// collectAlignBody splits rows at \\ and cells at &, keeps brace groups intact, and
// pulls \label/\nonumber out of the cell text.
func TestCollectAlignBody(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// Drive the collector directly by priming the base input then reading to \end{align}.
	e.base = []rune("a &= b \\nonumber \\label{k} \\\\ c &= d \\end{align}")
	e.bpos = 0
	rows := e.collectAlignBody("align")
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(rows))
	}
	if len(rows[0].cells) != 2 || strings.TrimSpace(rows[0].cells[0]) != "a" {
		t.Errorf("row0 cells = %q", rows[0].cells)
	}
	if !rows[0].nonumber {
		t.Error("row0 should be \\nonumber")
	}
	if len(rows[0].labels) != 1 || rows[0].labels[0] != "k" {
		t.Errorf("row0 labels = %v, want [k]", rows[0].labels)
	}
	// In "c &= d" the = belongs to the second cell (it is what aligns).
	if strings.TrimSpace(rows[1].cells[1]) != "= d" {
		t.Errorf("row1 cell1 = %q, want \"= d\"", rows[1].cells[1])
	}
}
