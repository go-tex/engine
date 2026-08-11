package engine

import (
	"errors"
	"testing"
)

// charLine walks a node tree and returns the source line stamped on the first
// occurrence of each character — enough to check glyphs carry their origin line.
func charLine(nodes []node) map[rune]int {
	m := map[rune]int{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				if _, ok := m[c.ch]; !ok {
					m[c.ch] = c.srcLine
				}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(nodes)
	return m
}

// Characters carry the source line they were typed on: a single newline is an
// interword space, so A/B/C on consecutive lines stay one paragraph but keep their
// distinct line numbers.
func TestCharSourceLines(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\noindent A\nB\nC"); err != nil {
		t.Fatal(err)
	}
	got := charLine(e.mvl)
	for ch, want := range map[rune]int{'A': 1, 'B': 2, 'C': 3} {
		if got[ch] != want {
			t.Errorf("glyph %q source line = %d, want %d", ch, got[ch], want)
		}
	}
}

// The packed line box inherits the source line of its first glyph.
func TestBoxSourceLine(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run("\\noindent hello"); err != nil {
		t.Fatal(err)
	}
	// A paragraph contributes its line boxes (hboxes) to the main vertical list.
	var line *boxNode
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok {
			line = b
			break
		}
	}
	if line == nil {
		t.Fatal("no line box on the main vertical list")
	}
	if line.srcLine != 1 {
		t.Errorf("line box srcLine = %d, want 1", line.srcLine)
	}
}

// An undefined control sequence fails with a SourceError pointing at its line and
// column, and the location survives through the error interface.
func TestSourceErrorPosition(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// \oops sits at column 0 of line 3 (two leading newlines).
	_, err := e.Run("\n\n\\oops")
	if err == nil {
		t.Fatal("expected an error for the undefined command")
	}
	var se SourceError
	if !errors.As(err, &se) {
		t.Fatalf("error is %T, want SourceError", err)
	}
	if se.Line != 3 || se.Col != 0 {
		t.Errorf("error at %d:%d, want 3:0", se.Line, se.Col)
	}
	if got, want := se.Error(), "texengine: 3:1: Undefined control sequence \\oops"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

// SourceSpans ties rendered glyphs back to source lines, and the lookups map a
// click → line (inverse) and a line → its output rects (forward).
func TestSourceSpans(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// "Alpha" is on source line 2, "Bravo" on line 4 (blank line 3 = \par).
	if _, err := e.Run("\\documentclass{article}\\begin{document}\n\\noindent Alpha\n\nBravo\n\\end{document}"); err != nil {
		t.Fatal(err)
	}
	pages := e.SourceSpans(20)
	if len(pages) != 1 {
		t.Fatalf("pages = %d, want 1", len(pages))
	}
	spans := pages[0]
	// Every glyph is tagged with either line 2 or line 4.
	byLine := map[int]int{}
	for _, s := range spans {
		byLine[s.Line]++
	}
	if byLine[2] != 5 || byLine[4] != 5 {
		t.Errorf("glyphs per line = %v, want 5 on line 2 and 5 on line 4", byLine)
	}
	// Inverse: a click at the centre of a line-2 glyph resolves to line 2.
	center := func(want int) int {
		for _, s := range spans {
			if s.Line == want {
				return LineAt(spans, s.X+s.W/2, s.Y+s.H/2)
			}
		}
		return -1
	}
	if got := center(2); got != 2 {
		t.Errorf("LineAt(line-2 glyph) = %d, want 2", got)
	}
	if got := center(4); got != 4 {
		t.Errorf("LineAt(line-4 glyph) = %d, want 4", got)
	}
	// A click far from any glyph maps to no line.
	if got := LineAt(spans, 5000, 5000); got != 0 {
		t.Errorf("LineAt(empty area) = %d, want 0", got)
	}
	// Forward: line 2 highlights its five glyph rects.
	rects := RectsForLine(spans, 2)
	if len(rects) != 5 {
		t.Errorf("RectsForLine(2) = %d rects, want 5", len(rects))
	}
	for _, r := range rects {
		if r.Line != 2 {
			t.Errorf("RectsForLine(2) returned a span from line %d", r.Line)
		}
	}
}

// The SVG output groups glyphs under <g data-l="N"> matching their source line.
func TestSVGDataLine(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(dotFont{}) // a font whose glyphs have a non-empty path so <path>s emit
	if _, err := e.Run("\\documentclass{article}\\begin{document}\n\\noindent ab\n\ncd\n\\end{document}"); err != nil {
		t.Fatal(err)
	}
	svg := e.RenderPages(20)
	joined := ""
	for _, p := range svg {
		joined += p
	}
	for _, want := range []string{`data-l="2"`, `data-l="4"`} {
		if !contains(joined, want) {
			t.Errorf("SVG missing %s", want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// lineColAt maps a rune offset to a 1-based line and 0-based column.
func TestLineColAt(t *testing.T) {
	e := New()
	e.base = []rune("ab\ncd\nef")
	e.buildLineStarts()
	cases := []struct {
		pos, line, col int
	}{
		{0, 1, 0}, {1, 1, 1}, {2, 1, 2}, // "ab\n"
		{3, 2, 0}, {4, 2, 1}, // "cd"
		{6, 3, 0}, {7, 3, 1}, // "ef"
	}
	for _, c := range cases {
		if l, col := e.lineColAt(c.pos); l != c.line || col != c.col {
			t.Errorf("lineColAt(%d) = %d:%d, want %d:%d", c.pos, l, col, c.line, c.col)
		}
	}
}
