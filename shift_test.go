package engine

import (
	"strings"
	"testing"
)

// \raise lifts an inner box: the outer hbox's height grows by the raise and its
// depth is unaffected; \lower does the reverse.
func TestRaiseLowerDimensions(t *testing.T) {
	e := New()
	// inner box: height 4pt, depth 0. Raise by 3pt ⇒ outer height 7pt, depth 0.
	e.Run(`\setbox0=\hbox{\raise3pt\hbox{\vrule width2pt height4pt depth0pt}}`)
	if b := e.box[0]; b.height != 7*unity || b.depth != 0 {
		t.Errorf("raise: ht=%d dp=%d want 7pt/0", b.height, b.depth)
	}
	// Lower by 3pt ⇒ height 1pt, depth 3pt.
	e2 := New()
	e2.Run(`\setbox0=\hbox{\lower3pt\hbox{\vrule width2pt height4pt depth0pt}}`)
	if b := e2.box[0]; b.height != 1*unity || b.depth != 3*unity {
		t.Errorf("lower: ht=%d dp=%d want 1pt/3pt", b.height, b.depth)
	}
}

// \raise moves the glyph up in the rendered SVG (smaller y).
func TestRaiseRendersHigher(t *testing.T) {
	e := New()
	e.Run(`\setbox0=\hbox{\raise5pt\hbox{\vrule width2pt height4pt}}`)
	svg := e.RenderBox(0, 0)
	// outer box height 9pt; baseline at y=9; inner box raised 5 ⇒ its baseline at
	// y=4; the 4pt-tall rule spans y=0..4.
	if !strings.Contains(svg, `<rect x="0" y="0" width="2" height="4"/>`) {
		t.Errorf("raised rule not at expected position:\n%s", svg)
	}
}
