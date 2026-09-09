// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"
)

// LaTeX's picture environment draws with \line, \vector, \circle, \oval and
// \qbezier (ltpictur.dtx). The engine defined \put and \multiput — for what their
// ABSENCE cost, since an undefined \put leaves "(-0.33\textwidth," in the input
// and TeX then reads it as an assignment — but not these, "the engine having no
// picture layer to draw them with".
//
// That premise stopped being true when the pgf work landed a graphics seam:
// \special{gotex:…} carries an SVG fragment that the SVG driver writes verbatim
// and the PDF driver interprets into vector operators (special.go, svgpdf.go),
// with {?x}/{?y} substituted at shipout for the literal's own reference point on
// the page. A picture command is exactly that: a mark at the point TeX has put
// the box. So they are drawn here, through the same seam pgf uses.
//
// Undefined, they were worse than absent: \line(1,1){30} left "(1,1)30" on the
// page as prose. Against tectonic, a picture with a frame, a circle, a diagonal
// and an arrow drew all four in the reference and NONE here — only the numbers,
// scattered through the text.
//
// TeX resolves the units, because \unitlength is a LaTeX register and the
// coordinate forms are the ones ltpictur already scans: a bare factor takes
// \unitlength as its unit, "-0.33\textwidth" keeps its own (\@defaultunitsset,
// classkernel.go). Each macro hands this layer plain <number> and <dimen>
// arguments and the geometry is done in scaled points.

// pictureLineTo returns the SVG displacement (dx, dy in points, y DOWN as SVG
// counts it) of \line(xarg,yarg){len}.
//
// The length argument is the line's HORIZONTAL extent, not its length, except on
// a vertical line where it is the vertical one — that is ltpictur's rule, and the
// reason \line(2,1){40} and \line(1,1){40} end at the same x:
//
//	\ifnum\@xarg =\z@ \@vline \else \ifnum\@yarg =\z@ \@hline \else \@sline\fi\fi
//	                                                              ltpictur, \line
func pictureLineTo(xarg, yarg int, length float64) (dx, dy float64) {
	switch {
	case xarg == 0:
		if yarg < 0 {
			return 0, length // SVG y grows downwards
		}
		return 0, -length
	case yarg == 0:
		if xarg < 0 {
			return -length, 0
		}
		return length, 0
	}
	dx = length
	if xarg < 0 {
		dx = -length
	}
	// The slope is yarg/xarg and the run is |length|, so the rise follows. Negated
	// for SVG, whose y grows downwards where TeX's grows up.
	dy = -dx * float64(yarg) / float64(xarg)
	return dx, dy
}

// pictureColor is the colour a picture command draws in: the current \color, or
// black. "currentColor" would not do — parseSVGFill IGNORES it (svgimage.go),
// leaving the stroke at SVG's initial value, which is none: the lines simply did
// not appear. The maths layer bakes the colour in for the same reason.
func (e *Engine) pictureColor() string {
	if e.curColor != 0 {
		return hexColor(e.curColor)
	}
	return "black"
}

// arrowHeadSize returns the length and half-width of \vector's head for a line of
// width w, in points.
//
// LaTeX draws the head from a font character, and it has exactly TWO of them —
// one in line10 for \thinlines and one in linew10 for \thicklines — so the head
// does NOT scale with the line width, it steps. Measured off tectonic at 300dpi:
//
//	\thinlines  (0.4pt): 4.08pt long, 2.88pt across the base
//	\thicklines (0.8pt): 6.00pt long, 3.84pt across the base
//
// A single multiple of w cannot give both (10.2w and 3.6w against 7.5w and 2.4w).
// The straight line through the two reproduces each of them exactly, which is
// every case LaTeX itself has an answer for, and interpolates for a
// \linethickness in between.
func arrowHeadSize(w float64) (length, half float64) {
	length = 4.08 + 4.8*(w-0.4)
	half = 1.44 + 1.2*(w-0.4)
	return math.Max(length, w), math.Max(half, w/2)
}

// arrowHead returns the SVG path of a filled arrowhead at (x, y) pointing along
// (dx, dy). Drawn as a triangle it is the same shape at any slope, where LaTeX's
// font heads exist only for the six slopes its arrow font carries.
func arrowHead(x, y, dx, dy, w float64, color string) string {
	n := math.Hypot(dx, dy)
	if n == 0 {
		return ""
	}
	ux, uy := dx/n, dy/n
	long, wide := arrowHeadSize(w)
	bx, by := x-ux*long, y-uy*long
	px, py := -uy*wide, ux*wide
	return fmt.Sprintf(`<path d="M %s %s L %s %s L %s %s Z" fill="%s" stroke="none"/>`,
		f(x), f(y), f(bx+px), f(by+py), f(bx-px), f(by-py), color)
}

// strokeAttrs is the stroke a picture command draws with: the current
// \linethickness, in the current colour, and no fill.
func strokeAttrs(w float64, color string) string {
	return fmt.Sprintf(`fill="none" stroke="%s" stroke-width="%s"`, color, f(w))
}

// emitPicture places one SVG fragment in the current list, at the point TeX has
// reached. The fragment is wrapped so its own coordinates are relative to that
// point: {?x}/{?y} are substituted at shipout.
func (e *Engine) emitPicture(svg string) {
	e.place(specialNode{text: gotexSpecial + svg, srcLine: e.curSrcLine})
}

// doPictureLine implements \gotex@line <xarg> <yarg> <length> <width>, the
// drawing half of \line and \vector (\vector passes head=true).
func (e *Engine) doPictureLine(head bool) {
	xarg := e.scanInt()
	yarg := e.scanInt()
	length := spToPt(e.scanDimen())
	w := spToPt(e.scanDimen())
	if length < 0 {
		return // \@badlinearg: a negative length draws nothing
	}
	dx, dy := pictureLineTo(xarg, yarg, length)
	if dx == 0 && dy == 0 && !head {
		return
	}
	color := e.pictureColor()
	var b strings.Builder
	fmt.Fprintf(&b, `<path d="M {?x} {?y} l %s %s" %s/>`, f(dx), f(dy), strokeAttrs(w, color))
	if head {
		// The head is drawn at the far end, in the fragment's own coordinates: the
		// driver substitutes {?x}/{?y} for the origin, so the tip is that plus the
		// displacement. Written as a second path because a filled triangle and a
		// stroked line cannot share one paint operation.
		b.WriteString(`<g transform="translate({?x},{?y})">`)
		b.WriteString(arrowHead(dx, dy, dx, dy, w, color))
		b.WriteString(`</g>`)
	}
	e.emitPicture(b.String())
}

// doPictureCircle implements \gotex@circle <diameter> <width> <filled>: \circle
// draws the outline, \circle* fills it. LaTeX takes the DIAMETER, and \put
// centres the circle on its own point.
func (e *Engine) doPictureCircle() {
	d := spToPt(e.scanDimen())
	w := spToPt(e.scanDimen())
	filled := e.scanInt() != 0
	if d <= 0 {
		return
	}
	r := d / 2
	attrs := strokeAttrs(w, e.pictureColor())
	if filled {
		attrs = `fill="` + e.pictureColor() + `" stroke="none"`
	}
	e.emitPicture(fmt.Sprintf(`<circle cx="{?x}" cy="{?y}" r="%s" %s/>`, f(r), attrs))
}

// readBraceText consumes a following {…} group and returns its text. grabGroup
// starts INSIDE the group — it counts from depth 1 — so the opening brace has to
// be taken first, which is what \special does too (special.go).
func (e *Engine) readBraceText() string {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return ""
	}
	if t.cs_ || t.cat != catBegin {
		e.back(t)
		return ""
	}
	return e.toksToString(e.expandList(e.grabGroup()))
}

// ovalPart turns \oval's [part] letters into the quadrant mask doPictureOval
// keeps: 1 top, 2 bottom, 4 left, 8 right, 0 for the whole oval. "tr" is the top
// RIGHT quarter, so the bits name what to KEEP and a quadrant survives only when
// every named bit agrees with it.
func ovalPart(s string) int {
	mask := 0
	for _, r := range s {
		switch r {
		case 't', 'T':
			mask |= 1
		case 'b', 'B':
			mask |= 2
		case 'l', 'L':
			mask |= 4
		case 'r', 'R':
			mask |= 8
		}
	}
	return mask
}

// doPictureOval implements \gotex@oval <width> <height> <width> {<part>}: a
// rectangle with quarter-circle corners, centred on the point, and \oval's
// optional [part] keeping only the half or quarter named. The part is \oval's
// own letters — t, b, l, r, in any combination — and empty means the whole oval.
func (e *Engine) doPictureOval() {
	ow := spToPt(e.scanDimen())
	oh := spToPt(e.scanDimen())
	w := spToPt(e.scanDimen())
	part := ovalPart(e.readBraceText())
	if ow <= 0 || oh <= 0 {
		return
	}
	// LaTeX's corners are quarter circles of the largest radius the box allows,
	// which is half the shorter side.
	r := math.Min(ow, oh) / 2
	hw, hh := ow/2, oh/2
	keep := func(top, left bool) bool {
		if part == 0 {
			return true
		}
		if part&1 != 0 && !top {
			return false
		}
		if part&2 != 0 && top {
			return false
		}
		if part&4 != 0 && !left {
			return false
		}
		if part&8 != 0 && left {
			return false
		}
		return true
	}
	color := e.pictureColor()
	// A quarter circle as a cubic Bézier: the control points sit k*r along the
	// tangents, k = 4/3*(sqrt(2)-1) — the same approximation svgimage.go uses for
	// a circle. Written with C rather than SVG's arc command A because the PDF
	// driver's path reader does not implement A (it now skips it rather than
	// spinning, but skipping would drop the corner).
	const k = 0.5522847498307936
	var b strings.Builder
	b.WriteString(`<g transform="translate({?x},{?y})">`)
	// Each quadrant is the straight run into a corner plus the corner itself. SVG
	// y grows downwards, so "top" is negative y.
	quad := func(top, left bool, d string) {
		if keep(top, left) {
			fmt.Fprintf(&b, `<path d="%s" %s/>`, d, strokeAttrs(w, color))
		}
	}
	kr := k * r
	quad(true, true, fmt.Sprintf("M %s %s L %s %s C %s %s %s %s %s %s L %s %s",
		f(-hw), f(0), f(-hw), f(-hh+r),
		f(-hw), f(-hh+r-kr), f(-hw+r-kr), f(-hh), f(-hw+r), f(-hh),
		f(0), f(-hh)))
	quad(true, false, fmt.Sprintf("M %s %s L %s %s C %s %s %s %s %s %s L %s %s",
		f(0), f(-hh), f(hw-r), f(-hh),
		f(hw-r+kr), f(-hh), f(hw), f(-hh+r-kr), f(hw), f(-hh+r),
		f(hw), f(0)))
	quad(false, false, fmt.Sprintf("M %s %s L %s %s C %s %s %s %s %s %s L %s %s",
		f(hw), f(0), f(hw), f(hh-r),
		f(hw), f(hh-r+kr), f(hw-r+kr), f(hh), f(hw-r), f(hh),
		f(0), f(hh)))
	quad(false, true, fmt.Sprintf("M %s %s L %s %s C %s %s %s %s %s %s L %s %s",
		f(0), f(hh), f(-hw+r), f(hh),
		f(-hw+r-kr), f(hh), f(-hw), f(hh-r+kr), f(-hw), f(hh-r),
		f(-hw), f(0)))
	b.WriteString(`</g>`)
	e.emitPicture(b.String())
}

// doQbezier implements \gotex@qbezier <x1> <y1> <x2> <y2> <x3> <y3> <width>: a
// quadratic Bézier from the first point through the second as control point to
// the third, all relative to the current point. LaTeX's optional [N] asks for N
// plotted dots; a real curve needs no such count and the argument is dropped by
// the macro.
func (e *Engine) doQbezier() {
	x1, y1 := spToPt(e.scanDimen()), spToPt(e.scanDimen())
	x2, y2 := spToPt(e.scanDimen()), spToPt(e.scanDimen())
	x3, y3 := spToPt(e.scanDimen()), spToPt(e.scanDimen())
	w := spToPt(e.scanDimen())
	e.emitPicture(fmt.Sprintf(
		`<g transform="translate({?x},{?y})"><path d="M %s %s Q %s %s %s %s" %s/></g>`,
		f(x1), f(-y1), f(x2), f(-y2), f(x3), f(-y3), strokeAttrs(w, e.pictureColor())))
}
