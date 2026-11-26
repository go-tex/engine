package engine

import "testing"

// textspaceMathWidth sums the widths of the math nodes under nodes.
func textspaceMathWidth(nodes []node) int {
	w := 0
	for _, n := range nodes {
		switch v := n.(type) {
		case mathNode:
			w += v.width
		case *boxNode:
			w += textspaceMathWidth(v.list)
		}
	}
	return w
}

// go-tex/math v0.51.0 keeps the interword spaces inside \text (amstext sets the
// argument in an \hbox in text mode); v0.50.0 read it as \mathrm and dropped them,
// so \text{ is independent from } printed as "isindependentfrom" (arXiv 2407.19892).
// This pins that the engine reaches the maths layer with the blanks intact.
func TestTextKeepsItsSpaceThroughTheEngine(t *testing.T) {
	width := func(body string) int {
		t.Helper()
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run("\\hsize=300pt\n\\[ " + body + " \\]"); err != nil {
			t.Fatal(err)
		}
		return textspaceMathWidth(e.mvl)
	}
	glued, spaced := width(`\text{ab}`), width(`\text{a b}`)
	if spaced <= glued {
		t.Fatalf(`\text{a b} is %d sp wide, \text{ab} is %d: the space was dropped`, spaced, glued)
	}
	if two := width(`\text{a  b}`); two != spaced {
		t.Errorf(`a run of blanks is one space: %d, want %d`, two, spaced)
	}
}
