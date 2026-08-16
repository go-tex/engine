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

// specialLiteral returns the drawable payload of a special: the text after the
// "gotex:" namespace with its position placeholders resolved against (x, y), the
// special's reference point in page coordinates (points). ok is false for a
// special in another driver's namespace, which this engine draws not at all.
func specialLiteral(text string, x, y float64) (string, bool) {
	s := strings.TrimSpace(text)
	if !strings.HasPrefix(s, gotexSpecial) {
		return "", false
	}
	s = strings.TrimPrefix(s, gotexSpecial)
	r := strings.NewReplacer("{?x}", f(x), "{?y}", f(y), "{?nl}", "\n")
	return r.Replace(s), true
}
