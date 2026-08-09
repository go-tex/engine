// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strconv"
	"strings"
)

// This file is the page builder (stacking paragraph boxes into pages of a given
// height) and an SVG driver that paints the box tree to a viewable page. A PDF
// driver (via go-pdfkit) consumes the same box tree; SVG is the immediate,
// self-contained way to *see* what the engine produced.

// BuildPages stacks paragraph boxes into pages no taller than vsize, inserting
// parskip between paragraphs. A paragraph taller than vsize occupies its own
// (overfull) page rather than being split (block splitting comes later).
func BuildPages(paras []VBox, vsize, parskip float64) []VBox {
	var pages []VBox
	var cur []Node
	var h float64
	flush := func() {
		if len(cur) > 0 {
			pages = append(pages, vpackTight(cur))
			cur, h = nil, 0
		}
	}
	for _, p := range paras {
		ph := p.H + p.D
		if h > 0 && h+parskip+ph > vsize {
			flush()
		}
		if len(cur) > 0 {
			cur = append(cur, SetGlue{W: parskip, Set: parskip})
			h += parskip
		}
		cur = append(cur, p)
		h += ph
	}
	flush()
	return pages
}

// vpackTight stacks nodes at their natural heights (no baselineskip logic; the
// nodes are already-packed paragraph boxes and inter-paragraph glue).
func vpackTight(list []Node) VBox {
	var w, total, lastDepth float64
	for _, n := range list {
		nw, nh, nd := n.dims()
		if nw > w {
			w = nw
		}
		total += nh + nd
		lastDepth = nd
	}
	return VBox{W: w, H: total - lastDepth, D: lastDepth, List: list}
}

// RenderPageSVG paints a page box onto an SVG of the given dimensions, with the
// content offset by (marginX, marginY). Glyphs are drawn as vector paths from
// the font; rules as rectangles.
func RenderPageSVG(page VBox, font *OpenTypeFont, pageW, pageH, marginX, marginY float64) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="0 0 %s %s">`,
		f(pageW), f(pageH), f(pageW), f(pageH))
	b.WriteString(`<rect width="100%" height="100%" fill="white"/>`)
	b.WriteString(`<g fill="black">`)
	paintV(&b, page, font, marginX, marginY)
	b.WriteString(`</g></svg>`)
	return b.String()
}

// paintV paints a vertical box whose top-left is (x, top): it stacks children,
// advancing the vertical cursor by each child's height+depth (and glue by its
// set size).
func paintV(b *strings.Builder, box VBox, font *OpenTypeFont, x, top float64) {
	y := top
	for _, n := range box.List {
		switch c := n.(type) {
		case HBox:
			paintH(b, c, font, x, y+c.H) // baseline = top + height
			y += c.H + c.D
		case VBox:
			paintV(b, c, font, x, y)
			y += c.H + c.D
		case SetGlue:
			y += c.Set
		case Rule:
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s"/>`, f(x), f(y), f(c.W), f(c.H+c.D))
			y += c.H + c.D
		}
	}
}

// paintH paints a horizontal box on the given baseline, advancing the horizontal
// cursor by each child's width (glue by its set size).
func paintH(b *strings.Builder, box HBox, font *OpenTypeFont, x, baseline float64) {
	cx := x
	for _, n := range box.List {
		switch c := n.(type) {
		case Char:
			if font != nil {
				if d := font.glyphPath(c.R); d != "" {
					fmt.Fprintf(b, `<path transform="translate(%s,%s)" d="%s"/>`, f(cx), f(baseline), d)
				}
			}
			cx += c.W
		case SetGlue:
			cx += c.Set
		case HBox:
			paintH(b, c, font, cx, baseline)
			cx += c.W
		case Rule:
			fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s"/>`, f(cx), f(baseline-c.H), f(c.W), f(c.H+c.D))
			cx += c.W
		}
	}
}

// f formats a float compactly for SVG.
func f(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
