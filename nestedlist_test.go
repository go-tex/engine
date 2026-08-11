package engine

import "testing"

// lineLeftskip returns, for each top-level line box in the main vertical list,
// the width of its leading \leftskip glue paired with the glyphs on that line.
// Nested list environments must accumulate \leftskip, so deeper lines carry a
// wider leading glue.
func lineLeftskips(nodes []node) []struct {
	skip int
	text string
} {
	var out []struct {
		skip int
		text string
	}
	for _, n := range nodes {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox {
			continue
		}
		skip := 0
		for _, c := range b.list {
			if g, ok := c.(glueNode); ok {
				skip = g.spec.width
				break
			}
		}
		out = append(out, struct {
			skip int
			text string
		}{skip, mvlText([]node{b})})
	}
	return out
}

// A nested enumerate numbers each level with its own counter and format:
// level 1 = "1.", level 2 = "(a)", and the outer counter resumes untouched
// once the inner list closes.
func TestNestedEnumerateLabels(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{enumerate}\item Un\item Deux` +
		`\begin{enumerate}\item Deux-a\item Deux-b\end{enumerate}` +
		`\item Trois\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "1.Un2.Deux(a)Deux-a(b)Deux-b3.Trois"; got != want {
		t.Fatalf("nested enumerate typeset %q, want %q", got, want)
	}
}

// Cumulative \leftskip: level-2 items are indented twice as far as level-1
// items, and indentation unwinds so the item after the inner list returns to
// the level-1 indent.
func TestNestedEnumerateIndent(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{enumerate}\item Un\item Deux` +
		`\begin{enumerate}\item Deux-a\item Deux-b\end{enumerate}` +
		`\item Trois\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	lines := lineLeftskips(e.mvl)
	if len(lines) != 5 {
		t.Fatalf("expected 5 list lines, got %d: %+v", len(lines), lines)
	}
	const pt = 65536
	want := []struct {
		skip int
		text string
	}{
		{24 * pt, "1.Un"},
		{24 * pt, "2.Deux"},
		{48 * pt, "(a)Deux-a"},
		{48 * pt, "(b)Deux-b"},
		{24 * pt, "3.Trois"},
	}
	for i, w := range want {
		if lines[i].skip != w.skip || lines[i].text != w.text {
			t.Errorf("line %d = {skip %d, %q}, want {skip %d, %q}",
				i, lines[i].skip, lines[i].text, w.skip, w.text)
		}
	}
}

// A nested itemize picks a distinct bullet per level (level 1 = "•",
// level 2 = "--") and indents each level a further 24pt.
func TestNestedItemizeBullets(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{itemize}\item A\item B` +
		`\begin{itemize}\item B1\item B2\end{itemize}` +
		`\item C\end{itemize}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "•A•B--B1--B2•C"; got != want {
		t.Fatalf("nested itemize typeset %q, want %q", got, want)
	}
	lines := lineLeftskips(e.mvl)
	const pt = 65536
	want := []struct {
		skip int
		text string
	}{
		{24 * pt, "•A"},
		{24 * pt, "•B"},
		{48 * pt, "--B1"},
		{48 * pt, "--B2"},
		{24 * pt, "•C"},
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %+v", len(want), len(lines), lines)
	}
	for i, w := range want {
		if lines[i].skip != w.skip || lines[i].text != w.text {
			t.Errorf("line %d = {skip %d, %q}, want {skip %d, %q}",
				i, lines[i].skip, lines[i].text, w.skip, w.text)
		}
	}
}

// itemize nested inside enumerate (and the reverse) share the same cumulative
// \leftskip machinery, so a mixed nesting indents consistently regardless of
// which environment wraps which.
func TestMixedListNesting(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{enumerate}\item One` +
		`\begin{itemize}\item bullet\end{itemize}` +
		`\item Two\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "1.One•bullet2.Two"; got != want {
		t.Fatalf("mixed nesting typeset %q, want %q", got, want)
	}
	lines := lineLeftskips(e.mvl)
	const pt = 65536
	want := []int{24 * pt, 48 * pt, 24 * pt}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %+v", len(want), len(lines), lines)
	}
	for i, w := range want {
		if lines[i].skip != w {
			t.Errorf("line %d skip = %d, want %d (text %q)", i, lines[i].skip, w, lines[i].text)
		}
	}
}

// Deeper enumerate levels use roman-then-alpha formats: level 3 = "i.",
// level 4 = "A.".
func TestEnumerateDeepLevels(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\begin{enumerate}\item a` +
		`\begin{enumerate}\item b` +
		`\begin{enumerate}\item c` +
		`\begin{enumerate}\item d\end{enumerate}` +
		`\end{enumerate}\end{enumerate}\end{enumerate}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "1.a(a)bi.cA.d"; got != want {
		t.Fatalf("deep enumerate typeset %q, want %q", got, want)
	}
}
