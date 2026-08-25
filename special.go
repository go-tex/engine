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

// A picture can sit inside another, and a box placed inside a picture is wrapped
// in that picture's inverse. Both nest, so the resolver keeps a stack of what is
// in force rather than a single current value, and the driver declares all four
// events: a picture opening and closing, and an inverse opening and closing.
const (
	endOriginMark = "<gotex:endorigin/>"
	unmapMark     = "<gotex:unmap/>"
	endUnmapMark  = "<gotex:endunmap/>"

	endOriginElem = "gotex:endorigin"
	unmapElem     = "gotex:unmap"
	endUnmapElem  = "gotex:endunmap"
)

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
	for mark, elem := range map[string]string{
		endOriginMark: endOriginElem, unmapMark: unmapElem, endUnmapMark: endUnmapElem,
	} {
		s = strings.ReplaceAll(s, mark, `<`+elem+`/>`)
	}
	r := strings.NewReplacer("{?x}", f(x), "{?y}", f(y), "{?nl}", "\n")
	return r.Replace(s), true
}

// originResolver resolves the origin back-references in a stream of literals as
// they arrive, keeping the last origin declared. A driver that interprets the
// stream in one go uses resolveSpecialOrigins; one that walks the page and needs
// each literal finished as it reaches it (the PDF driver, which has to know the
// transformation in force when it draws a character) resolves them one at a time.
// scopeKind says what a level of the resolver's stack is: a picture applies the
// page map, an inverse takes it back off again.
type scopeKind uint8

const (
	pictureScope scopeKind = iota
	unmapScope
)

type scopeLevel struct {
	kind scopeKind
	x, y float64 // a picture's origin
}

type originResolver struct {
	ox, oy float64
	// stack is every picture and every inverse still open, outermost first. The
	// two cancel, so what is in force is the pictures with no inverse after them.
	stack []scopeLevel
}

// inForce returns the picture maps still applying, outermost first: the ones a
// box or a nested picture would be transformed by. An inverse cancels the
// picture before it, which is exactly what happens when a picture is placed
// inside a node's box.
func (r *originResolver) inForce() []scopeLevel {
	var out []scopeLevel
	for _, s := range r.stack {
		switch s.kind {
		case pictureScope:
			out = append(out, s)
		case unmapScope:
			if n := len(out); n > 0 {
				out = out[:n-1]
			}
		}
	}
	return out
}

// unmapChain is the inverse of the page maps in force, innermost first. One
// picture's map is translate(ox,oy)scale(0.996264,-0.996264), so its inverse is
// scale(1.00375,-1.00375)translate(-ox,-oy).
//
// It is empty whenever nothing is in force, which is the ordinary case and also
// the case of a picture placed inside a node's box — there the box's own inverse
// already brought us back to the page. It is not empty when a picture opens
// directly inside another, as pgfplots' axis does, and that is what stops the
// page map being applied twice.
func (r *originResolver) unmapChain() string {
	force := r.inForce()
	var b strings.Builder
	for i := len(force) - 1; i >= 0; i-- {
		b.WriteString("scale(1.00375,-1.00375)translate(")
		b.WriteString(f(-force[i].x))
		b.WriteString(",")
		b.WriteString(f(-force[i].y))
		b.WriteString(")")
	}
	return b.String()
}

// next finishes one literal: the declarations in it are consumed (and become the
// origin from here on) and the back-references before each are resolved against
// the origin that was in force.
func (r *originResolver) next(s string) string {
	if !strings.Contains(s, "{?") && !strings.Contains(s, "<gotex:") {
		return s
	}
	var out strings.Builder
	rest := s
	for {
		i, kind, closing := nextScopeDecl(rest)
		if i < 0 {
			break
		}
		j := strings.Index(rest[i:], "/>")
		if j < 0 {
			break
		}
		decl := rest[i : i+j+2]
		// The segment before the declaration resolves against the state as it
		// stands, which is what lets a picture's own transform name the maps
		// enclosing it rather than itself.
		out.WriteString(r.subst(rest[:i]))
		if closing {
			r.pop()
		} else if kind == pictureScope {
			x, y := originCoords(decl)
			r.ox, r.oy = x, y
			r.stack = append(r.stack, scopeLevel{kind: pictureScope, x: x, y: y})
		} else {
			r.stack = append(r.stack, scopeLevel{kind: unmapScope})
		}
		rest = rest[i+j+2:]
	}
	out.WriteString(r.subst(rest))
	return out.String()
}

// pop closes the innermost scope and restores the origin a back-reference sees.
func (r *originResolver) pop() {
	if n := len(r.stack); n > 0 {
		r.stack = r.stack[:n-1]
	}
	r.ox, r.oy = 0, 0
	for i := len(r.stack) - 1; i >= 0; i-- {
		if r.stack[i].kind == pictureScope {
			r.ox, r.oy = r.stack[i].x, r.stack[i].y
			break
		}
	}
}

// nextScopeDecl finds the next declaration and says what it is.
func nextScopeDecl(s string) (at int, kind scopeKind, closing bool) {
	type cand struct {
		i       int
		kind    scopeKind
		closing bool
	}
	best := cand{i: -1}
	for _, c := range []cand{
		{strings.Index(s, "<"+originElem+" "), pictureScope, false},
		{strings.Index(s, "<"+endOriginElem), pictureScope, true},
		{strings.Index(s, "<"+unmapElem+"/"), unmapScope, false},
		{strings.Index(s, "<"+endUnmapElem), unmapScope, true},
	} {
		if c.i >= 0 && (best.i < 0 || c.i < best.i) {
			best = c
		}
	}
	return best.i, best.kind, best.closing
}

// subst resolves one segment's back-references against the state in force.
func (r *originResolver) subst(s string) string {
	if strings.Contains(s, "{?unmap}") {
		s = strings.ReplaceAll(s, "{?unmap}", r.unmapChain())
	}
	return substOrigin(s, r.ox, r.oy)
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
