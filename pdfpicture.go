// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strings"

// This file puts the PDF driver's TEXT inside the coordinate scopes a drawing
// package opens, so both drivers agree about where a picture's words go.
//
// The two drivers see a picture differently. The SVG driver writes glyphs into
// the same stream as the driver literals, so a scope opened by a literal
// naturally contains whatever is painted after it: a TikZ node's text lands
// inside the picture's transform without the driver doing anything. The PDF
// driver draws glyphs as it walks the box tree and interprets the literals
// separately (svgpdf.go), so text used to ignore the scopes entirely and every
// node's words piled up at the picture's origin.
//
// The fix is to track, while walking, the transformation the literals have in
// force — the composition of the transforms of every <g> scope still open — and
// draw a character through it. The transforms are in the same page coordinates
// the literals use (y down from the top-left); PDF space is y up from the
// bottom-left, so the map is conjugated by that flip before it goes into the
// content stream.

// pictureCTM tracks the transformation the open literal scopes impose, in the
// literals' own (SVG page) coordinates.
type pictureCTM struct {
	stack []affine // one entry per open <g>, each the full map at that depth
	org   originResolver
}

// cur is the transformation in force: identity when no scope is open.
func (p *pictureCTM) cur() affine {
	if len(p.stack) == 0 {
		return identity()
	}
	return p.stack[len(p.stack)-1]
}

// active reports whether any scope imposes a transformation, so the common case
// (no picture, or scopes that only set colours) costs nothing.
func (p *pictureCTM) active() bool { return p.cur() != identity() }

// feed consumes one finished literal, opening and closing scopes as it goes, and
// returns it unchanged for the vector interpreter. Each literal is a FRAGMENT —
// a scope is opened by one and closed by another — so the tags are read directly
// rather than parsed as a document, which would close every open tag at the end
// of the fragment it appeared in. Only <g> matters here: what the shapes are is
// svgpdf.go's business.
func (p *pictureCTM) feed(lit string) string {
	for i := 0; i < len(lit); {
		lt := strings.IndexByte(lit[i:], '<')
		if lt < 0 {
			break
		}
		i += lt
		gt := strings.IndexByte(lit[i:], '>')
		if gt < 0 {
			break
		}
		tag := lit[i : i+gt+1]
		i += gt + 1
		switch {
		case strings.HasPrefix(tag, "</g"):
			if len(p.stack) > 0 {
				p.stack = p.stack[:len(p.stack)-1]
			}
		case strings.HasPrefix(tag, "<g") && !strings.HasSuffix(tag, "/>") &&
			(len(tag) == 3 || tag[2] == ' ' || tag[2] == '>'):
			p.stack = append(p.stack, p.cur().mul(parseTransform(tagAttr(tag, "transform"))))
		}
	}
	return lit
}

// tagAttr reads an attribute's value out of a tag's source text.
func tagAttr(tag, name string) string {
	key := " " + name + `="`
	i := strings.Index(tag, key)
	if i < 0 {
		return ""
	}
	rest := tag[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// pdfMap turns a map written in the literals' page coordinates into the same map
// in PDF user space, whose y runs the other way from the other corner: the flip
// F(x,y) = (x, H−y) is its own inverse, so the PDF map is F ∘ m ∘ F.
func pdfMap(m affine, pageH float64) affine {
	return affine{
		a: m.a, b: -m.b,
		c: -m.c, d: m.d,
		e: m.c*pageH + m.e,
		f: pageH - m.d*pageH - m.f,
	}
}
