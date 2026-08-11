package engine

import "testing"

// firstVboxOfWidth finds the first vbox of the given width reachable in a tree —
// the \parbox placed inline in a paragraph line.
func firstVboxOfWidth(nodes []node, width int) (*boxNode, bool) {
	for _, n := range nodes {
		if b, ok := n.(*boxNode); ok {
			if b.kind == vbox && b.width == width {
				return b, true
			}
			if r, ok := firstVboxOfWidth(b.list, width); ok {
				return r, true
			}
		}
	}
	return nil, false
}

// \parbox typesets its content to the given width as a vbox; content wider than
// the measure wraps onto several lines.
func TestParbox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// each "aaaa" is 20pt, so three of them wrap to three lines in a 20pt box.
	if _, err := e.Run(`\noindent\parbox{20pt}{aaaa aaaa aaaa}`); err != nil {
		t.Fatal(err)
	}
	pb, ok := firstVboxOfWidth(e.mvl, 20*unity)
	if !ok {
		t.Fatal("no 20pt parbox vbox placed")
	}
	lines := 0
	for _, n := range pb.list {
		if _, ok := n.(*boxNode); ok {
			lines++
		}
	}
	if lines < 2 {
		t.Errorf("parbox wrapped into %d lines, want ≥2", lines)
	}
}

// alignParbox anchors the vertical reference point per [pos]: t at the first
// line, b at the last (as packed), c centred.
func TestAlignParbox(t *testing.T) {
	mk := func() *boxNode {
		return &boxNode{
			kind: vbox, height: 40 * unity, depth: 0,
			list: []node{
				&boxNode{kind: hbox, height: 7 * unity},
				&boxNode{kind: hbox, height: 7 * unity},
			},
		}
	}
	if b := alignParbox(mk(), 'c'); b.height != 20*unity || b.depth != 20*unity {
		t.Errorf("[c] = %d/%d, want 20pt/20pt", b.height, b.depth)
	}
	if b := alignParbox(mk(), 't'); b.height != 7*unity || b.depth != 33*unity {
		t.Errorf("[t] = %d/%d, want 7pt/33pt", b.height, b.depth)
	}
	if b := alignParbox(mk(), 'b'); b.height != 40*unity || b.depth != 0 {
		t.Errorf("[b] = %d/%d, want 40pt/0", b.height, b.depth)
	}
}
