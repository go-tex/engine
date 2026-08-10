package engine

import "testing"

// A blank line between two chunks of text becomes \par, so they set as separate
// paragraphs; a single newline is just interword space.
func TestBlankLineIsPar(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	// "aa" and "bb" separated by a BLANK line ⇒ two paragraphs (two lines).
	if _, err := e.Run("aa\n\nbb"); err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			lines++
		}
	}
	if lines != 2 {
		t.Fatalf("blank line should split into 2 paragraphs, got %d lines", lines)
	}
}

func TestSingleNewlineIsSpace(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 500 * unity
	// single newline ⇒ one paragraph, one line, with an interword space between.
	if _, err := e.Run("aa\nbb"); err != nil {
		t.Fatal(err)
	}
	lines, spaces := 0, 0
	for _, n := range e.mvl {
		b, ok := n.(*boxNode)
		if !ok || b.kind != hbox {
			continue
		}
		lines++
		for _, c := range b.list {
			if _, ok := c.(glueNode); ok {
				spaces++
			}
		}
	}
	if lines != 1 {
		t.Fatalf("single newline should stay one paragraph, got %d lines", lines)
	}
	if spaces == 0 {
		t.Errorf("single newline should leave an interword space")
	}
}
