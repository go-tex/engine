package engine

import "testing"

// \vtop's reference point is at the top: height = the first box's height, depth =
// everything below.
//
// Two 8pt/2pt boxes stacked in a vbox are separated by INTERLINE GLUE, as anywhere
// else: \baselineskip(12) − prevdepth(2) − height(8) = 2pt. Total 8+2+2+8+2 = 22pt,
// so \vbox is 20pt high with 2pt depth and \vtop 8pt high with 14pt depth.
//
// Checked against real TeX with the engine's own defaults (\baselineskip=12pt,
// \lineskip=1pt): ht0/dp0 = 20.0pt/2.0pt, ht1/dp1 = 8.0pt/14.0pt. (With
// \baselineskip=0pt — INITEX, no format — the same boxes give 18pt/2pt and 8pt/12pt,
// which is what this test used to assert.)
func TestVtopReferencePoint(t *testing.T) {
	body := `{\hbox{\vrule width3pt height8pt depth2pt}\hbox{\vrule width3pt height8pt depth2pt}}`
	e := New()
	e.Run(`\setbox0=\vtop` + body)
	if b := e.box[0]; b.height != 8*unity || b.depth != 14*unity {
		t.Errorf("vtop ht=%d dp=%d want 8pt/14pt", b.height, b.depth)
	}
	e2 := New()
	e2.Run(`\setbox0=\vbox` + body)
	if b := e2.box[0]; b.height != 20*unity || b.depth != 2*unity {
		t.Errorf("vbox ht=%d dp=%d want 20pt/2pt", b.height, b.depth)
	}
}
