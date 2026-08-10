package engine

import "testing"

// \vtop's reference point is at the top: height = the first box's height, depth =
// everything below. Two 8pt/2pt boxes stacked ⇒ total 20pt; \vtop height 8pt,
// depth 12pt (vs \vbox height 18pt, depth 2pt).
func TestVtopReferencePoint(t *testing.T) {
	body := `{\hbox{\vrule width3pt height8pt depth2pt}\hbox{\vrule width3pt height8pt depth2pt}}`
	e := New()
	e.Run(`\setbox0=\vtop` + body)
	if b := e.box[0]; b.height != 8*unity || b.depth != 12*unity {
		t.Errorf("vtop ht=%d dp=%d want 8pt/12pt", b.height, b.depth)
	}
	e2 := New()
	e2.Run(`\setbox0=\vbox` + body)
	if b := e2.box[0]; b.height != 18*unity || b.depth != 2*unity {
		t.Errorf("vbox ht=%d dp=%d want 18pt/2pt", b.height, b.depth)
	}
}
