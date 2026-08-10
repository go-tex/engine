// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"io"

	"github.com/go-pdfkit/pdfkit"
)

// This file is the PDF driver: it renders the paginated box tree to a real PDF
// via go-pdfkit, which embeds the font as a subset and draws glyphs as text (so
// the output is selectable/copy-pasteable). It is the counterpart of the SVG
// driver over the same sp box tree — the output format TeX exists to produce.
// PDF space is points with the origin at the lower-left and y increasing upward,
// so engine y (down from the top) maps to pageH − y.
//
// Limitations: math nodes (SVG from go-tex/math) are not yet drawn into the PDF
// (they are logged as skipped); box shifts and rules are handled.

// RenderPDF writes the main vertical list, split into \vsize pages, as a PDF to w.
// Each page is (content width + 2·margin) × (content height + 2·margin) points.
// A current OpenType font (with embeddable bytes) is required to draw text.
func (e *Engine) RenderPDF(w io.Writer, margin float64) error {
	ef, ok := e.curFont.(embeddableFont)
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
		d := &pdfDraw{p: p, face: face, size: size, pageH: ph}
		d.box(page, margin, margin+spToPt(page.height))
	}
	return doc.Write(w)
}

// pdfDraw carries the drawing state for one page.
type pdfDraw struct {
	p     *pdfkit.Page
	face  *pdfkit.Font
	size  float64
	pageH float64
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
			cx += spToPt(b.setWidth(c.spec))
		case charNode:
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
		case mathNode:
			cx += spToPt(c.width) // math (SVG) not yet drawn into PDF
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
		case mathNode:
			cy += spToPt(c.height + c.depth)
		}
	}
}

// rect fills a rectangle whose top-left is (x, engineTop) in engine coordinates.
func (d *pdfDraw) rect(x, engineTop, w, h float64) {
	d.p.Rectangle(pdfkit.Rect{X: x, Y: d.y(engineTop + h), Width: w, Height: h})
	d.p.Fill()
}
