// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"io"
	"strings"

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
	vmargin := e.renderVMargin(margin)
	paperW, paperH, _ := e.paperSizePt()
	for _, page := range e.Pages() {
		pw := spToPt(page.width) + 2*margin
		ph := spToPt(page.height+page.depth) + 2*vmargin
		if paperW > 0 && paperH > 0 {
			pw, ph = paperW, paperH
		}
		p := doc.AddPage(pdfkit.NewPageSize(pw, ph))
		p.SetFont(face, size)
		d := &pdfDraw{p: p, face: face, size: size, cur: size, pageH: ph}
		if e.hasPageColor { // \pagecolor: fill the whole page behind the content
			d.setColor(e.pageColor)
			d.rect(0, 0, pw, ph)
			d.setColor(0)
		}
		d.box(page, margin, vmargin+spToPt(page.height))
		d.drawSpecials()
	}
	// Section headings become the PDF's outline (the viewer's bookmark sidebar):
	// every \@tocentry of kind "toc" (a sectioning entry, not a caption) is an item
	// at its level, jumping to the page its anchor fell on. AddOutlineItem ignores an
	// out-of-range page.
	for _, te := range e.tocEntries {
		if te.kind != "toc" {
			continue
		}
		if pg := e.pageOfIndex(te.marker); pg >= 1 {
			doc.AddOutlineItem(te.title, te.level, pg-1)
		}
	}
	return doc.Write(w)
}

// pdfDraw carries the drawing state for one page.
type pdfDraw struct {
	p        *pdfkit.Page
	face     *pdfkit.Font
	size     float64 // base (\normalsize) size
	cur      float64 // font size currently selected in the content stream
	curColor uint32  // fill colour currently selected (0xRRGGBB; 0 = black)
	pageH    float64
	specials []string   // driver literals collected in paint order (see special)
	ctm      pictureCTM // the transformation those literals have in force
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

// setColor switches the PDF fill colour (used by both text and rectangles) when it
// differs from what's selected. Colour 0 means black.
func (d *pdfDraw) setColor(c uint32) {
	if c == d.curColor {
		return
	}
	d.curColor = c
	d.p.SetFillColor(pdfkit.RGB8(uint8(c>>16), uint8(c>>8), uint8(c)))
}

// y flips an engine (top-down) y coordinate into PDF (bottom-up) space.
func (d *pdfDraw) y(engineY float64) float64 { return d.pageH - engineY }

// ligatureText maps an f-ligature codepoint to its component ASCII letters. The
// paragraph builder folds "fi"→U+FB01 etc. for shaping; drawing the ligature glyph
// through TextShaped with those component letters as the source makes the PDF's
// /ToUnicode CMap map the glyph back to plain ASCII, so a copy or full-text search
// of a ligated word ("first") recovers "first", not the raw ligature codepoint.
var ligatureText = map[rune]string{
	'ﬀ': "ff", 'ﬁ': "fi", 'ﬂ': "fl", 'ﬃ': "ffi", 'ﬄ': "ffl",
}

// drawChar draws one character at (x, y) in PDF space. An f-ligature is drawn via
// TextShaped from its component letters (so the ligature glyph is produced by the
// font's liga feature while the text layer stays ASCII); any other character goes
// through the plain cmap Text path.
func (d *pdfDraw) drawChar(x, y float64, ch rune) {
	if s, ok := ligatureText[ch]; ok {
		if err := d.p.TextShaped(x, y, s, "liga"); err == nil {
			return
		}
	}
	d.p.Text(x, y, string(ch))
}

// drawCharMapped draws a character at the engine position (x, baseline), through
// the transformation the open picture scopes impose. Outside a picture there is
// none and the character is drawn straight, exactly as before.
func (d *pdfDraw) drawCharMapped(x, baseline float64, ch rune) {
	if !d.ctm.active() {
		d.drawChar(x, d.y(baseline), ch)
		return
	}
	m := pdfMap(d.ctm.cur(), d.pageH)
	d.p.Save()
	d.p.Transform(m.a, m.b, m.c, m.d, m.e, m.f)
	d.drawChar(x, d.y(baseline), ch)
	d.p.Restore()
}

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
			d.setColor(c.color)
			d.drawCharMapped(cx, baseline, c.ch)
			cx += spToPt(c.width)
		case ruleNode:
			d.setColor(0)
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
		case linkNode:
			d.link(c, cx, baseline)
			cx += spToPt(c.width())
		case internalLinkNode:
			d.internalLink(c, cx, baseline)
			cx += spToPt(c.width())
		case decoNode:
			d.deco(c, cx, baseline)
			cx += spToPt(c.width())
		case transformNode:
			d.transform(c, cx, baseline)
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
		case specialNode:
			d.special(c, cx, baseline)
		}
	}
}

// special records a driver literal with the reference point it reached, in the
// order the page paints it. The literals are drawn together once the page's box
// tree has been walked (see drawSpecials): a pgf picture's operators arrive as
// many specials that open and close shared graphics scopes, so they are
// interpreted as one stream, not one by one.
func (d *pdfDraw) special(s specialNode, x, baseline float64) {
	lit, ok := specialLiteral(s.text, x, baseline)
	if !ok {
		return
	}
	// Finish the literal here rather than over the whole page: the scopes it
	// opens have to be known now, since a character drawn after it is drawn
	// through them (see pdfpicture.go).
	lit = d.ctm.org.next(lit)
	d.specials = append(d.specials, d.ctm.feed(lit))
}

// drawSpecials interprets the page's collected driver literals as vector marks.
// It runs after the text and boxes so a picture's paths sit over the page, and
// resets the tracked fill colour afterwards, since drawing selects its own.
func (d *pdfDraw) drawSpecials() {
	if len(d.specials) == 0 {
		return
	}
	drawSVGStream(d.p, strings.Join(d.specials, ""), d.pageH)
	d.specials = nil
	d.curColor = 0
	d.p.SetFillColor(pdfkit.RGB8(0, 0, 0))
}

// frame draws a framed box: four filled rules of thickness fr.rule border the
// content, which is then painted on the same baseline, inset by rule+sep from the
// left edge. x is the frame's left edge; baseline is the content baseline.
func (d *pdfDraw) frame(fr frameNode, x, baseline float64) {
	w := spToPt(fr.width())
	top := baseline - spToPt(fr.height())
	total := spToPt(fr.height() + fr.depth())
	if fr.bg != 0 { // \colorbox/\fcolorbox background
		d.setColor(fr.bg)
		d.rect(x, top, w, total)
	}
	if fr.rule > 0 { // border (black for \fbox, coloured for \fcolorbox)
		d.setColor(fr.ruleColor)
		ru := spToPt(fr.rule)
		d.rect(x, top, w, ru)          // top edge
		d.rect(x, top+total-ru, w, ru) // bottom edge
		d.rect(x, top, ru, total)      // left edge
		d.rect(x+w-ru, top, ru, total) // right edge
	}
	d.box(fr.inner, x+spToPt(fr.rule+fr.sep), baseline)
}

// link paints a hyperlink's inner box and adds a clickable /Link annotation over
// it, so \href/\url is navigable in the PDF just as the SVG driver's <a href> makes
// it in SVG (see paintLinkSP). The annotation rectangle is the inner box —
// engine-top-down (x, baseline-height) .. (x+width, baseline+depth) — flipped into
// PDF's y-up default user space, which is where an annotation /Rect lives.
func (d *pdfDraw) link(ln linkNode, x, baseline float64) {
	d.box(ln.inner, x, baseline)
	if ln.url == "" {
		return
	}
	d.p.AddLink(pdfkit.Rect{
		X:      x,
		Y:      d.y(baseline + spToPt(ln.inner.depth)),
		Width:  spToPt(ln.inner.width),
		Height: spToPt(ln.inner.height + ln.inner.depth),
	}, ln.url)
}

// deco draws a text decoration: the inner box on the baseline, then a rule of
// thickness decoRule at the decoration's y position, in its colour.
func (d *pdfDraw) deco(dn decoNode, x, baseline float64) {
	d.box(dn.inner, x, baseline)
	d.setColor(dn.color)
	d.rect(x, baseline+spToPt(dn.decoRuleTop()), spToPt(dn.width()), spToPt(decoRule))
	d.setColor(0)
}

// transform draws a graphicx box transformation (\scalebox/\reflectbox/
// \resizebox/\rotatebox) using go-pdfkit's CTM: it saves the graphics state,
// concatenates the affine map about the content reference point, paints the inner
// box, then restores. go-pdfkit exposes a full matrix API (Save/Restore/Transform
// — verified with `go doc github.com/go-pdfkit/pdfkit.Page`), so the PDF driver
// realises these transforms just as faithfully as the SVG driver; unlike the
// \href link-annotation limitation, there is no shortfall here.
//
// The reference point sits at engine (x+refDX, baseline); in PDF (y-up) space
// that is Pref = (prefX, pageH-baseline). The concatenated matrix is
// translate(Pref)·L·translate(-Pref) with L the math-space linear map — which,
// because PDF and the math frame are both y-up, uses the coefficients unchanged.
// pdfkit's Transform(a,b,c,d,e,f) maps (x,y) to (a*x+c*y+e, b*x+d*y+f). The saved
// state also restores the fill colour and font size, so the tracked cur/curColor
// fields are rewound to match, keeping later state changes correct.
func (d *pdfDraw) transform(tn transformNode, x, baseline float64) {
	savedColor, savedSize := d.curColor, d.cur
	d.p.Save()
	prefX := x + spToPt(tn.refDX)
	prefY := d.y(baseline)
	e := prefX - (tn.a*prefX + tn.c*prefY)
	fY := prefY - (tn.b*prefX + tn.d*prefY)
	d.p.Transform(tn.a, tn.b, tn.c, tn.d, e, fY)
	d.box(tn.inner, prefX, baseline)
	d.p.Restore()
	d.curColor, d.cur = savedColor, savedSize
}

// drawImage embeds an image XObject: PNG/JPEG go in verbatim, an SVG is drawn as
// vector paths (drawSVGImage, see svgimage.go), anything else is decoded and
// re-encoded by go-pdfkit. drawSVGImage changes the page fill colour as it selects
// per-shape fills, so afterwards the tracked colour is re-selected to keep later
// text/rule colours correct.
func (d *pdfDraw) drawImage(n imageNode, r pdfkit.Rect) {
	switch n.format {
	case imgPNG:
		_ = d.p.DrawPNG(n.data, r)
	case imgJPEG:
		_ = d.p.DrawJPEG(n.data, r)
	case imgSVG:
		drawSVGImage(d.p, n.data, r)
		d.p.SetFillColor(pdfkit.RGB8(uint8(d.curColor>>16), uint8(d.curColor>>8), uint8(d.curColor)))
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
			d.setColor(0)
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
		case decoNode:
			d.deco(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case transformNode:
			d.transform(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case linkNode:
			d.link(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case internalLinkNode:
			d.internalLink(c, x, cy+spToPt(c.height()))
			cy += spToPt(c.height() + c.depth())
		case mathNode:
			drawMathSVG(d.p, c.svg, x, d.y(cy))
			cy += spToPt(c.height + c.depth)
		case specialNode:
			d.special(c, x, cy)
		}
	}
}

// leader paints a glue node's set width as a \leaders-like fill: leaderRule draws
// a thin baseline rule (\hrulefill), leaderDots tiles '.' glyphs (\dotfill).
func (d *pdfDraw) leader(kind glueLeader, x, baseline, w float64) {
	switch kind {
	case leaderRule:
		d.setColor(0)
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
