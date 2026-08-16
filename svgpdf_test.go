package engine

import (
	"bytes"
	"strings"
	"testing"

	"github.com/go-pdfkit/pdfkit"
)

// streamFor draws an SVG fragment on a pageH-tall page and returns the page's
// content stream, so a test can assert the exact imaging operators emitted.
func streamFor(t *testing.T, fragment string, pageH float64) string {
	t.Helper()
	doc := pdfkit.New(pdfkit.Options{})
	p := doc.AddPage(pdfkit.NewPageSize(200, pageH))
	drawSVGStream(p, fragment, pageH)
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatalf("write PDF: %v", err)
	}
	for _, s := range pdfStreams(t, buf.Bytes()) {
		if strings.Contains(s, " m\n") || strings.Contains(s, " re\n") || strings.Contains(s, "S\n") ||
			strings.HasSuffix(strings.TrimSpace(s), "S") || strings.HasSuffix(strings.TrimSpace(s), "f") {
			return s
		}
	}
	return ""
}

// wantOps asserts the content stream contains each operator line, in order.
func wantOps(t *testing.T, stream string, ops ...string) {
	t.Helper()
	at := 0
	for _, op := range ops {
		i := strings.Index(stream[at:], op)
		if i < 0 {
			t.Fatalf("stream lacks %q after offset %d:\n%s", op, at, stream)
		}
		at += i + len(op)
	}
}

// A stroked path maps SVG's y-down page coordinates onto PDF's y-up user space:
// a point (x, y) on a page of height H lands at (x, H−y). The stroke colour and
// width are selected before the path is painted with S (stroke, no fill).
func TestSVGStreamStrokedPath(t *testing.T) {
	s := streamFor(t, `<path d="M 10 20 L 30 40" stroke="red" stroke-width="2" fill="none"/>`, 100)
	wantOps(t, s, "1 0 0 RG", "2 w", "10 80 m", "30 60 l", "S")
	if strings.Contains(s, "\nf\n") {
		t.Errorf("fill=none must not fill:\n%s", s)
	}
}

// A path with both a fill and a stroke is painted with B (fill then stroke), and
// the two colours are selected independently (rg for fill, RG for stroke).
func TestSVGStreamFillAndStroke(t *testing.T) {
	s := streamFor(t, `<path d="M 0 0 L 10 0 L 10 10 Z" fill="blue" stroke="black"/>`, 50)
	wantOps(t, s, "0 0 1 rg", "0 0 0 RG", "0 50 m", "10 50 l", "10 40 l", "B")
}

// A shape with neither fill nor stroke paints nothing at all.
func TestSVGStreamNoPaint(t *testing.T) {
	if s := streamFor(t, `<path d="M 0 0 L 10 10" fill="none" stroke="none"/>`, 50); s != "" {
		t.Errorf("an unpainted path emitted operators:\n%s", s)
	}
}

// A <g> scope's presentation attributes are inherited by the shapes inside it,
// and its transform composes with the page mapping.
func TestSVGStreamGroupInheritance(t *testing.T) {
	s := streamFor(t, `<g stroke="#00ff00" stroke-width="3" fill="none" transform="translate(5,5)">`+
		`<path d="M 0 0 L 10 0"/></g>`, 100)
	wantOps(t, s, "0 1 0 RG", "3 w", "5 95 m", "15 95 l", "S")
}

// A group's state is scoped: a shape after the group closes is back to the
// inherited (initial) state, not the group's.
func TestSVGStreamScopePops(t *testing.T) {
	s := streamFor(t, `<g stroke="red" fill="none"><path d="M 0 0 L 1 1"/></g>`+
		`<path d="M 2 2 L 3 3" stroke="blue" fill="none"/>`, 10)
	wantOps(t, s, "1 0 0 RG", "S", "0 0 1 RG", "2 8 m", "3 7 l", "S")
}

// A rect is emitted as its four mapped corners, so a rotated coordinate map
// yields the true quadrilateral rather than an axis-aligned box.
func TestSVGStreamRect(t *testing.T) {
	s := streamFor(t, `<rect x="10" y="10" width="20" height="30" fill="black"/>`, 100)
	wantOps(t, s, "10 90 m", "30 90 l", "30 60 l", "10 60 l", "h", "f")
}

// A <line> is stroked between its two mapped endpoints.
func TestSVGStreamLine(t *testing.T) {
	s := streamFor(t, `<line x1="0" y1="0" x2="40" y2="20" stroke="black"/>`, 100)
	wantOps(t, s, "0 100 m", "40 80 l", "S")
}

// A polygon closes its outline; a polyline does not.
func TestSVGStreamPolyAndPolyline(t *testing.T) {
	poly := streamFor(t, `<polygon points="0,0 10,0 10,10" fill="black"/>`, 20)
	wantOps(t, poly, "0 20 m", "10 20 l", "10 10 l", "h", "f")
	line := streamFor(t, `<polyline points="0,0 10,0" fill="none" stroke="black"/>`, 20)
	wantOps(t, line, "0 20 m", "10 20 l", "S")
	if strings.Contains(line, "\nh\n") {
		t.Errorf("a polyline must not be closed:\n%s", line)
	}
}

// A circle is approximated by four cubic Béziers through its extreme points.
func TestSVGStreamCircle(t *testing.T) {
	s := streamFor(t, `<circle cx="50" cy="50" r="10" fill="black"/>`, 100)
	wantOps(t, s, "60 50 m", "c", "c", "c", "c", "h", "f")
}

// The contents of <defs>, <clipPath> and <mask> describe paint or clipping, not
// marks: they are parsed (so the stream stays balanced) but never drawn.
func TestSVGStreamSkipsNonDrawing(t *testing.T) {
	s := streamFor(t, `<defs><path d="M 0 0 L 9 9" fill="black"/></defs>`+
		`<clipPath id="c"><rect x="0" y="0" width="9" height="9" fill="black"/></clipPath>`, 50)
	if s != "" {
		t.Errorf("hidden subtree was drawn:\n%s", s)
	}
}

// A transform scales the stroke width with the coordinate system, as SVG
// specifies: a 2× map makes a 1pt pen 2pt wide on the page.
func TestSVGStreamStrokeWidthScales(t *testing.T) {
	s := streamFor(t, `<g transform="scale(2)"><path d="M 0 0 L 10 0" stroke="black" stroke-width="1" fill="none"/></g>`, 100)
	wantOps(t, s, "2 w", "0 100 m", "20 100 l", "S")
}

// A dash pattern, line cap and join reach the graphics state, scaled the same way.
func TestSVGStreamStrokeStyle(t *testing.T) {
	s := streamFor(t, `<path d="M 0 0 L 10 0" stroke="black" fill="none" stroke-dasharray="3 1"`+
		` stroke-linecap="round" stroke-linejoin="bevel"/>`, 10)
	wantOps(t, s, "1 J", "2 j", "[3 1] 0 d", "S")
}

// Presentation attributes may arrive in a style="" declaration instead.
func TestSVGStreamStyleAttribute(t *testing.T) {
	s := streamFor(t, `<path d="M 0 0 L 5 5" style="stroke:#0000ff;stroke-width:4;fill:none"/>`, 10)
	wantOps(t, s, "0 0 1 RG", "4 w", "S")
}

// Opacity becomes a PDF graphics state, and is reset once the shape is painted.
func TestSVGStreamOpacity(t *testing.T) {
	s := streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-opacity="0.5"/>`, 10)
	if !strings.Contains(s, "gs") {
		t.Errorf("stroke-opacity did not select an ExtGState:\n%s", s)
	}
}

// A matrix() transform is honoured, so an arbitrary coordinate map (what a
// drawing package emits for a rotated or skewed picture) lands correctly.
func TestSVGStreamMatrixTransform(t *testing.T) {
	// matrix(0,1,-1,0,0,0) rotates 90°: (10,0) → (0,10).
	s := streamFor(t, `<g transform="matrix(0,1,-1,0,0,0)"><path d="M 10 0 L 10 0" stroke="black" fill="none"/></g>`, 100)
	wantOps(t, s, "0 90 m")
}

// An unbalanced stream (a scope opened but never closed, as when a picture is
// truncated) still draws everything up to that point instead of failing.
func TestSVGStreamUnbalanced(t *testing.T) {
	s := streamFor(t, `<g stroke="black" fill="none"><path d="M 0 0 L 5 5"/>`, 10)
	wantOps(t, s, "0 10 m", "5 5 l", "S")
}

// An empty (or whitespace-only) stream draws nothing.
func TestSVGStreamEmpty(t *testing.T) {
	if s := streamFor(t, "   \n", 10); s != "" {
		t.Errorf("empty stream emitted operators:\n%s", s)
	}
}

// parseTransform composes the SVG transform functions the drivers rely on.
func TestParseTransformFunctions(t *testing.T) {
	cases := []struct {
		in     string
		x, y   float64
		wx, wy float64
	}{
		{"translate(3,4)", 1, 1, 4, 5},
		{"scale(2)", 3, 4, 6, 8},
		{"scale(2,-1)", 3, 4, 6, -4},
		{"rotate(90)", 1, 0, 0, 1},
		{"rotate(90,1,0)", 1, 0, 1, 0}, // the centre is fixed
		{"matrix(0,1,-1,0,5,6)", 1, 0, 5, 7},
		{"skewX(45)", 1, 1, 2, 1},
		{"skewY(45)", 1, 1, 1, 2},
		{"translate(1,0) scale(2)", 1, 1, 3, 2}, // composed left to right
	}
	for _, c := range cases {
		gx, gy := parseTransform(c.in).apply(c.x, c.y)
		if !closeTo(gx, c.wx) || !closeTo(gy, c.wy) {
			t.Errorf("%s applied to (%g,%g) = (%g,%g), want (%g,%g)", c.in, c.x, c.y, gx, gy, c.wx, c.wy)
		}
	}
}

// scaleFactor is the length scale of a map: exact for uniform ones.
func TestAffineScaleFactor(t *testing.T) {
	cases := []struct {
		m    affine
		want float64
	}{
		{identity(), 1},
		{affine{2, 0, 0, 2, 0, 0}, 2},
		{affine{0, 1, -1, 0, 0, 0}, 1},    // a rotation preserves length
		{affine{0, 0, 0, 0, 0, 0}, 0},     // a degenerate map has no scale
		{affine{2, 0, 0, -2, 0, 0}, 2},    // a flip is still a 2× scale
		{affine{4, 0, 0, 1, 0, 0}, 2},     // non-uniform: the geometric mean
		{parseTransform("scale(3)"), 3},   //
		{parseTransform("rotate(30)"), 1}, //
	}
	for _, c := range cases {
		if got := c.m.scaleFactor(); !closeTo(got, c.want) {
			t.Errorf("scaleFactor(%+v) = %g, want %g", c.m, got, c.want)
		}
	}
}

// parseDashArray reads a pattern and rejects "none" and negative lengths.
func TestParseDashArray(t *testing.T) {
	if got := parseDashArray("3 1.5,2"); len(got) != 3 || got[1] != 1.5 {
		t.Errorf("parseDashArray = %v", got)
	}
	if got := parseDashArray("none"); got != nil {
		t.Errorf("none must disable dashing, got %v", got)
	}
	if got := parseDashArray("3 -1"); got != nil {
		t.Errorf("a negative dash length must be rejected, got %v", got)
	}
	if got := scaleDash([]float64{2, 4}, 0.5); got[0] != 1 || got[1] != 2 {
		t.Errorf("scaleDash = %v", got)
	}
	if got := scaleDash(nil, 2); got != nil {
		t.Errorf("scaleDash of nothing = %v", got)
	}
}

// parseAlpha clamps to 0..1 and falls back when the value is unreadable.
func TestParseAlpha(t *testing.T) {
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"0.25", 0.25}, {"2", 1}, {"-1", 0}, {"", 0.7}, {"opaque", 0.7},
	} {
		if got := parseAlpha(c.in, 0.7); got != c.want {
			t.Errorf("parseAlpha(%q) = %g, want %g", c.in, got, c.want)
		}
	}
}

func closeTo(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// A degenerate shape constructs no path, so nothing is painted and no stray
// operator is left behind.
func TestSVGStreamDegenerateShapes(t *testing.T) {
	for _, frag := range []string{
		`<rect x="0" y="0" width="0" height="10" fill="black"/>`,
		`<rect x="0" y="0" width="10" height="-1" fill="black"/>`,
		`<circle cx="5" cy="5" r="0" fill="black"/>`,
		`<ellipse cx="5" cy="5" rx="4" ry="0" fill="black"/>`,
		`<polygon points="1,1" fill="black"/>`,
		`<polyline points="" fill="none" stroke="black"/>`,
		`<path d="" fill="black"/>`,
		`<path fill="black"/>`,
	} {
		if s := streamFor(t, frag, 20); s != "" {
			t.Errorf("%s painted something:\n%s", frag, s)
		}
	}
}

// A zero-width pen draws no stroke (SVG paints nothing for stroke-width 0), and a
// negative one is ignored so the inherited width stands.
func TestSVGStreamStrokeWidthEdges(t *testing.T) {
	if s := streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-width="0"/>`, 10); s != "" {
		t.Errorf("a zero-width pen painted:\n%s", s)
	}
	s := streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-width="-2"/>`, 10)
	wantOps(t, s, "1 w", "S") // the initial width, not −2
	s = streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-width="wide"/>`, 10)
	wantOps(t, s, "1 w", "S")
}

// An unreadable or inherit-valued paint leaves the inherited one in place; an
// unknown element and unknown attributes are ignored.
func TestSVGStreamUnknownValues(t *testing.T) {
	s := streamFor(t, `<g stroke="black" fill="none"><path d="M 0 0 L 5 5" stroke="chartreuse" `+
		`data-x="1" fill="inherit"/></g>`, 10)
	wantOps(t, s, "0 0 0 RG", "S") // "chartreuse" is not in the colour table
	if s := streamFor(t, `<foreignObject><b>hi</b></foreignObject>`, 10); s != "" {
		t.Errorf("an unknown element painted:\n%s", s)
	}
}

// fill-opacity, the shorthand opacity, a dash offset and a miter limit all reach
// the graphics state; an out-of-range miter limit is ignored.
func TestSVGStreamMoreAttributes(t *testing.T) {
	s := streamFor(t, `<path d="M 0 0 L 5 5" fill="black" fill-opacity="0.25"/>`, 10)
	if !strings.Contains(s, "gs") {
		t.Errorf("fill-opacity did not select an ExtGState:\n%s", s)
	}
	s = streamFor(t, `<g opacity="0.5"><path d="M 0 0 L 5 5" fill="black"/></g>`, 10)
	if !strings.Contains(s, "gs") {
		t.Errorf("opacity did not select an ExtGState:\n%s", s)
	}
	s = streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-dasharray="2"`+
		` stroke-dashoffset="1" stroke-miterlimit="8"/>`, 10)
	wantOps(t, s, "8 M", "[2] 1 d", "S")
	s = streamFor(t, `<path d="M 0 0 L 5 5" stroke="black" fill="none" stroke-miterlimit="0"/>`, 10)
	wantOps(t, s, "4 M", "S") // below 1 is not a miter limit; the initial one stands
}

// An ellipse with a rotated map still passes through its four mapped extremes.
func TestSVGStreamEllipseTransformed(t *testing.T) {
	s := streamFor(t, `<g transform="translate(10,10)"><ellipse cx="0" cy="0" rx="5" ry="2" fill="black"/></g>`, 20)
	wantOps(t, s, "15 10 m", "c", "h", "f")
}
