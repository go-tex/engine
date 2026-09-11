// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Laying a parsed \xymatrix out and drawing it. See xymatrix.go for where the
// geometry's numbers come from and why everything here is filled rather than
// stroked.

import (
	"fmt"
	"math"
	"strings"
)

// xyRule is the thickness of a diagram's lines, in points. XY-pic draws them
// from its line fonts at the current size; 0.4pt is what a 10pt document's rules
// measure, and it is what \thinlines means elsewhere in this engine.
const xyRule = 0.4

// xyHeadLong and xyHeadWide size an arrowhead, in points. Measured off tectonic
// at 600dpi on `A \ar[r] & B`: the head is 3.36bp across, and about three times
// that long.
const (
	xyHeadLong = 5.0
	xyHeadWide = 1.75
)

// xyDoubleSep is the distance between the two rules of an equality (@{=}), centre
// to centre, in points. Measured off tectonic at 600dpi across the middle of the
// reported diagram: the two rules are 0.36 and 0.48bp thick with 1.56bp of white
// between them, so their centres are about 2pt apart. Drawn one rule-thickness
// apart — 0.4pt, which is what this was — the pair reads as one thick line.
const xyDoubleSep = 2.0

// xyCanvas is the picture being built, and the extent of everything actually put
// in it.
//
// It exists because an <svg> CLIPS to its own viewport, silently. Sizing the
// picture from the grid alone cut whatever reached past it: a label on the left
// of the leftmost column's arrow is placed at a NEGATIVE x — the reported
// \ar[d]_f landed at translate(-3.65, 20.12) — and half of the f was simply not
// there. So every primitive records where it went, and the picture is sized and
// shifted to hold all of it.
type xyCanvas struct {
	b                      strings.Builder
	minX, minY, maxX, maxY float64
	any                    bool
}

// at records that something was drawn at (x, y).
func (c *xyCanvas) at(x, y float64) {
	if !c.any {
		c.minX, c.minY, c.maxX, c.maxY, c.any = x, y, x, y, true
		return
	}
	c.minX, c.minY = math.Min(c.minX, x), math.Min(c.minY, y)
	c.maxX, c.maxY = math.Max(c.maxX, x), math.Max(c.maxY, y)
}

// box records a rectangle by its two opposite corners.
func (c *xyCanvas) box(x0, y0, x1, y1 float64) { c.at(x0, y0); c.at(x1, y1) }

// placed is a cell after layout: its rendered maths and where the cell's BOX
// sits, in the composed picture's coordinates (y downwards from the top).
type placed struct {
	node         mathNode
	cx, cy       float64 // the box's centre
	halfW, halfH float64 // half the box, margins included
	hasMath      bool
}

// makeXymatrix lays a \xymatrix out and returns it as one formula box.
//
// Returning a single mathNode is what makes this cheap: everything downstream —
// centring the display, the searchable text layer, both drivers — already knows
// what to do with one, so a diagram travels the same road as any other formula.
func (e *Engine) makeXymatrix(opts, body, src string) (mathNode, bool) {
	grid := parseXymatrix(body)
	if len(grid) == 0 {
		return mathNode{}, false
	}
	colSep, rowSep := xySep(opts, 'C'), xySep(opts, 'R')

	// Render every cell first: the column widths and row heights follow from what
	// the cells actually measure.
	cells := make([][]placed, len(grid))
	cols := 0
	for i, row := range grid {
		cells[i] = make([]placed, len(row))
		if len(row) > cols {
			cols = len(row)
		}
		for j, c := range row {
			if strings.TrimSpace(c.math) != "" {
				cells[i][j].node = e.makeMath(c.math, false)
				cells[i][j].hasMath = true
			}
		}
	}
	if cols == 0 {
		return mathNode{}, false
	}

	colW := make([]float64, cols)
	rowH := make([]float64, len(grid))
	for i := range cells {
		for j := range cells[i] {
			n := cells[i][j].node
			w := spToPt(n.width) + 2*xyMargin
			h := spToPt(n.height+n.depth) + 2*xyMargin
			if w > colW[j] {
				colW[j] = w
			}
			if h > rowH[i] {
				rowH[i] = h
			}
		}
	}
	// A column or row with nothing in it still occupies its margins, so an empty
	// cell does not collapse the grid it is part of.
	for j := range colW {
		if colW[j] == 0 {
			colW[j] = 2 * xyMargin
		}
	}
	for i := range rowH {
		if rowH[i] == 0 {
			rowH[i] = 2 * xyMargin
		}
	}

	// Centres: half a box, the separation, half the next box.
	colC := make([]float64, cols)
	x := 0.0
	for j := range colW {
		if j > 0 {
			x += colW[j-1]/2 + colSep + colW[j]/2
		} else {
			x = colW[0] / 2
		}
		colC[j] = x
	}
	rowC := make([]float64, len(grid))
	y := 0.0
	for i := range rowH {
		if i > 0 {
			y += rowH[i-1]/2 + rowSep + rowH[i]/2
		} else {
			y = rowH[0] / 2
		}
		rowC[i] = y
	}
	width := colC[cols-1] + colW[cols-1]/2
	height := rowC[len(rowH)-1] + rowH[len(rowH)-1]/2

	for i := range cells {
		for j := range cells[i] {
			cells[i][j].cx, cells[i][j].cy = colC[j], rowC[i]
			cells[i][j].halfW, cells[i][j].halfH = colW[j]/2, rowH[i]/2
		}
	}

	cv := &xyCanvas{}
	// The grid itself is always in the picture, even where a cell is empty: an
	// empty row or column still holds its place.
	cv.box(0, 0, width, height)
	// The cells.
	for i := range cells {
		for j := range cells[i] {
			p := cells[i][j]
			if !p.hasMath {
				continue
			}
			w, h := spToPt(p.node.width), spToPt(p.node.height+p.node.depth)
			x, y := p.cx-w/2, p.cy-h/2
			cv.box(x, y, x+w, y+h)
			fmt.Fprintf(&cv.b, `<g transform="translate(%s,%s)">%s</g>`, f(x), f(y), p.node.svg)
		}
	}
	// The arrows, after the cells so a head is never hidden under a glyph.
	for i, row := range grid {
		for j, c := range row {
			for _, a := range c.arrows {
				ti, tj := i+a.dr, j+a.dc
				if ti < 0 || ti >= len(cells) || tj < 0 || tj >= cols {
					continue // points off the grid: nothing to join
				}
				e.drawXyArrow(cv, cells[i][j], cellAt(cells, ti, tj, colC, rowC, colW, rowH), a)
			}
		}
	}

	// Everything is shifted so the leftmost and topmost mark sits at the origin,
	// and the viewport is the full extent — otherwise the <svg> would clip it.
	w, h := cv.maxX-cv.minX, cv.maxY-cv.minY
	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		f(w), f(h), f(w), f(h))
	fmt.Fprintf(&out, `<g transform="translate(%s,%s)">`, f(-cv.minX), f(-cv.minY))
	out.WriteString(cv.b.String())
	out.WriteString(`</g></svg>`)

	// The box's baseline: the grid is centred on the maths axis, which is what
	// makes a square of objects look level with the line it interrupts. The
	// grid's centre moves with the shift, so the axis is measured in the picture's
	// new coordinates.
	axis := height/2 - cv.minY
	return mathNode{
		svg:    out.String(),
		src:    src,
		width:  ptToSP(w),
		height: ptToSP(axis + 2.5),
		depth:  ptToSP(h - axis - 2.5),
	}, true
}

// cellAt gives the box of a grid position, filling in a row that is shorter than
// the widest one so an arrow can still point at where that cell would be.
func cellAt(cells [][]placed, i, j int, colC, rowC, colW, rowH []float64) placed {
	if j < len(cells[i]) {
		return cells[i][j]
	}
	return placed{cx: colC[j], cy: rowC[i], halfW: colW[j] / 2, halfH: rowH[i] / 2}
}

// xyCurveDefault is how far a @/^/ or @/_/ arrow's belly leaves the straight
// line between the two cells, in points.
//
// Read off XY-pic rather than guessed: \ar@/^/ is \ar@slashing{^} (xyarrow.tex),
// which places the curve's control point at the midpoint plus TWICE the slide
// vector, and \vfromslide@i (xy.tex) makes that vector .5pc long when — as here —
// no distance is given. A quadratic Bézier passes half way to its control point,
// so the arrow's middle sits .5pc = 6pt off the chord.
const xyCurveDefault = 6.0

// xyCurveSamples is how many straight pieces a curve is drawn in. At the sizes a
// diagram uses, 32 puts the largest gap between the drawn chain and the true
// curve under a hundredth of a point.
const xyCurveSamples = 32

// xyPt is a point in the picture.
type xyPt struct{ x, y float64 }

// xyShaft is an arrow's centre line, sampled: two points for a straight arrow, a
// chain of short pieces for a curved one, with the distance travelled recorded
// beside each point.
//
// It exists so that everything said about an arrow is said in ONE unit — how far
// along it something is. Where the shaft stops short of its head, where a label
// sits, where the line is broken to let a label through: all of them are a
// distance, and none of them has to know whether the arrow is straight or bent.
type xyShaft struct {
	pts []xyPt
	cum []float64 // cum[i] is the distance from pts[0] to pts[i]
}

// newXyShaft records the running length of a polyline.
func newXyShaft(pts []xyPt) xyShaft {
	cum := make([]float64, len(pts))
	for i := 1; i < len(pts); i++ {
		cum[i] = cum[i-1] + math.Hypot(pts[i].x-pts[i-1].x, pts[i].y-pts[i-1].y)
	}
	return xyShaft{pts: pts, cum: cum}
}

func (s xyShaft) length() float64 { return s.cum[len(s.cum)-1] }

// at gives the point d along the shaft and the unit direction there.
func (s xyShaft) at(d float64) (xyPt, float64, float64) {
	if len(s.pts) < 2 {
		return s.pts[0], 1, 0
	}
	i := 0
	for i < len(s.cum)-2 && s.cum[i+1] < d {
		i++
	}
	seg := s.cum[i+1] - s.cum[i]
	t := 0.0
	if seg > 0 {
		t = (d - s.cum[i]) / seg
	}
	a, b := s.pts[i], s.pts[i+1]
	ux, uy := b.x-a.x, b.y-a.y
	n := math.Hypot(ux, uy)
	if n == 0 {
		return a, 1, 0
	}
	return xyPt{a.x + (b.x-a.x)*t, a.y + (b.y-a.y)*t}, ux / n, uy / n
}

// offset returns the same shaft moved d to the RIGHT of travel — the second rule
// of an equality, and the reason a curved @{=} stays parallel to itself.
func (s xyShaft) offset(d float64) xyShaft {
	out := make([]xyPt, len(s.pts))
	for i := range s.pts {
		_, ux, uy := s.at(s.cum[i])
		out[i] = xyPt{s.pts[i].x - uy*d, s.pts[i].y + ux*d}
	}
	return newXyShaft(out)
}

// draw fills the shaft between two distances along it.
func (s xyShaft) draw(c *xyCanvas, from, to float64, dashed bool) {
	if to <= from {
		return
	}
	for i := 0; i < len(s.pts)-1; i++ {
		a, b := math.Max(from, s.cum[i]), math.Min(to, s.cum[i+1])
		if b <= a {
			continue
		}
		p, _, _ := s.at(a)
		q, _, _ := s.at(b)
		if dashed {
			dashedQuad(c, p.x, p.y, q.x, q.y, xyRule)
		} else {
			quad(c, p.x, p.y, q.x, q.y, xyRule)
		}
	}
}

// xyShaftFor builds the centre line joining two cells, straight or curved.
func xyShaftFor(from, to placed, a xyArrow) (xyShaft, bool) {
	dx, dy := to.cx-from.cx, to.cy-from.cy
	if dx == 0 && dy == 0 {
		return xyShaft{}, false
	}
	dir, amount, curved := readXyCurve(a.curve)
	if !curved {
		// Leave each box along the line between the centres. Stopping at the BOX
		// and not at the ink is what XY-pic does, and it is why an arrow keeps the
		// same clearance whether the entry is an A or a fraction.
		x0, y0 := boxExit(from, dx, dy)
		x1, y1 := boxExit(to, -dx, -dy)
		if math.Hypot(x1-x0, y1-y0) <= 0 {
			return xyShaft{}, false
		}
		return newXyShaft([]xyPt{{x0, y0}, {x1, y1}}), true
	}
	// A curve leaves its box wherever it crosses it, which is NOT where the
	// straight line would: the whole point of the curve is that it sets off in
	// another direction. So the Bézier is built between the two CENTRES and then
	// cut where it enters open air.
	n := math.Hypot(dx, dy)
	ux, uy := dx/n, dy/n
	// (-uy, ux) is right of travel in a y-downward frame, and XY-pic's ^ is the
	// left side — so ^ subtracts it, as a ^label does.
	px, py := uy*dir*amount, -ux*dir*amount
	p0 := xyPt{from.cx, from.cy}
	p1 := xyPt{to.cx, to.cy}
	ctl := xyPt{(p0.x+p1.x)/2 + 2*px, (p0.y+p1.y)/2 + 2*py}
	const probe = 256
	inFrom := func(p xyPt) bool {
		return math.Abs(p.x-from.cx) <= from.halfW && math.Abs(p.y-from.cy) <= from.halfH
	}
	inTo := func(p xyPt) bool {
		return math.Abs(p.x-to.cx) <= to.halfW && math.Abs(p.y-to.cy) <= to.halfH
	}
	t0, t1 := 0.0, 1.0
	for i := 0; i <= probe; i++ {
		t := float64(i) / probe
		if inFrom(xyBezier(p0, ctl, p1, t)) {
			t0 = t
		} else {
			break
		}
	}
	for i := probe; i >= 0; i-- {
		t := float64(i) / probe
		if inTo(xyBezier(p0, ctl, p1, t)) {
			t1 = t
		} else {
			break
		}
	}
	if t1 <= t0 {
		return xyShaft{}, false
	}
	pts := make([]xyPt, 0, xyCurveSamples+1)
	for i := 0; i <= xyCurveSamples; i++ {
		t := t0 + (t1-t0)*float64(i)/xyCurveSamples
		pts = append(pts, xyBezier(p0, ctl, p1, t))
	}
	return newXyShaft(pts), true
}

// xyBezier evaluates the quadratic Bézier XY-pic draws a curved arrow with.
func xyBezier(p0, c, p1 xyPt, t float64) xyPt {
	u := 1 - t
	return xyPt{
		u*u*p0.x + 2*u*t*c.x + t*t*p1.x,
		u*u*p0.y + 2*u*t*c.y + t*t*p1.y,
	}
}

// readXyCurve reads an @/…/ body: which side the arrow bows out to, and by how
// much. The body is a direction (^ or _) and an optional distance.
func readXyCurve(spec string) (dir, amount float64, ok bool) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return 0, 0, false
	}
	switch s[0] {
	case '^':
		dir = 1
	case '_':
		dir = -1
	default:
		return 0, 0, false // @/…/ with a direction this does not read
	}
	amount = xyCurveDefault
	if rest := strings.TrimSpace(s[1:]); rest != "" {
		if v := xyDimen(rest); v != 0 {
			amount = v
		}
	}
	return dir, amount, true
}

// xyDimen reads a TeX dimension — a number and a unit — in points.
func xyDimen(s string) float64 {
	j := 0
	for j < len(s) && (s[j] == '-' || s[j] == '+' || s[j] == '.' || (s[j] >= '0' && s[j] <= '9')) {
		j++
	}
	v := parseFloat(s[:j])
	switch unit := strings.TrimSpace(s[j:]); {
	case strings.HasPrefix(unit, "pc"):
		return v * 12
	case strings.HasPrefix(unit, "pt"):
		return v
	case strings.HasPrefix(unit, "mm"):
		return v * 72.27 / 25.4
	case strings.HasPrefix(unit, "cm"):
		return v * 72.27 / 2.54
	case strings.HasPrefix(unit, "in"):
		return v * 72.27
	case strings.HasPrefix(unit, "ex"):
		return v * 4.3
	case strings.HasPrefix(unit, "em"):
		return v * 10
	}
	return 0
}

// xySetLabel is a label after it has been typeset and placed: what to draw, how
// big it is, and how far along the arrow it goes.
type xySetLabel struct {
	node mathNode
	w, h float64
	side byte
	at   float64
}

// measureXyLabels typesets an arrow's labels. They are measured BEFORE anything
// is drawn because a label on the line (|) breaks the line, and how wide the gap
// is cannot be known until the label has been set.
//
// A label is set in SCRIPT style: XY-pic's \labelstyle is \scriptstyle
// (xyarrow.tex), against \objectstyle = \textstyle for the entries themselves,
// which is why the f beside an arrow is visibly smaller than the A it comes from.
func (e *Engine) measureXyLabels(a xyArrow, total float64) []xySetLabel {
	var out []xySetLabel
	for _, l := range a.labels {
		n := e.makeMath(`\scriptstyle `+l.text, false)
		if n.svg == "" {
			continue
		}
		out = append(out, xySetLabel{
			node: n,
			w:    spToPt(n.width),
			h:    spToPt(n.height + n.depth),
			side: l.side,
			at:   l.pos * total,
		})
	}
	return out
}

// xyRuns gives the stretches of [from,to] that are actually drawn: everything
// except the gaps a label sitting ON the line leaves behind it.
//
// XY-pic breaks the connection there (\Cbreak@@) and widens the label's box by
// \labelmargin@ = \jot = 3pt on each side (\droplabel@) — so the rule stops
// clear of the label rather than running under it.
func xyRuns(labels []xySetLabel, sh xyShaft, from, to float64) [][2]float64 {
	runs := [][2]float64{{from, to}}
	for _, l := range labels {
		if l.side != '|' {
			continue
		}
		_, ux, uy := sh.at(l.at)
		half := (math.Abs(ux)*l.w+math.Abs(uy)*l.h)/2 + xyMargin
		a, b := l.at-half, l.at+half
		var next [][2]float64
		for _, r := range runs {
			if b <= r[0] || a >= r[1] {
				next = append(next, r)
				continue
			}
			if a > r[0] {
				next = append(next, [2]float64{r[0], a})
			}
			if b < r[1] {
				next = append(next, [2]float64{b, r[1]})
			}
		}
		runs = next
	}
	return runs
}

// drawXyArrow joins two cell boxes, stopping at each box's edge.
func (e *Engine) drawXyArrow(c *xyCanvas, from, to placed, a xyArrow) {
	sh, ok := xyShaftFor(from, to, a)
	if !ok {
		return
	}
	tail, head, double, dashed := readXyStyle(a.style)
	total := sh.length()
	labels := e.measureXyLabels(a, total)
	// The shaft stops short of the head so the two do not overlap; a head drawn
	// on top of a shaft that runs under it thickens the point.
	end := total
	if head > 0 {
		end -= xyHeadLong * 0.8
	}
	for _, r := range xyRuns(labels, sh, 0, end) {
		if double {
			// Two parallel rules, xyDoubleSep apart from centre to centre.
			sh.offset(xyDoubleSep/2).draw(c, r[0], r[1], false)
			sh.offset(-xyDoubleSep/2).draw(c, r[0], r[1], false)
			continue
		}
		sh.draw(c, r[0], r[1], dashed)
	}
	// A second head sits a whole head-length back, so >> reads as two points and
	// not as one thick one.
	for i := 0; i < head; i++ {
		p, ux, uy := sh.at(total - float64(i)*xyHeadLong*0.85)
		triangle(c, p.x, p.y, ux, uy)
	}
	if tail != 0 {
		p, ux, uy := sh.at(0)
		hook(c, p.x, p.y, ux, uy, tail)
	}
	e.drawXyLabels(c, sh, labels)
}

// boxExit is where the line from a box's centre in direction (dx,dy) leaves it.
func boxExit(p placed, dx, dy float64) (float64, float64) {
	if dx == 0 {
		return p.cx, p.cy + math.Copysign(p.halfH, dy)
	}
	if dy == 0 {
		return p.cx + math.Copysign(p.halfW, dx), p.cy
	}
	tx := p.halfW / math.Abs(dx)
	ty := p.halfH / math.Abs(dy)
	t := math.Min(tx, ty)
	return p.cx + dx*t, p.cy + dy*t
}

// readXyStyle reads an @{…} body: how the tail is hooked, how many heads there
// are, and whether the shaft is doubled or broken.
//
// The forms a diagram actually uses: -> a map, - a plain line, = an equality,
// ->> an epimorphism, ^(-> and _(-> a monomorphism's hook, .> and --> a broken
// line. Anything else falls back to a plain arrow, which is nearer than nothing.
func readXyStyle(style string) (tail, head int, double, dashed bool) {
	if style == "" {
		return 0, 1, false, false
	}
	s := style
	if i := strings.Index(s, "("); i > 0 {
		switch s[i-1] {
		case '^':
			tail = 1
		case '_':
			tail = -1
		}
		s = s[i+1:]
	}
	head = strings.Count(s, ">")
	if strings.Contains(s, "=") {
		double = true
	}
	if strings.Contains(s, ".") || strings.Contains(s, "--") {
		dashed = true
	}
	if !double && !dashed && !strings.Contains(s, "-") && head == 0 {
		head = 1 // an @{…} this does not understand still points somewhere
	}
	return tail, head, double, dashed
}

// quad fills the rectangle of width w centred on the segment (x0,y0)-(x1,y1).
func quad(c *xyCanvas, x0, y0, x1, y1, w float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 {
		return
	}
	px, py := -dy/n*w/2, dx/n*w/2
	c.at(x0+px, y0+py)
	c.at(x1+px, y1+py)
	c.at(x1-px, y1-py)
	c.at(x0-px, y0-py)
	fmt.Fprintf(&c.b, `<path d="M %s %s L %s %s L %s %s L %s %s Z" fill="black"/>`,
		f(x0+px), f(y0+py), f(x1+px), f(y1+py), f(x1-px), f(y1-py), f(x0-px), f(y0-py))
}

// dashedQuad draws the same segment broken up. XY-pic's @{.>} is DOTTED rather
// than dashed — measured against tectonic, its marks are about as long as the
// rule is thick and sit about a rule apart — so the dash is short enough to read
// as a dot at text size.
func dashedQuad(c *xyCanvas, x0, y0, x1, y1, w float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 {
		return
	}
	ux, uy := dx/n, dy/n
	dash, gap := w, w*2.5
	for t := 0.0; t < n; t += dash + gap {
		end := math.Min(t+dash, n)
		quad(c, x0+ux*t, y0+uy*t, x0+ux*end, y0+uy*end, w)
	}
}

// triangle fills an arrowhead whose point is at (x,y), pointing along (ux,uy).
func triangle(c *xyCanvas, x, y, ux, uy float64) {
	bx, by := x-ux*xyHeadLong, y-uy*xyHeadLong
	px, py := -uy*xyHeadWide, ux*xyHeadWide
	c.at(x, y)
	c.at(bx+px, by+py)
	c.at(bx-px, by-py)
	fmt.Fprintf(&c.b, `<path d="M %s %s L %s %s L %s %s Z" fill="black"/>`,
		f(x), f(y), f(bx+px), f(by+py), f(bx-px), f(by-py))
}

// hook draws the little curl at the tail of a monomorphism's arrow, on the side
// given by dir (+1 or -1). It is filled, like everything else here: the curl is
// the ring between two half circles.
func hook(c *xyCanvas, x, y, ux, uy float64, dir int) {
	const r = 2.2
	// The centre of the curl sits one radius along the arrow, offset to the side.
	px, py := -uy*float64(dir), ux*float64(dir)
	cx, cy := x+ux*r, y+uy*r
	outer, inner := r+xyRule/2, r-xyRule/2
	// The curl reaches at most one outer radius from its centre in any direction.
	c.box(cx-outer, cy-outer, cx+outer, cy+outer)
	// A half ring: out along one side, round, and back.
	const k = 0.5522847498307936
	fmt.Fprintf(&c.b, `<path d="M %s %s C %s %s %s %s %s %s L %s %s C %s %s %s %s %s %s Z" fill="black"/>`,
		f(cx-px*outer), f(cy-py*outer),
		f(cx-px*outer-ux*outer*k), f(cy-py*outer-uy*outer*k),
		f(cx-ux*outer-px*outer*k), f(cy-uy*outer-py*outer*k),
		f(cx-ux*outer), f(cy-uy*outer),
		f(cx-ux*inner), f(cy-uy*inner),
		f(cx-ux*inner-px*inner*k), f(cy-uy*inner-py*inner*k),
		f(cx-px*inner-ux*inner*k), f(cy-py*inner-uy*inner*k),
		f(cx-px*inner), f(cy-py*inner))
}

// drawXyLabels sets an arrow's annotations beside it: ^ on the left of travel,
// _ on the right, | on the line itself, in the gap the shaft left for it.
func (e *Engine) drawXyLabels(c *xyCanvas, sh xyShaft, labels []xySetLabel) {
	for _, l := range labels {
		p, ux, uy := sh.at(l.at)
		// Perpendicular to travel, far enough out to clear the rule and the text.
		//
		// (-uy, ux) turns +90 degrees in a y-DOWNWARD frame, which is RIGHT of the
		// direction of travel. XY-pic's _ is the right side and ^ the left — so a
		// downward \ar[d]_f carries its f on the page's LEFT, which is where the
		// reference puts it.
		off := l.h/2 + 2
		px, py := -uy, ux
		cx, cy := p.x, p.y
		switch l.side {
		case '_':
			cx, cy = p.x+px*off, p.y+py*off
		case '^':
			cx, cy = p.x-px*off, p.y-py*off
		}
		c.box(cx-l.w/2, cy-l.h/2, cx+l.w/2, cy+l.h/2)
		fmt.Fprintf(&c.b, `<g transform="translate(%s,%s)">%s</g>`, f(cx-l.w/2), f(cy-l.h/2), l.node.svg)
	}
}
