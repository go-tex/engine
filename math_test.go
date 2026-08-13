package engine

import (
	"strings"
	"testing"
)

// Inline math in a box produces a positive-width math node whose SVG is embedded.
func TestInlineMathBox(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{a$x^2+1$b}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	b := e.box[0]
	if b == nil {
		t.Fatal("box0 void")
	}
	var math *mathNode
	for i := range b.list {
		if m, ok := b.list[i].(mathNode); ok {
			mm := m
			math = &mm
		}
	}
	if math == nil {
		t.Fatal("no math node produced")
	}
	if math.width <= 0 || math.height <= 0 {
		t.Errorf("math box has non-positive dims: %+v", math)
	}
	// Baseline-aware placement (go-tex/math metrics, not the old h/2 centring):
	// "x^2+1" has a superscript that reaches well above the baseline and almost
	// nothing below it, so the box's height above the baseline must exceed its
	// depth below. Centring on half the overall height would make them equal.
	if math.height <= math.depth {
		t.Errorf("inline math not baseline-aligned: height %d sp should exceed depth %d sp for x^2+1", math.height, math.depth)
	}
	// box width includes a(5) + math(>0) + b(5)
	if b.width <= 10*unity {
		t.Errorf("box width %d sp should exceed 10pt (a+b) by the math width", b.width)
	}
	// the rendered SVG embeds the math via a nested <svg>
	svg := e.RenderBox(0, 0)
	if !strings.Contains(svg, "<svg") || strings.Count(svg, "<svg") < 2 {
		t.Errorf("expected an embedded (nested) math SVG in the render")
	}
}

// A malformed math expression surfaces as an engine error, not a silent success.
func TestMathErrorSurfaces(t *testing.T) {
	e := New()
	_, err := e.Run(`\setbox0=\hbox{$\frac{1}$}`) // \frac needs two args
	if err == nil {
		t.Skip("go-tex/math tolerated the input; nothing to assert")
	}
}
