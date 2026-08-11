// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements hyperref's hyperlink commands: \url, \href and \nolinkurl.
//
//   - \url{URL}        typesets URL literally (in the tt font when one is bound,
//                      otherwise the current font) AND makes it clickable to URL.
//   - \href{URL}{text} typesets text normally and makes it clickable to URL.
//   - \nolinkurl{URL}  typesets URL literally like \url but with NO link.
//
// A URL carries characters — ~, %, #, _, & — whose ordinary catcodes would make
// the tokenizer misread them, so \url and \nolinkurl read their argument raw from
// the base input (exactly as \verb does; see verbatim.go), bypassing catcodes and
// macro expansion. \href reads its first {URL} raw for the same reason and its
// second {text} through the normal box-building path so macros there still expand.
//
// The clickable target is modelled as a linkNode wrapping the content's packed
// hbox — the same "box wrapper" shape as frameNode (see boxframe.go). The SVG
// driver emits an <a href="URL"> around the inner box, so the engine's SVG is a
// genuinely clickable document in a browser (the differentiator gotex targets).
// go-pdfkit exposes no link-annotation API yet, so the PDF driver paints the inner
// box without a clickable rectangle (documented in pdfdriver.go's link method).

import "strings"

// linkNode wraps a packed inner hbox in a hyperlink to url. It carries the same
// reference-point dimensions as its inner box, so it packs, breaks across lines and
// paints like any other box item; the drivers additionally wrap the inner content
// in a clickable target (an SVG <a>; PDF has no link annotation in go-pdfkit yet).
type linkNode struct {
	url   string
	inner *boxNode
}

func (linkNode) isNode() {}

// width, height and depth are the link's reference-point dimensions (sp): it is
// dimensionally transparent, taking exactly its inner box's size.
func (ln linkNode) width() int  { return ln.inner.width }
func (ln linkNode) height() int { return ln.inner.height }
func (ln linkNode) depth() int  { return ln.inner.depth }

// doURL implements \url{URL}: it reads URL literally (URLs carry catcode-active
// characters like ~, #, %, & that must not be interpreted), typesets it in the
// verbatim (tt) font and wraps the result in a hyperlink pointing to itself.
func (e *Engine) doURL() {
	url, _ := e.readRawBracedArg()
	inner := e.urlBox(url)
	if inner == nil {
		return
	}
	e.placeInline(linkNode{url: url, inner: inner})
}

// doNolinkurl implements \nolinkurl{URL}: like \url it typesets URL literally in
// the verbatim font, but produces no clickable link (just the monospace rendering).
func (e *Engine) doNolinkurl() {
	url, _ := e.readRawBracedArg()
	inner := e.urlBox(url)
	if inner == nil {
		return
	}
	e.placeInline(inner)
}

// doHref implements \href{URL}{text}: the first argument is read literally (as for
// \url) so URL characters keep their meaning; the second is composed normally
// (macros expand) and the whole is wrapped in a hyperlink pointing to URL.
func (e *Engine) doHref() {
	url, _ := e.readRawBracedArg()
	list, _ := e.grabHboxList()
	e.placeInline(linkNode{url: url, inner: hpackSP(list, packNatural, 0)})
}

// urlBox typesets a literal URL string as a natural-width hbox in the verbatim
// (tt) font, or nil when no font is available to measure with.
func (e *Engine) urlBox(url string) *boxNode {
	font := e.verbFont()
	if font == nil {
		return nil
	}
	return hpackSP(e.verbNodes(url, font, e.curSrcLine), packNatural, 0)
}

// placeInline adds an inline node to the current paragraph, starting one (indented)
// when in vertical mode — links and URLs join the running text like \verb material.
func (e *Engine) placeInline(n node) {
	if !e.inPar {
		e.beginParagraph(true)
	}
	e.parList = append(e.parList, n)
}

// readRawBracedArg reads a brace-delimited argument literally from the base input:
// it skips leading spaces, requires a '{', then returns the runes up to the matching
// '}' (tracking nested brace depth) without tokenizing or expanding them — the
// reading discipline \url, \nolinkurl and \href's URL need so characters like ~, %,
// #, _ and & keep their literal value. ok is false when no '{' follows (the cursor
// is left unchanged in that case). An unterminated argument consumes to end of input.
func (e *Engine) readRawBracedArg() (string, bool) {
	p := e.bpos
	for p < len(e.base) {
		if cc := e.catOf(e.base[p]); cc != catSpace && cc != catEOL {
			break
		}
		p++
	}
	if p >= len(e.base) || e.base[p] != '{' {
		return "", false
	}
	p++ // consume the opening '{'
	start := p
	depth := 1
	for p < len(e.base) {
		switch e.base[p] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				text := string(e.base[start:p])
				e.bpos = p + 1 // consume the closing '}'
				return text, true
			}
		}
		p++
	}
	e.bpos = p
	return string(e.base[start:p]), false
}

// escapeXMLAttr escapes a string for use inside a double-quoted XML/SVG attribute,
// so a URL containing &, <, >, " or ' cannot break out of the href="…" it fills.
func escapeXMLAttr(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	).Replace(s)
}
