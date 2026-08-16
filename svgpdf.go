// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"

	"github.com/go-pdfkit/pdfkit"
)

// This file draws the page's \special stream (see special.go) into the PDF. The
// stream is an SVG fragment: the SVG driver writes it out verbatim, and here it
// is interpreted into go-pdfkit's imaging operators so both drivers show the same
// marks. It is deliberately a *stream* interpreter rather than a document one —
// a drawing package emits scopes that open in one special and close in another,
// so the whole page's literals are parsed as a single element sequence.
//
// Supported: <g> scopes (transform, presentation attributes), <path>, <rect>,
// <line>, <circle>, <ellipse>, <polygon>, <polyline>; fill and stroke colour
// (named, #rgb/#rrggbb, rgb(), "none"), stroke-width, stroke-opacity/
// fill-opacity/opacity, stroke-dasharray, stroke-linecap/linejoin/miterlimit,
// and the transform functions translate/scale/rotate/matrix/skewX/skewY.
// Ignored (drawn by nobody, as an unsupporting driver would): gradients and
// other paint servers, <defs>/<clipPath>/<mask> contents, <text>, <image>,
// <use>, filters, and clipping.
//
// Coordinates: the literals are in page points with SVG's y-down origin at the
// top-left corner; PDF user space is y-up from the bottom-left, so a point (x,y)
// lands at (x, pageH−y) — the mapping the rest of the drivers already use.

// svgPaint is a resolved paint: a colour, or none (nothing is drawn).
type svgPaint struct {
	color uint32
	none  bool
}

// svgState is the inherited graphics state of one element scope.
type svgState struct {
	m           affine
	fill        svgPaint
	stroke      svgPaint
	strokeWidth float64
	fillAlpha   float64
	strokeAlpha float64
	dash        []float64
	dashPhase   float64
	lineCap     int
	lineJoin    int
	miterLimit  float64
	hidden      bool // inside <defs>/<clipPath>/<mask>: parsed but not painted
}

// defaultSVGState is SVG's initial graphics state: black fill, no stroke.
func defaultSVGState(m affine) svgState {
	return svgState{
		m:           m,
		fill:        svgPaint{color: 0},
		stroke:      svgPaint{none: true},
		strokeWidth: 1,
		fillAlpha:   1,
		strokeAlpha: 1,
		miterLimit:  4,
	}
}

// drawSVGStream interprets a page's concatenated driver literals as vector marks
// on the page. pageH is the page height in points (the SVG y origin).
func drawSVGStream(p *pdfkit.Page, fragment string, pageH float64) {
	if strings.TrimSpace(fragment) == "" {
		return
	}
	// The literals form a sequence of sibling elements, not one document: wrap
	// them in a root so the XML decoder sees a well-formed tree. An unbalanced
	// stream (a scope a package opened but never closed) still parses up to the
	// point it breaks, which is what a driver does with a truncated picture.
	dec := xml.NewDecoder(strings.NewReader(`<gotex:root xmlns:gotex="urn:gotex" xmlns:xlink="http://www.w3.org/1999/xlink">` + fragment + `</gotex:root>`))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose

	st := &svgStack{p: p, pageH: pageH}
	st.stack = []svgState{defaultSVGState(identity())}
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			st.start(t)
		case xml.EndElement:
			st.end()
		}
	}
}

// svgStack walks the element sequence, keeping the graphics-state stack.
type svgStack struct {
	p     *pdfkit.Page
	pageH float64
	stack []svgState
}

func (s *svgStack) top() svgState { return s.stack[len(s.stack)-1] }

func (s *svgStack) end() {
	if len(s.stack) > 1 {
		s.stack = s.stack[:len(s.stack)-1]
	}
}

// start pushes the element's inherited state and paints it if it is a shape.
func (s *svgStack) start(t xml.StartElement) {
	cur := s.top()
	name := t.Name.Local
	switch name {
	case "defs", "clipPath", "mask", "symbol", "marker", "pattern", "linearGradient", "radialGradient":
		cur.hidden = true
	}
	cur.apply(t)
	s.stack = append(s.stack, cur)
	if cur.hidden {
		return
	}
	switch name {
	case "path":
		s.paint(cur, func() bool { return buildSVGPath(s.p, attr(t, "d"), cur.m, 0, s.pageH) })
	case "rect":
		s.paint(cur, func() bool { return s.rect(t, cur.m) })
	case "line":
		// A line segment encloses no area, so only its stroke can show: an
		// inherited fill would emit a fill operator that paints nothing.
		noFill := cur
		noFill.fill = svgPaint{none: true}
		s.paint(noFill, func() bool { return s.line(t, cur.m) })
	case "circle":
		r := parseFloat(attr(t, "r"))
		s.paint(cur, func() bool {
			return s.ellipse(parseFloat(attr(t, "cx")), parseFloat(attr(t, "cy")), r, r, cur.m)
		})
	case "ellipse":
		s.paint(cur, func() bool {
			return s.ellipse(parseFloat(attr(t, "cx")), parseFloat(attr(t, "cy")),
				parseFloat(attr(t, "rx")), parseFloat(attr(t, "ry")), cur.m)
		})
	case "polygon":
		s.paint(cur, func() bool { return s.poly(attr(t, "points"), cur.m, true) })
	case "polyline":
		s.paint(cur, func() bool { return s.poly(attr(t, "points"), cur.m, false) })
	}
}

// apply folds an element's presentation attributes into the state. Attributes are
// read from both the attribute list and a `style="a:b;c:d"` declaration.
func (st *svgState) apply(t xml.StartElement) {
	set := func(name, value string) {
		switch name {
		case "transform":
			st.m = st.m.mul(parseTransform(value))
		case "fill":
			if c, none, ok := parseSVGFill(value); ok {
				st.fill = svgPaint{color: c, none: none}
			}
		case "stroke":
			if c, none, ok := parseSVGFill(value); ok {
				st.stroke = svgPaint{color: c, none: none}
			}
		case "stroke-width":
			if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil && v >= 0 {
				st.strokeWidth = v
			}
		case "fill-opacity":
			st.fillAlpha = parseAlpha(value, st.fillAlpha)
		case "stroke-opacity":
			st.strokeAlpha = parseAlpha(value, st.strokeAlpha)
		case "opacity":
			a := parseAlpha(value, 1)
			st.fillAlpha *= a
			st.strokeAlpha *= a
		case "stroke-dasharray":
			st.dash = parseDashArray(value)
		case "stroke-dashoffset":
			st.dashPhase = parseFloat(value)
		case "stroke-linecap":
			st.lineCap = map[string]int{"butt": 0, "round": 1, "square": 2}[strings.TrimSpace(value)]
		case "stroke-linejoin":
			st.lineJoin = map[string]int{"miter": 0, "round": 1, "bevel": 2}[strings.TrimSpace(value)]
		case "stroke-miterlimit":
			if v := parseFloat(value); v >= 1 {
				st.miterLimit = v
			}
		}
	}
	for _, a := range t.Attr {
		if a.Name.Local == "style" {
			for _, decl := range strings.Split(a.Value, ";") {
				if k, v, ok := strings.Cut(decl, ":"); ok {
					set(strings.TrimSpace(k), v)
				}
			}
			continue
		}
		set(a.Name.Local, a.Value)
	}
}

// paint runs a path constructor and then the painting operator the state calls
// for: fill, stroke, both, or neither (in which case the path is discarded).
func (s *svgStack) paint(st svgState, build func() bool) {
	doFill := !st.fill.none
	doStroke := !st.stroke.none && st.strokeWidth > 0
	if !doFill && !doStroke {
		return
	}
	if doFill {
		s.p.SetFillColor(svgColor(st.fill.color))
	}
	if doStroke {
		s.p.SetStrokeColor(svgColor(st.stroke.color))
		// The stroke width is a length in the element's own coordinate system, so
		// it scales with the current transform. go-pdfkit strokes in page space
		// here (paths are pre-transformed), so the width is scaled by the map's
		// area factor — exact for the uniform maps a drawing package emits.
		s.p.SetLineWidth(st.strokeWidth * st.m.scaleFactor())
		s.p.SetLineCap(st.lineCap)
		s.p.SetLineJoin(st.lineJoin)
		s.p.SetMiterLimit(st.miterLimit)
		s.p.SetDash(scaleDash(st.dash, st.m.scaleFactor()), st.dashPhase*st.m.scaleFactor())
	}
	if st.fillAlpha != 1 || st.strokeAlpha != 1 {
		s.p.SetAlpha(st.fillAlpha, st.strokeAlpha)
	}
	if !build() {
		s.p.EndPath()
		s.reset(st)
		return
	}
	switch {
	case doFill && doStroke:
		s.p.FillStroke()
	case doFill:
		s.p.Fill()
	default:
		s.p.Stroke()
	}
	s.reset(st)
}

// reset returns the shared page state the other painters assume: opaque, and no
// dash pattern. Colours are re-selected by every painter, so they are left as is.
func (s *svgStack) reset(st svgState) {
	if st.fillAlpha != 1 || st.strokeAlpha != 1 {
		s.p.SetAlpha(1, 1)
	}
	if len(st.dash) > 0 {
		s.p.SetDash(nil, 0)
	}
}

// map transforms an SVG point into PDF user space.
func (s *svgStack) pt(x, y float64, m affine) (float64, float64) {
	vx, vy := m.apply(x, y)
	return vx, s.pageH - vy
}

func (s *svgStack) rect(t xml.StartElement, m affine) bool {
	w, h := parseFloat(attr(t, "width")), parseFloat(attr(t, "height"))
	if w <= 0 || h <= 0 {
		return false
	}
	x, y := parseFloat(attr(t, "x")), parseFloat(attr(t, "y"))
	// The corners are mapped individually so a rotated map still yields the right
	// quadrilateral (Rectangle would give an axis-aligned box in page space).
	x0, y0 := s.pt(x, y, m)
	x1, y1 := s.pt(x+w, y, m)
	x2, y2 := s.pt(x+w, y+h, m)
	x3, y3 := s.pt(x, y+h, m)
	s.p.MoveTo(x0, y0)
	s.p.LineTo(x1, y1)
	s.p.LineTo(x2, y2)
	s.p.LineTo(x3, y3)
	s.p.ClosePath()
	return true
}

func (s *svgStack) line(t xml.StartElement, m affine) bool {
	x0, y0 := s.pt(parseFloat(attr(t, "x1")), parseFloat(attr(t, "y1")), m)
	x1, y1 := s.pt(parseFloat(attr(t, "x2")), parseFloat(attr(t, "y2")), m)
	s.p.MoveTo(x0, y0)
	s.p.LineTo(x1, y1)
	return true
}

func (s *svgStack) ellipse(cx, cy, rx, ry float64, m affine) bool {
	if rx <= 0 || ry <= 0 {
		return false
	}
	k := svgKappa
	move := func(x, y float64) { px, py := s.pt(x, y, m); s.p.MoveTo(px, py) }
	curve := func(x1, y1, x2, y2, x3, y3 float64) {
		a1, b1 := s.pt(x1, y1, m)
		a2, b2 := s.pt(x2, y2, m)
		a3, b3 := s.pt(x3, y3, m)
		s.p.CurveTo(a1, b1, a2, b2, a3, b3)
	}
	move(cx+rx, cy)
	curve(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry)
	curve(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy)
	curve(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry)
	curve(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy)
	s.p.ClosePath()
	return true
}

// poly builds a polyline, closing it for <polygon>.
func (s *svgStack) poly(points string, m affine, closed bool) bool {
	nums := parseFloats(points)
	if len(nums) < 4 {
		return false
	}
	for i := 0; i+1 < len(nums); i += 2 {
		x, y := s.pt(nums[i], nums[i+1], m)
		if i == 0 {
			s.p.MoveTo(x, y)
		} else {
			s.p.LineTo(x, y)
		}
	}
	if closed {
		s.p.ClosePath()
	}
	return true
}

// scaleFactor is the length scale of an affine map: the square root of the
// absolute determinant, which is the exact factor for the uniform (rotation and
// scaling) maps a drawing package emits and a fair mean for the rest.
func (m affine) scaleFactor() float64 {
	det := math.Abs(m.a*m.d - m.b*m.c)
	if det == 0 {
		return 0
	}
	return math.Sqrt(det)
}

// parseAlpha reads an opacity value (0..1), falling back to def when unreadable.
func parseAlpha(s string, def float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return math.Max(0, math.Min(1, v))
}

// parseDashArray reads a stroke-dasharray; "none" (or an unreadable value) turns
// dashing off.
func parseDashArray(s string) []float64 {
	if strings.TrimSpace(s) == "none" {
		return nil
	}
	nums := parseFloats(s)
	for _, v := range nums {
		if v < 0 {
			return nil
		}
	}
	return nums
}

// scaleDash maps a dash pattern through the current length scale.
func scaleDash(dash []float64, scale float64) []float64 {
	if len(dash) == 0 || scale == 1 {
		return dash
	}
	out := make([]float64, len(dash))
	for i, v := range dash {
		out[i] = v * scale
	}
	return out
}
