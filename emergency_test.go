package engine

import "testing"

// Even when the first Knuth-Plass pass finds no feasible breaking (here a big
// \parindent makes the opening line short with no stretch), the emergency pass
// must still wrap the paragraph into multiple lines rather than one long line.
func TestEmergencyLineBreak(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 40 * unity
	e.parindent = 20 * unity // forces an unjustifiable first line in pass 1
	e.baselineskip = 10 * unity
	if _, err := e.Run(`www www www www www www\par`); err != nil {
		t.Fatal(err)
	}
	lines := 0
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b.kind == hbox {
			lines++
		}
	}
	if lines < 2 {
		t.Fatalf("emergency pass should still wrap into ≥2 lines, got %d", lines)
	}
}
