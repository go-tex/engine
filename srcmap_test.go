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
