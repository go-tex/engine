// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
)

// This file is the engine's graphics-literal seam: TeX's \special primitive and
// the whatsit node it produces. A \special carries driver-specific text that the
// typesetter never interprets — it has no width, height or depth, so it rides
// along in a horizontal or vertical list without disturbing the layout, and each
// output driver decides what to do with it when the page is shipped out.
//
// It is the hook a graphics package targets: pgf/TikZ (like every other TeX
// drawing package) draws by emitting a stream of driver operators through
// \special, which the driver turns into real marks on the page. The engine's
// own driver namespace is "gotex:": its payload is an SVG fragment, emitted
// verbatim by the SVG driver and interpreted into vector operators by the PDF
// driver (see svgpdf.go). Specials in any other namespace (dvips:, pdf:, …) are
// carried but drawn by nobody, exactly as a driver that does not know them.
//
// Position placeholders, following the dvisvgm convention pgf already targets:
// a literal's {?x} and {?y} are substituted at shipout with the special's own
// reference point on the page (in points, SVG coordinates: origin top-left, y
// down), and {?nl} with a newline. That lets a picture be positioned by the
// typesetter — TeX boxes the picture, the driver draws it where the box landed.

// gotexSpecial is the driver namespace whose payload this engine draws.
const gotexSpecial = "gotex:"

// specialNode is a whatsit holding one \special's expanded text. It contributes
// no dimensions to the box that carries it.
type specialNode struct {
	text    string
	srcLine int // 1-based source line (0 = unknown)
}

func (specialNode) isNode() {}

// doSpecial implements \special{<balanced text>}: the text is expanded at once
// (as TeX does when it builds the whatsit) and placed in the current list.
func (e *Engine) doSpecial() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || t.cs_ || t.cat != catBegin {
		if ok {
			e.back(t)
		}
		return
	}
	text := e.toksToString(e.expandList(e.grabGroup()))
	e.place(specialNode{text: text, srcLine: e.curSrcLine})
}

// originMark is a directive, not output: a special whose payload contains it
// declares its own reference point to be the origin of the picture that follows,
// which later literals recover through {?ox}/{?oy} (and {?-ox}/{?-oy} for the
// negated form). A driver needs that because the scopes it opens also transform
// the *text* the typesetter paints inside the picture: to place a box where TeX
// put it, the driver wraps it in the inverse of the picture's own map, and the
// inverse needs the origin the map was built from.
const originMark = "<gotex:origin/>"

// originElem is how a declared origin is carried through the emitted stream until
// resolveSpecialOrigins consumes it: the painter knows each special's own point
// but not the one a later special will refer back to, so the declaration travels
// with its coordinates and the back-references are resolved in one pass over the
// finished stream.
const originElem = "gotex:origin"

// specialLiteral returns the drawable payload of a special: the text after the
// "gotex:" namespace with its own-position placeholders resolved against (x, y),
// the special's reference point in page coordinates (points). ok is false for a
// special in another driver's namespace, which this engine draws not at all.
// Origin placeholders are left for resolveSpecialOrigins.
func specialLiteral(text string, x, y float64) (string, bool) {
	s := strings.TrimSpace(text)
	if !strings.HasPrefix(s, gotexSpecial) {
		return "", false
	}
	s = strings.TrimPrefix(s, gotexSpecial)
	s = strings.ReplaceAll(s, originMark,
		`<`+originElem+` x="`+f(x)+`" y="`+f(y)+`"/>`)
	r := strings.NewReplacer("{?x}", f(x), "{?y}", f(y), "{?nl}", "\n")
	return r.Replace(s), true
}

// originResolver resolves the origin back-references in a stream of literals as
// they arrive, keeping the last origin declared. A driver that interprets the
// stream in one go uses resolveSpecialOrigins; one that walks the page and needs
// each literal finished as it reaches it (the PDF driver, which has to know the
// transformation in force when it draws a character) resolves them one at a time.
type originResolver struct{ ox, oy float64 }

// next finishes one literal: the declarations in it are consumed (and become the
// origin from here on) and the back-references before each are resolved against
// the origin that was in force.
func (r *originResolver) next(s string) string {
	if !strings.Contains(s, "{?ox}") && !strings.Contains(s, "{?oy}") &&
		!strings.Contains(s, "{?-ox}") && !strings.Contains(s, "{?-oy}") &&
		!strings.Contains(s, "<"+originElem) {
		return s
	}
	var out strings.Builder
	rest := s
	for {
		i := strings.Index(rest, "<"+originElem)
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], "/>")
		if j < 0 {
			break
		}
		decl := rest[i : i+j+2]
		out.WriteString(substOrigin(rest[:i], r.ox, r.oy))
		r.ox, r.oy = originCoords(decl)
		rest = rest[i+j+2:]
	}
	out.WriteString(substOrigin(rest, r.ox, r.oy))
	return out.String()
}

// resolveSpecialOrigins finishes a page's emitted stream: each declaration is
// removed and the {?ox}/{?oy} (and negated {?-ox}/{?-oy}) placeholders after it
// are replaced by the coordinates it carried. A back-reference with no
// declaration before it resolves to the page origin.
func resolveSpecialOrigins(s string) string {
	var r originResolver
	return r.next(s)
}

// substOrigin replaces the origin back-references in one segment of the stream.
func substOrigin(s string, ox, oy float64) string {
	return strings.NewReplacer(
		"{?-ox}", f(-ox), "{?-oy}", f(-oy),
		"{?ox}", f(ox), "{?oy}", f(oy),
	).Replace(s)
}

// originCoords reads the x/y of an origin declaration element.
func originCoords(decl string) (float64, float64) {
	return attrNum(decl, "x"), attrNum(decl, "y")
}

// attrNum reads a numeric attribute from an element's source text.
func attrNum(elem, name string) float64 {
	key := " " + name + `="`
	i := strings.Index(elem, key)
	if i < 0 {
		return 0
	}
	rest := elem[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return 0
	}
	return parseFloat(rest[:j])
}
