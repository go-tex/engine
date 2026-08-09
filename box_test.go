package engine

import (
	"math"
	"testing"
)

func TestHpack(t *testing.T) {
	list := []Node{Char{W: 2, H: 1}, SetGlue{W: 1, Stretch: 1, Shrink: 0.5}, Char{W: 2, H: 1}}
	// natural
	nb := hpack(list, nil)
	if nb.W != 5 || nb.H != 1 {
		t.Errorf("natural hbox=%+v want W5 H1", nb)
	}
	// to width 6: stretch by 1 over stretch 1 → ratio 1; the glue sets to 1+1=2
	w := 6.0
	b := hpack(list, &w)
	if b.W != 6 {
		t.Errorf("W=%v want 6", b.W)
	}
	g := b.List[1].(SetGlue)
	if math.Abs(g.Set-2) > 1e-9 {
		t.Errorf("glue set=%v want 2", g.Set)
	}
}
func TestVpack(t *testing.T) {
	l1 := HBox{W: 6, H: 1, D: 0}
	l2 := HBox{W: 6, H: 1, D: 0}
	vb := vpack([]Node{l1, l2}, 2)
	// total: 1 (l1) + gap(2-0-1=1) + 1 (l2) = 3; last depth 0 → H=3, D=0
	if math.Abs(vb.H-3) > 1e-9 || vb.D != 0 {
		t.Errorf("vbox H=%v D=%v want 3,0", vb.H, vb.D)
	}
}
func TestBuildParagraph(t *testing.T) {
	items := para(6, 2, 1, 1, 0.5) // helper from linebreak_test
	p, ok := BuildParagraph(items, 8, 5, 10, 2)
	if !ok {
		t.Fatal("no paragraph")
	}
	if len(p.Lines) != 2 {
		t.Fatalf("lines=%d want 2", len(p.Lines))
	}
	// every line box is packed to the paragraph width
	for _, n := range p.Box.List {
		if hb, ok := n.(HBox); ok && math.Abs(hb.W-8) > 1e-9 {
			t.Errorf("line box W=%v want 8", hb.W)
		}
	}
}
