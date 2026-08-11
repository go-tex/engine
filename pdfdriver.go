// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"github.com/go-pdfkit/pdfkit"
)

// This file is the PDF driver: it renders the paginated box tree to a real PDF
// via go-pdfkit, which embeds the font as a subset and draws glyphs as text (so
// the output is selectable/copy-pasteable). It is the counterpart of the SVG
// driver over the same sp box tree — the output format TeX exists to produce.
// PDF space is points with the origin at the lower-left and y increasing upward,
// so engine y (down from the top) maps to pageH − y. Math (a go-tex/math SVG) is
// translated to PDF vector paths by drawMathSVG (see mathpdf.go), so it appears
// in the PDF, not only the SVG driver.

// RenderPDF writes the main vertical list, split into \vsize pages, as a PDF to w.
// Each page is (content width + 2·margin) × (content height + 2·margin) points.
// A current OpenType font (with embeddable bytes) is required to draw text.
func (e *Engine) RenderPDF(w io.Writer, margin float64) error {
	ef, ok := e.renderFont().(embeddableFont)
	if !ok {
		return fmt.Errorf("texengine: RenderPDF needs an embeddable font (set via \\font)")
	}
	face, err := pdfkit.LoadFont(ef.fontBytes())
	if err != nil {
		return fmt.Errorf("texengine: PDF font load: %w", err)
	}
	size := float64(ef.sizePt())
	doc := pdfkit.New(pdfkit.Options{})
	for _, page := range e.Pages() {
		pw := spToPt(page.width) + 2*margin
		ph := spToPt(page.height+page.depth) + 2*margin
		p := doc.AddPage(pdfkit.NewPageSize(pw, ph))
		p.SetFont(face, size)
		d := &pdfDraw{p: p, face: face, size: size, cur: size, pageH: ph}
		d.box(page, margin, margin+spToPt(page.height))
	}
	return doc.Write(w)
}

// pdfDraw carries the drawing state for one page.
type pdfDraw struct {
	p     *pdfkit.Page
	face  *pdfkit.Font
	size  float64 // base (\normalsize) size
	cur   float64 // font size currently selected in the content stream
	pageH float64
}

// setSize switches the PDF text size when a glyph's size differs from what's
// currently selected (same embedded face, different Tf size).
func (d *pdfDraw) setSize(sz float64) {
	if sz <= 0 {
		sz = d.size
	}
	if sz != d.cur {
		d.p.SetFont(d.face, sz)
		d.cur = sz
	}
}

// y flips an engine (top-down) y coordinate into PDF (bottom-up) space.
func (d *pdfDraw) y(engineY float64) float64 { return d.pageH - engineY }

// box paints a box with left edge x and the given baseline (engine coordinates).
func (d *pdfDraw) box(b *boxNode, x, baseline float64) {
	if b.kind == hbox {
		d.hlist(b, x, baseline)
	} else {
		d.vlist(b, x, baseline-spToPt(b.height))
	}
}

// hlist lays an hbox's material along the baseline.
func (d *pdfDraw) hlist(b *boxNode, x, baseline float64) {
	cx := x
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cx += spToPt(c.width)
		case glueNode:
			w := spToPt(b.setWidth(c.spec))
			d.leader(c.leader, cx, baseline, w)
			cx += w
		case charNode:
			d.setSize(float64(c.size))
			d.p.Text(cx, d.y(baseline), string(c.ch))
			cx += spToPt(c.width)
		case ruleNode:
			h := spToPt(ruleHeight(c, b))
			dp := spToPt(ruleDepth(c, b))
			w := spToPt(c.width)
			d.rect(cx, baseline-h, w, h+dp)
			cx += w
		case *boxNode:
			d.box(c, cx, baseline+spToPt(c.shift))
			cx += spToPt(c.width)
		case frameNode:
			d.frame(c, cx, baseline)
			cx += spToPt(c.width())
		case mathNode:
			drawMathSVG(d.p, c.svg, cx, d.y(baseline-spToPt(c.height)))
			cx += spToPt(c.width)
		case imageNode:
			// The image's lower-left corner sits on the baseline; PDF user space is
			// y-up, so the rect origin is (cx, pageH-baseline) with height upward.
			r := pdfkit.Rect{X: cx, Y: d.y(baseline), Width: spToPt(c.width), Height: spToPt(c.height)}
			d.drawImage(c, r)
			cx += spToPt(c.width)
		}
	}
}

// frame draws a framed box: four filled rules of thickness fr.rule border the
// content, which is then painted on the same baseline, inset by rule+sep from the
// left edge. x is the frame's left edge; baseline is the content baseline.
func (d *pdfDraw) frame(fr frameNode, x, baseline float64) {
	w := spToPt(fr.width())
	top := baseline - spToPt(fr.height())
	total := spToPt(fr.height() + fr.depth())
	ru := spToPt(fr.rule)
	d.rect(x, top, w, ru)          // top edge
	d.rect(x, top+total-ru, w, ru) // bottom edge
	d.rect(x, top, ru, total)      // left edge
	d.rect(x+w-ru, top, ru, total) // right edge
	d.box(fr.inner, x+spToPt(fr.rule+fr.sep), baseline)
}

// drawImage embeds an image XObject: PNG/JPEG go in verbatim, anything else is
// decoded and re-encoded by go-pdfkit.
func (d *pdfDraw) drawImage(n imageNode, r pdfkit.Rect) {
	switch n.format {
	case "png":
		_ = d.p.DrawPNG(n.data, r)
	case "jpeg":
		_ = d.p.DrawJPEG(n.data, r)
	default:
		if img, _, err := image.Decode(bytes.NewReader(n.data)); err == nil {
			d.p.DrawImage(img, r)
		}
	}
}

// vlist stacks a vbox's material from the top edge.
func (d *pdfDraw) vlist(b *boxNode, x, top float64) {
	cy := top
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cy += spToPt(c.width)
		case glueNode:
			cy += spToPt(b.setWidth(c.spec))
		case ruleNode:
			w := spToPt(ruleWidth(c, b))
			hd := spToPt(c.height + c.depth)
			d.rect(x, cy, w, hd)
			cy += hd
		case *boxNode:
			d.box(c, x+spToPt(c.shift), cy+spToPt(c.height))
			cy += spToPt(c.height + c.depth)
		case frameNode:
			d.frame(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case mathNode:
			drawMathSVG(d.p, c.svg, x, d.y(cy))
			cy += spToPt(c.height + c.depth)
		}
	}
}

// leader paints a glue node's set width as a \leaders-like fill: leaderRule draws
// a thin baseline rule (\hrulefill), leaderDots tiles '.' glyphs (\dotfill).
func (d *pdfDraw) leader(kind glueLeader, x, baseline, w float64) {
	switch kind {
	case leaderRule:
		th := spToPt(defaultRule)
		d.rect(x, baseline-th, w, th)
	case leaderDots:
		n, cell := dotLeaderGeom(w, d.size)
		for i := 0; i < n; i++ {
			d.p.Text(x+float64(i)*cell, d.y(baseline), ".")
		}
	}
}

// rect fills a rectangle whose top-left is (x, engineTop) in engine coordinates.
func (d *pdfDraw) rect(x, engineTop, w, h float64) {
	d.p.Rectangle(pdfkit.Rect{X: x, Y: d.y(engineTop + h), Width: w, Height: h})
	d.p.Fill()
}
