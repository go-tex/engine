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

// This file draws a go-tex/math SVG into a PDF as vector paths, so math appears
// in PDF output (not only SVG). It walks the SVG subset go-tex/math emits —
// nested <g transform="translate(…)/scale(…)"> groups, <path d="…"> with M/L/C/Q/Z
// commands, and <rect> fraction bars — accumulating a 2×3 affine, and emits the
// pdfkit path operators. SVG is y-down inside the math box; the box's top sits at
// pdfTop in PDF (y-up) space, so a viewBox point (x,y) maps to (left+x, pdfTop−y).

// affine is a 2×3 matrix [a c e / b d f]; it maps (x,y) → (a·x+c·y+e, b·x+d·y+f).
type affine struct{ a, b, c, d, e, f float64 }

func identity() affine { return affine{1, 0, 0, 1, 0, 0} }

func (m affine) mul(n affine) affine { // m ∘ n (apply n first, then m)
	return affine{
		a: m.a*n.a + m.c*n.b,
		b: m.b*n.a + m.d*n.b,
		c: m.a*n.c + m.c*n.d,
		d: m.b*n.c + m.d*n.d,
		e: m.a*n.e + m.c*n.f + m.e,
		f: m.b*n.e + m.d*n.f + m.f,
	}
}

func (m affine) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// drawMathSVG parses a go-tex/math SVG and draws it into the page. left is the
// PDF x of the math box's left edge; pdfTop is the PDF y of its top edge.
func drawMathSVG(p *pdfkit.Page, svg string, left, pdfTop float64) {
	dec := xml.NewDecoder(strings.NewReader(svg))
	stack := []affine{identity()}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			cur := stack[len(stack)-1]
			cur = cur.mul(parseTransform(attr(t, "transform")))
			switch t.Name.Local {
			case "g", "svg":
				stack = append(stack, cur)
			case "path":
				drawSVGPath(p, attr(t, "d"), cur, left, pdfTop)
				stack = append(stack, cur) // path has no children but keep stack balanced
			case "rect":
				drawSVGRect(p, t, cur, left, pdfTop)
				stack = append(stack, cur)
			default:
				stack = append(stack, cur)
			}
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// parseTransform handles SVG's transform list — translate(a[,b]) and scale(a[,b])
// (the forms go-tex/math emits) plus rotate(deg[,cx,cy]), matrix(a,b,c,d,e,f) and
// skewX/skewY(deg) (the forms a drawing package's driver emits for an arbitrary
// coordinate map) — composed left-to-right.
func parseTransform(s string) affine {
	m := identity()
	for len(s) > 0 {
		i := strings.IndexByte(s, '(')
		if i < 0 {
			break
		}
		op := strings.TrimSpace(s[:i])
		j := strings.IndexByte(s, ')')
		if j < 0 {
			break
		}
		args := parseFloats(s[i+1 : j])
		s = s[j+1:]
		switch op {
		case "translate":
			tx, ty := arg(args, 0), arg(args, 1)
			m = m.mul(affine{1, 0, 0, 1, tx, ty})
		case "scale":
			sx := arg(args, 0)
			sy := sx
			if len(args) > 1 {
				sy = args[1]
			}
			m = m.mul(affine{sx, 0, 0, sy, 0, 0})
		case "rotate":
			rad := arg(args, 0) * math.Pi / 180
			cos, sin := math.Cos(rad), math.Sin(rad)
			r := affine{cos, sin, -sin, cos, 0, 0}
			if len(args) >= 3 { // rotate about (cx,cy)
				cx, cy := args[1], args[2]
				r = affine{1, 0, 0, 1, cx, cy}.mul(r).mul(affine{1, 0, 0, 1, -cx, -cy})
			}
			m = m.mul(r)
		case "matrix":
			if len(args) >= 6 {
				m = m.mul(affine{args[0], args[1], args[2], args[3], args[4], args[5]})
			}
		case "skewX":
			m = m.mul(affine{1, 0, math.Tan(arg(args, 0) * math.Pi / 180), 1, 0, 0})
		case "skewY":
			m = m.mul(affine{1, math.Tan(arg(args, 0) * math.Pi / 180), 0, 1, 0, 0})
		}
	}
	return m
}

func drawSVGRect(p *pdfkit.Page, e xml.StartElement, m affine, left, pdfTop float64) {
	x := parseFloat(attr(e, "x"))
	y := parseFloat(attr(e, "y"))
	w := parseFloat(attr(e, "width"))
	h := parseFloat(attr(e, "height"))
	// transform the four corners; rects here are axis-aligned (translate/scale).
	x0, y0 := m.apply(x, y)
	x1, y1 := m.apply(x+w, y+h)
	px, py := left+fmin(x0, x1), pdfTop-fmax(y0, y1)
	p.Rectangle(pdfkit.Rect{X: px, Y: py, Width: fabs(x1 - x0), Height: fabs(y1 - y0)})
	p.Fill()
}

// drawSVGPath parses an SVG path "d" and fills it, mapping points through m and
// the box placement. Supports M/L/H/V/C/Q/Z (absolute and relative).
func drawSVGPath(p *pdfkit.Page, d string, m affine, left, pdfTop float64) {
	if buildSVGPath(p, d, m, left, pdfTop) {
		p.Fill()
	}
}

// buildSVGPath emits an SVG path "d" as PDF path-construction operators, mapping
// points through m and the box placement, and reports whether anything was
// constructed. It paints nothing: the caller chooses the painting operator, so a
// stroked path (a drawing package's usual output, see svgpdf.go) and a filled one
// (a glyph outline) share one path parser.
func buildSVGPath(p *pdfkit.Page, d string, m affine, left, pdfTop float64) bool {
	pt := newPathTokens(d)
	var cx, cy, startX, startY float64 // current point in SVG viewBox coords
	moveTo := func(x, y float64) {
		cx, cy, startX, startY = x, y, x, y
		vx, vy := m.apply(x, y)
		p.MoveTo(left+vx, pdfTop-vy)
	}
	lineTo := func(x, y float64) {
		cx, cy = x, y
		vx, vy := m.apply(x, y)
		p.LineTo(left+vx, pdfTop-vy)
	}
	curveTo := func(x1, y1, x2, y2, x3, y3 float64) {
		cx, cy = x3, y3
		a1, b1 := m.apply(x1, y1)
		a2, b2 := m.apply(x2, y2)
		a3, b3 := m.apply(x3, y3)
		p.CurveTo(left+a1, pdfTop-b1, left+a2, pdfTop-b2, left+a3, pdfTop-b3)
	}
	drew := false
	for pt.more() {
		cmd := pt.cmd()
		rel := cmd >= 'a' && cmd <= 'z'
		base := func() (float64, float64) {
			if rel {
				return cx, cy
			}
			return 0, 0
		}
		switch cmd | 0x20 { // lowercase
		case 'm':
			bx, by := base()
			moveTo(bx+pt.num(), by+pt.num())
			drew = true
			for pt.moreNums() { // subsequent pairs are implicit lineto
				bx, by = base()
				lineTo(bx+pt.num(), by+pt.num())
			}
		case 'l':
			for pt.moreNums() {
				bx, by := base()
				lineTo(bx+pt.num(), by+pt.num())
			}
		case 'h':
			for pt.moreNums() {
				bx, _ := base()
				lineTo(bx+pt.num(), cy)
			}
		case 'v':
			for pt.moreNums() {
				_, by := base()
				lineTo(cx, by+pt.num())
			}
		case 'c':
			for pt.moreNums() {
				bx, by := base()
				curveTo(bx+pt.num(), by+pt.num(), bx+pt.num(), by+pt.num(), bx+pt.num(), by+pt.num())
			}
		case 'q':
			for pt.moreNums() {
				bx, by := base()
				qx, qy := bx+pt.num(), by+pt.num()
				ex, ey := bx+pt.num(), by+pt.num()
				// quadratic → cubic control points
				c1x, c1y := cx+2.0/3.0*(qx-cx), cy+2.0/3.0*(qy-cy)
				c2x, c2y := ex+2.0/3.0*(qx-ex), ey+2.0/3.0*(qy-ey)
				curveTo(c1x, c1y, c2x, c2y, ex, ey)
			}
		case 'z':
			lineTo(startX, startY)
			p.ClosePath()
		}
	}
	return drew
}

// pathTokens is a tiny scanner over an SVG path's command/number stream.
type pathTokens struct {
	s   string
	i   int
	cur byte
}

func newPathTokens(s string) *pathTokens { return &pathTokens{s: s} }

func (t *pathTokens) skipSep() {
	for t.i < len(t.s) {
		c := t.s[t.i]
		if c == ' ' || c == ',' || c == '\t' || c == '\n' || c == '\r' {
			t.i++
			continue
		}
		break
	}
}

func (t *pathTokens) more() bool { t.skipSep(); return t.i < len(t.s) }

func (t *pathTokens) cmd() byte {
	t.skipSep()
	if t.i < len(t.s) && isAlpha(t.s[t.i]) {
		t.cur = t.s[t.i]
		t.i++
	}
	return t.cur
}

// moreNums reports whether a number follows (so repeated arg-sets can be read).
func (t *pathTokens) moreNums() bool {
	t.skipSep()
	return t.i < len(t.s) && !isAlpha(t.s[t.i])
}

func (t *pathTokens) num() float64 {
	t.skipSep()
	start := t.i
	for t.i < len(t.s) {
		c := t.s[t.i]
		if (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '+' || c == 'e' || c == 'E' {
			// a '-' mid-number starts a new number unless right after e/E
			if (c == '-' || c == '+') && t.i > start && t.s[t.i-1] != 'e' && t.s[t.i-1] != 'E' {
				break
			}
			t.i++
			continue
		}
		break
	}
	v, _ := strconv.ParseFloat(t.s[start:t.i], 64)
	return v
}

func isAlpha(c byte) bool { return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') }

func parseFloats(s string) []float64 {
	var out []float64
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if v, err := strconv.ParseFloat(strings.TrimSpace(f), 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseFloat(s string) float64 { v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return v }

func arg(a []float64, i int) float64 {
	if i < len(a) {
		return a[i]
	}
	return 0
}

func fabs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
func fmin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func fmax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
