// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements hyperref's INTERNAL (in-document) navigation commands:
//
//   - \hypertarget{name}{text} typesets text normally and marks it as a named
//     destination the document can jump to (an anchor).
//   - \hyperlink{name}{text}   typesets text normally and makes it a clickable
//     same-document link to the destination named name.
//
// Together they turn the engine's SVG into a navigable document: the SVG driver
// emits <g id="name"> around a target and <a href="#name"> around a link, so a
// click on the link scrolls the browser to the target's anchor. This complements
// \href (external links, see hyperlink.go), which emits <a href="URL" target=…>.
//
// Both commands read their first {name} raw (like \href's {URL}) so the anchor
// name is stable and never rewritten by catcodes or macro expansion, and their
// second {text} through the normal box-building path so macros there still expand
// (e.g. a document may wrap the link text in \textcolor). The content is packed
// into an hbox wrapped in an internalLinkNode — the same "box wrapper" shape as
// linkNode/frameNode — so it packs, breaks across lines and paints like any box.
//
// go-pdfkit exposes no named-destination or link-annotation API (its Page and
// Document types offer no Dest/Destination/GoTo/Link/Annotation/Outline method —
// verified with `go doc github.com/go-pdfkit/pdfkit` and `.Page`/`.Document`), so
// the PDF driver paints the inner content without a clickable jump or a reachable
// anchor; the live internal navigation is carried by the SVG driver. When pdfkit
// gains destinations, add a /Dest for a target and a /Link GoTo annot for a link
// in (*pdfDraw).internalLink below.

import (
	"fmt"
	"strings"
)

// internalLinkNode wraps a packed inner hbox as an in-document anchor: when target
// is true it is a named destination (\hypertarget), otherwise a same-document link
// to the destination named name (\hyperlink). It carries its inner box's reference
// dimensions, so it is dimensionally transparent like linkNode.
type internalLinkNode struct {
	name   string
	target bool // true: \hypertarget (destination); false: \hyperlink (jump)
	inner  *boxNode
}

func (internalLinkNode) isNode() {}

// width, height and depth are the inner box's dimensions: the node adds no space.
func (n internalLinkNode) width() int  { return n.inner.width }
func (n internalLinkNode) height() int { return n.inner.height }
func (n internalLinkNode) depth() int  { return n.inner.depth }

// doHypertarget implements \hypertarget{name}{text}: it typesets text and marks it
// as the named destination name (an SVG anchor the fragment #name scrolls to).
func (e *Engine) doHypertarget() { e.doInternalLink(true) }

// doHyperlink implements \hyperlink{name}{text}: it typesets text as a clickable
// same-document link jumping to the destination named name (SVG <a href="#name">).
func (e *Engine) doHyperlink() { e.doInternalLink(false) }

// doInternalLink reads {name} raw (so the anchor name stays stable) and {text} as
// a normal hbox list (so macros expand), then places an internalLinkNode inline.
func (e *Engine) doInternalLink(target bool) {
	name, _ := e.readRawBracedArg()
	list, _ := e.grabHboxList()
	e.placeInline(internalLinkNode{name: name, target: target, inner: hpackSP(list, packNatural, 0)})
}

// paintInternalLinkSP paints an in-document anchor: a destination becomes an SVG
// group carrying id="name" (the fragment #name jumps to it); a link becomes an SVG
// <a href="#name"> (a same-document click, no target=_blank). The name is escaped
// for the id/href attribute. x is the node's left edge; baseline is the content
// baseline.
func paintInternalLinkSP(sb *strings.Builder, n internalLinkNode, x, baseline float64, font fontFace, tc *textCursor) {
	if n.target {
		fmt.Fprintf(sb, `<g id="%s">`, escapeXMLAttr(n.name))
		paintBoxSP(sb, n.inner, x, baseline, font, tc)
		sb.WriteString(`</g>`)
		return
	}
	fmt.Fprintf(sb, `<a href="#%s">`, escapeXMLAttr(n.name))
	paintBoxSP(sb, n.inner, x, baseline, font, tc)
	sb.WriteString(`</a>`)
}

// internalLink paints an in-document anchor's inner box. go-pdfkit exposes no
// named-destination or link-annotation API (see the file comment), so the PDF
// renders the content without a reachable anchor or clickable jump; the SVG driver
// carries the live navigation. When pdfkit gains destinations, register a /Dest for
// a target and a /Link GoTo annotation over Rect (x, baseline-height) ..
// (x+width, baseline+depth) for a link here.
func (d *pdfDraw) internalLink(n internalLinkNode, x, baseline float64) {
	d.box(n.inner, x, baseline)
}
