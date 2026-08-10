package engine

import "testing"

// \char<n> typesets the glyph with that character code, measured like a literal.
func TestCharByCode(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	// \char65 = 'A'; in a box it measures the same as a literal letter (5pt)
	e.Run(`\setbox0=\hbox{\char65\char66}`)
	if b := e.box[0]; b == nil || b.width != 10*unity {
		t.Fatalf("\\char box width %v want 10pt", e.box[0])
	}
	// verify the runes landed as 'A','B'
	b := e.box[0]
	var got string
	for _, n := range b.list {
		if c, ok := n.(charNode); ok {
			got += string(c.ch)
		}
	}
	if got != "AB" {
		t.Errorf("chars = %q want AB", got)
	}
}
