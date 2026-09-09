// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/go-pdfkit/pdfkit"
)

// renderPictureBody compiles a picture environment and returns the SVG of every
// page, where the drawing commands leave their marks.
func renderPictureBody(t *testing.T, body string) string {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	src := `\documentclass{article}\begin{document}\setlength{\unitlength}{1mm}` +
		`\begin{picture}(100,60)` + body + `\end{picture}\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	return strings.Join(e.RenderPages(72), "")
}

// Undefined, a picture command is worse than absent: it leaves its own
// coordinates on the page as prose. Measured against tectonic on a picture with
// \framebox, \circle, \line, \vector and text, the reference drew all five and
// this engine drew none — only "10", "(1,0)25", "(1,1)30" scattered through the
// text. They draw now, and the page's ink box is the reference's to the pixel
// (476x712 at 300dpi).
func TestPictureCommandsDrawInsteadOfLeaking(t *testing.T) {
	svg := renderPictureBody(t, `\put(5,5){\line(1,0){20}}`)
	if strings.Contains(svg, "(1,0)") || strings.Contains(svg, ">20<") {
		t.Errorf("the coordinates leaked into the page:\n%s", svg)
	}
	if !strings.Contains(svg, "<path") {
		t.Errorf("no path drawn:\n%s", svg)
	}
}

// \line's third argument is the line's HORIZONTAL extent, not its length — except
// on a vertical line, where it is the vertical one. That is ltpictur's own rule
// (\@sline / \@hline / \@vline), and the reason \line(2,1){20} and \line(1,1){20}
// end at the same x. Measured against tectonic at 300dpi, both are 237px wide
// where the reference gives 236-238.
func TestPictureLineTo(t *testing.T) {
	cases := []struct {
		xarg, yarg     int
		length, dx, dy float64
	}{
		{1, 0, 20, 20, 0},     // horizontal, rightwards
		{-1, 0, 20, -20, 0},   // horizontal, leftwards
		{0, 1, 20, 0, -20},    // vertical, up (SVG y grows down)
		{0, -1, 20, 0, 20},    // vertical, down
		{1, 1, 20, 20, -20},   // 45 degrees: the rise equals the run
		{2, 1, 20, 20, -10},   // half the slope, the SAME horizontal extent
		{-1, 1, 15, -15, -15}, // leftwards and UP: the run flips, and up is -y in SVG
		{1, -2, 10, 10, 20},   // downwards
	}
	for _, c := range cases {
		dx, dy := pictureLineTo(c.xarg, c.yarg, c.length)
		if math.Abs(dx-c.dx) > 1e-9 || math.Abs(dy-c.dy) > 1e-9 {
			t.Errorf("line(%d,%d){%g} = (%g,%g), want (%g,%g)",
				c.xarg, c.yarg, c.length, dx, dy, c.dx, c.dy)
		}
	}
}

// LaTeX has exactly TWO arrowheads, one per line font, so \vector's head steps
// rather than scaling: 4.08pt long and 2.88pt across at \thinlines, 6.00pt and
// 3.84pt at \thicklines, measured off tectonic at 300dpi. The straight line
// through the two reproduces both.
func TestArrowHeadMatchesTheTwoLaTeXSizes(t *testing.T) {
	for _, c := range []struct{ w, long, half float64 }{
		{0.4, 4.08, 1.44},
		{0.8, 6.00, 1.92},
	} {
		long, half := arrowHeadSize(c.w)
		if math.Abs(long-c.long) > 0.01 || math.Abs(half-c.half) > 0.01 {
			t.Errorf("arrowHeadSize(%g) = %.3f/%.3f, want %.3f/%.3f", c.w, long, half, c.long, c.half)
		}
	}
	// A hairline must still carry a head rather than a negative one.
	if long, half := arrowHeadSize(0.01); long <= 0 || half <= 0 {
		t.Errorf("arrowHeadSize(0.01) = %.3f/%.3f, want positive", long, half)
	}
}

// \oval's [part] keeps one half or quarter; no bracket keeps the whole.
func TestOvalPart(t *testing.T) {
	for in, want := range map[string]int{
		"": 0, "t": 1, "b": 2, "l": 4, "r": 8,
		"tr": 1 | 8, "bl": 2 | 4, "TL": 1 | 4, "x": 0,
	} {
		if got := ovalPart(in); got != want {
			t.Errorf("ovalPart(%q) = %d, want %d", in, got, want)
		}
	}
}

// Every drawing command reaches the page, and \circle* fills where \circle
// strokes.
func TestEachPictureCommandDraws(t *testing.T) {
	for _, c := range []struct{ name, body, want string }{
		{"line", `\put(5,5){\line(1,1){20}}`, "<path"},
		{"vector", `\put(5,5){\vector(1,0){20}}`, "<path"},
		{"circle", `\put(20,20){\circle{15}}`, `<circle`},
		{"disc", `\put(20,20){\circle*{10}}`, `fill="black" stroke="none"`},
		{"oval", `\put(50,30){\oval(20,14)}`, "<path"},
		{"oval part", `\put(50,30){\oval(20,14)[t]}`, "<path"},
		{"qbezier", `\qbezier(5,50)(20,58)(35,50)`, " Q "},
		{"framebox", `\put(0,0){\framebox(60,40){}}`, "<rect"},
	} {
		svg := renderPictureBody(t, c.body)
		if !strings.Contains(svg, c.want) {
			t.Errorf("%s: %q is not in the page:\n%s", c.name, c.want, svg)
		}
	}
}

// \linethickness, \thinlines and \thicklines reach the stroke.
func TestPictureLineThickness(t *testing.T) {
	thin := renderPictureBody(t, `\thinlines\put(5,5){\line(1,0){20}}`)
	thick := renderPictureBody(t, `\thicklines\put(5,5){\line(1,0){20}}`)
	set := renderPictureBody(t, `\linethickness{2pt}\put(5,5){\line(1,0){20}}`)
	if !strings.Contains(thin, `stroke-width="0.4"`) {
		t.Errorf("\\thinlines is not 0.4pt:\n%s", thin)
	}
	if !strings.Contains(thick, `stroke-width="0.8"`) {
		t.Errorf("\\thicklines is not 0.8pt:\n%s", thick)
	}
	if !strings.Contains(set, `stroke-width="2"`) {
		t.Errorf("\\linethickness{2pt} did not reach the stroke:\n%s", set)
	}
}

// An unknown path command must not stop the PDF driver. 'a', SVG's elliptical
// arc, is the one that occurs: cmd() advances past the letter but an operator
// that reads no operand leaves its numbers in place, so more() stays true, cmd()
// finds no letter and returns the same command, and the loop never advances. The
// driver spun forever on a path it merely did not understand.
func TestUnknownPathCommandDoesNotSpin(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		doc := pdfkit.New(pdfkit.Options{})
		p := doc.AddPage(pdfkit.NewPageSize(200, 200))
		buildSVGPath(p, "M 0 0 A 5 5 0 0 1 10 10 L 20 20", identity(), 0, 200)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("buildSVGPath did not return on a path carrying an arc")
	}
}

// And the segments it CAN read are still drawn: skipping an arc's operands loses
// that segment, not the rest of the path.
func TestUnknownPathCommandKeepsWhatFollows(t *testing.T) {
	stream := streamFor(t, `<path d="M 0 0 A 5 5 0 0 1 10 10 L 20 20" stroke="#000" fill="none"/>`, 200)
	wantOps(t, stream, " m", " l")
}
