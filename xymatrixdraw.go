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

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		f(width), f(height), f(width), f(height))
	// The cells.
	for i := range cells {
		for j := range cells[i] {
			p := cells[i][j]
			if !p.hasMath {
				continue
			}
			w, h := spToPt(p.node.width), spToPt(p.node.height+p.node.depth)
			fmt.Fprintf(&b, `<g transform="translate(%s,%s)">%s</g>`,
				f(p.cx-w/2), f(p.cy-h/2), p.node.svg)
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
				e.drawXyArrow(&b, cells[i][j], cellAt(cells, ti, tj, colC, rowC, colW, rowH), a)
			}
		}
	}
	b.WriteString(`</svg>`)

	// The box's baseline: a one-row diagram sits on the text baseline like any
	// formula, and a taller one is centred on the maths axis, which is what makes
	// a square of objects look level with the line it interrupts.
	axis := height / 2
	return mathNode{
		svg:    b.String(),
		src:    src,
		width:  ptToSP(width),
		height: ptToSP(axis + 2.5),
		depth:  ptToSP(height - axis - 2.5),
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

// drawXyArrow joins two cell boxes, stopping at each box's edge.
func (e *Engine) drawXyArrow(b *strings.Builder, from, to placed, a xyArrow) {
	dx, dy := to.cx-from.cx, to.cy-from.cy
	if dx == 0 && dy == 0 {
		return
	}
	// Leave each box along the line between the centres. Stopping at the BOX and
	// not at the ink is what XY-pic does, and it is why an arrow keeps the same
	// clearance whether the entry is an A or a fraction.
	x0, y0 := boxExit(from, dx, dy)
	x1, y1 := boxExit(to, -dx, -dy)
	n := math.Hypot(x1-x0, y1-y0)
	if n <= 0 {
		return
	}
	ux, uy := (x1-x0)/n, (y1-y0)/n

	tail, head, double, dashed := readXyStyle(a.style)
	// The shaft stops short of the head so the two do not overlap; a head drawn
	// on top of a shaft that runs under it thickens the point.
	sx, sy := x1, y1
	if head > 0 {
		sx, sy = x1-ux*xyHeadLong*0.8, y1-uy*xyHeadLong*0.8
	}
	switch {
	case double:
		// Two parallel rules, half a rule's thickness either side of the line.
		px, py := -uy*xyRule, ux*xyRule
		quad(b, x0+px, y0+py, sx+px, sy+py, xyRule)
		quad(b, x0-px, y0-py, sx-px, sy-py, xyRule)
	case dashed:
		dashedQuad(b, x0, y0, sx, sy, xyRule)
	default:
		quad(b, x0, y0, sx, sy, xyRule)
	}
	// A second head sits a whole head-length back, so >> reads as two points and
	// not as one thick one.
	for i := 0; i < head; i++ {
		back := float64(i) * xyHeadLong * 0.85
		triangle(b, x1-ux*back, y1-uy*back, ux, uy)
	}
	if tail != 0 {
		hook(b, x0, y0, ux, uy, tail)
	}
	e.drawXyLabels(b, a, (x0+x1)/2, (y0+y1)/2, ux, uy)
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
func quad(b *strings.Builder, x0, y0, x1, y1, w float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 {
		return
	}
	px, py := -dy/n*w/2, dx/n*w/2
	fmt.Fprintf(b, `<path d="M %s %s L %s %s L %s %s L %s %s Z" fill="black"/>`,
		f(x0+px), f(y0+py), f(x1+px), f(y1+py), f(x1-px), f(y1-py), f(x0-px), f(y0-py))
}

// dashedQuad draws the same segment broken up. XY-pic's @{.>} is DOTTED rather
// than dashed — measured against tectonic, its marks are about as long as the
// rule is thick and sit about a rule apart — so the dash is short enough to read
// as a dot at text size.
func dashedQuad(b *strings.Builder, x0, y0, x1, y1, w float64) {
	dx, dy := x1-x0, y1-y0
	n := math.Hypot(dx, dy)
	if n == 0 {
		return
	}
	ux, uy := dx/n, dy/n
	dash, gap := w, w*2.5
	for t := 0.0; t < n; t += dash + gap {
		end := math.Min(t+dash, n)
		quad(b, x0+ux*t, y0+uy*t, x0+ux*end, y0+uy*end, w)
	}
}

// triangle fills an arrowhead whose point is at (x,y), pointing along (ux,uy).
func triangle(b *strings.Builder, x, y, ux, uy float64) {
	bx, by := x-ux*xyHeadLong, y-uy*xyHeadLong
	px, py := -uy*xyHeadWide, ux*xyHeadWide
	fmt.Fprintf(b, `<path d="M %s %s L %s %s L %s %s Z" fill="black"/>`,
		f(x), f(y), f(bx+px), f(by+py), f(bx-px), f(by-py))
}

// hook draws the little curl at the tail of a monomorphism's arrow, on the side
// given by dir (+1 or -1). It is filled, like everything else here: the curl is
// the ring between two half circles.
func hook(b *strings.Builder, x, y, ux, uy float64, dir int) {
	const r = 2.2
	// The centre of the curl sits one radius along the arrow, offset to the side.
	px, py := -uy*float64(dir), ux*float64(dir)
	cx, cy := x+ux*r, y+uy*r
	outer, inner := r+xyRule/2, r-xyRule/2
	// A half ring: out along one side, round, and back.
	const k = 0.5522847498307936
	fmt.Fprintf(b, `<path d="M %s %s C %s %s %s %s %s %s L %s %s C %s %s %s %s %s %s Z" fill="black"/>`,
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
// _ on the right, | across it.
func (e *Engine) drawXyLabels(b *strings.Builder, a xyArrow, mx, my, ux, uy float64) {
	for _, l := range a.labels {
		n := e.makeMath(l.text, false)
		if n.svg == "" {
			continue
		}
		w, h := spToPt(n.width), spToPt(n.height+n.depth)
		// Perpendicular to travel, far enough out to clear the rule and the text.
		//
		// (-uy, ux) turns +90 degrees in a y-DOWNWARD frame, which is RIGHT of the
		// direction of travel. XY-pic's _ is the right side and ^ the left — so a
		// downward \ar[d]_f carries its f on the page's LEFT, which is where the
		// reference puts it.
		off := h/2 + 2
		px, py := -uy, ux
		cx, cy := mx, my
		switch l.side {
		case '_':
			cx, cy = mx+px*off, my+py*off
		case '^':
			cx, cy = mx-px*off, my-py*off
		}
		fmt.Fprintf(b, `<g transform="translate(%s,%s)">%s</g>`, f(cx-w/2), f(cy-h/2), n.svg)
	}
}
