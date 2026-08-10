// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
)

// This file renders the stomach's scaled-point box tree (boxNode) to SVG. It is
// the driver that makes an explicit \hbox/\vbox *visible*: rules paint as filled
// rectangles, glue and kern advance the cursor (glue at its set width), and boxes
// nest recursively. Coordinates are emitted in points (1pt = 1 user unit), so the
// geometry is exactly the sp box tree divided by 65536. A PDF driver (go-pdfkit)
// will consume the same tree; SVG is the self-contained way to see it.

// spToPt converts scaled points to points as a float for output coordinates.
func spToPt(sp int) float64 { return float64(sp) / float64(unity) }

// RenderBox renders box register i to an SVG string with a uniform margin (pt).
// Empty if the register is void.
func (e *Engine) RenderBox(i int, margin float64) string {
	return renderBoxSVG(e.getBox(i), margin, e.curFont)
}

// Page vpacks the main vertical list (everything contributed at top level) into a
// single vbox at natural height. Empty (nil) if nothing was contributed.
func (e *Engine) Page() *boxNode {
	if len(e.mvl) == 0 {
		return nil
	}
	return vpackSP(e.mvl, packNatural, 0)
}

// RenderPage renders the main vertical list to an SVG page with the given margin.
func (e *Engine) RenderPage(margin float64) string {
	return renderBoxSVG(e.Page(), margin, e.curFont)
}

// renderBoxSVG paints a packed box onto an SVG sized to the box plus a uniform
// margin (in points). The box's reference point (left edge, baseline) sits at
// (margin, margin+height). font (may be nil) draws character glyphs.
func renderBoxSVG(b *boxNode, margin float64, font fontFace) string {
	if b == nil {
		return ""
	}
	pageW := spToPt(b.width) + 2*margin
	pageH := spToPt(b.height+b.depth) + 2*margin
	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%spt" height="%spt" viewBox="0 0 %s %s">`,
		f(pageW), f(pageH), f(pageW), f(pageH))
	sb.WriteString(`<rect width="100%" height="100%" fill="white"/><g fill="black">`)
	paintBoxSP(&sb, b, margin, margin+spToPt(b.height), font)
	sb.WriteString(`</g></svg>`)
	return sb.String()
}

// paintBoxSP paints a box whose left edge is at x and whose baseline is at the
// given y (SVG coordinates, points).
func paintBoxSP(sb *strings.Builder, b *boxNode, x, baseline float64, font fontFace) {
	if b.kind == hbox {
		paintHListSP(sb, b, x, baseline, font)
	} else {
		paintVListSP(sb, b, x, baseline-spToPt(b.height), font)
	}
}

// paintHListSP lays an hbox's material along the baseline, advancing the cursor
// by each item's (set) width.
func paintHListSP(sb *strings.Builder, b *boxNode, x, baseline float64, font fontFace) {
	cx := x
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cx += spToPt(c.width)
		case glueNode:
			cx += spToPt(b.setWidth(c.spec))
		case charNode:
			if font != nil {
				if d := font.glyphPathAt(c.ch); d != "" {
					fmt.Fprintf(sb, `<path transform="translate(%s,%s)" d="%s"/>`, f(cx), f(baseline), d)
				}
			}
			cx += spToPt(c.width)
		case mathNode:
			// embed the math SVG with its top at (cx, baseline-height): centred
			fmt.Fprintf(sb, `<g transform="translate(%s,%s)">%s</g>`, f(cx), f(baseline-spToPt(c.height)), c.svg)
			cx += spToPt(c.width)
		case ruleNode:
			h := ruleHeight(c, b)
			d := ruleDepth(c, b)
			w := spToPt(c.width) // an hbox never has a running-width rule
			rect(sb, cx, baseline-spToPt(h), w, spToPt(h+d))
			cx += w
		case *boxNode:
			paintBoxSP(sb, c, cx, baseline+spToPt(c.shift), font) // shift>0 lowers the box
			cx += spToPt(c.width)
		}
	}
}

// paintVListSP stacks a vbox's material from the top edge (top), advancing the
// vertical cursor by each item's size and carrying the running depth.
func paintVListSP(sb *strings.Builder, b *boxNode, x, top float64, font fontFace) {
	cy := top
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cy += spToPt(c.width)
		case glueNode:
			cy += spToPt(b.setWidth(c.spec))
		case ruleNode:
			w := ruleWidth(c, b)
			hd := spToPt(c.height + c.depth)
			rect(sb, x, cy, spToPt(w), hd)
			cy += hd
		case *boxNode:
			paintBoxSP(sb, c, x+spToPt(c.shift), cy+spToPt(c.height), font)
			cy += spToPt(c.height + c.depth)
		case mathNode: // display math on its own line
			fmt.Fprintf(sb, `<g transform="translate(%s,%s)">%s</g>`, f(x), f(cy), c.svg)
			cy += spToPt(c.height + c.depth)
		}
	}
}

// A running rule dimension follows the enclosing box; these resolve it.
func ruleWidth(r ruleNode, b *boxNode) int {
	if r.widthRun {
		return b.width
	}
	return r.width
}
func ruleHeight(r ruleNode, b *boxNode) int {
	if r.heightRun {
		return b.height
	}
	return r.height
}
func ruleDepth(r ruleNode, b *boxNode) int {
	if r.depthRun {
		return b.depth
	}
	return r.depth
}

// rect writes one filled rectangle (points).
func rect(sb *strings.Builder, x, y, w, h float64) {
	fmt.Fprintf(sb, `<rect x="%s" y="%s" width="%s" height="%s"/>`, f(x), f(y), f(w), f(h))
}
