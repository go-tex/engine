package engine

import "testing"

// firstLineLead returns the width of the leading box on the first line of the
// page (the \parindent box), or -1 if the first node is not a box.
func firstLineLead(e *Engine) int {
	for _, n := range e.mvl {
		if line, ok := n.(*boxNode); ok && line.kind == hbox {
			if len(line.list) > 0 {
				if lead, ok := line.list[0].(*boxNode); ok {
					return lead.width
				}
			}
			return -1
		}
	}
	return -1
}

func TestParagraphIndent(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 200 * unity
	e.parindent = 10 * unity
	if _, err := e.Run(`abc def\par`); err != nil {
		t.Fatal(err)
	}
	if got := firstLineLead(e); got != 10*unity {
		t.Errorf("indent box width %d sp want %d", got, 10*unity)
	}
}

func TestNoindent(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	e.hsize = 200 * unity
	if _, err := e.Run(`\noindent abc\par`); err != nil {
		t.Fatal(err)
	}
	// first node on the line should be a char, not an indent box
	if got := firstLineLead(e); got != -1 {
		t.Errorf("expected no indent box, got lead width %d", got)
	}
}

func TestParindentParam(t *testing.T) {
	e := New()
	got, _ := e.Run(`\parindent=15pt \message{\the\parindent}`)
	if trimNL(got) != "15.0pt" {
		t.Errorf("got %q want 15.0pt", trimNL(got))
	}
}
