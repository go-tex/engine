package engine

import "testing"

// An \hbox inside running text is an inline box on the line, not a paragraph
// break: the first (only) line must contain the 10pt inline box between the
// characters, and the paragraph stays a single line.
func TestInlineBoxInParagraph(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 500 * unity // wide: everything on one line
	if _, err := e.Run(`\noindent a\hbox{\kern10pt}b\par`); err != nil {
		t.Fatal(err)
	}
	// exactly one line
	var line *boxNode
	nLines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			line = b
			nLines++
		}
	}
	if nLines != 1 {
		t.Fatalf("expected 1 line, got %d", nLines)
	}
	// find the inline box among the line's children
	foundInline := false
	for _, n := range line.list {
		if b, ok := n.(*boxNode); ok && b.width == 10*unity {
			foundInline = true
		}
	}
	if !foundInline {
		t.Errorf("inline \\hbox{\\kern10pt} not found on the line: %+v", line.list)
	}
	// line natural content = a(5) + box(10) + b(5) = 20pt
	nat := 0
	for _, n := range line.list {
		switch c := n.(type) {
		case charNode:
			nat += c.width
		case *boxNode:
			nat += c.width
		}
	}
	if nat != 20*unity {
		t.Errorf("line natural content %d sp want %d", nat, 20*unity)
	}
}
