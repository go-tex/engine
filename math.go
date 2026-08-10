// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"strings"

	texmath "github.com/go-tex/math"
)

// This file gives the engine math mode. A $…$ (or $$…$$) group is delegated
// whole to the go-tex/math typesetter — the engine does not reimplement the math
// sub-language; it collects the raw math source, hands it to that library, and
// embeds the resulting self-contained SVG as a math box on the current list. The
// library reports only overall width/height (no baseline), so the box is centred
// on the text baseline — an honest approximation until go-tex/math exposes the
// math axis.

// mathRendererT is the engine field's type (a go-tex/math renderer).
type mathRendererT = *texmath.Renderer

// mathNode is a rendered piece of math: its self-contained SVG plus the box
// dimensions (sp) used to place and line-break it.
type mathNode struct {
	svg                  string
	width, height, depth int
}

func (mathNode) isNode() {}

// mathRenderer lazily builds the go-tex/math renderer with its embedded MATH font.
func (e *Engine) mathRenderer() *texmath.Renderer {
	if e.mathR == nil {
		r, err := texmath.New(texmath.DefaultFont())
		if err != nil {
			e.fail("cannot init math renderer: " + err.Error())
			return nil
		}
		e.mathR = r
	}
	return e.mathR
}

// mathSize is the point size at which math is rendered: the current text font's
// size, or 10pt when no font is set.
func (e *Engine) mathSize() int {
	if e.curFont != nil {
		if s := e.curFont.sizePt(); s > 0 {
			return s
		}
	}
	return 10
}

// scanMathSource reads raw (unexpanded) tokens up to the closing math shift,
// reconstructing the math source string for go-tex/math. It returns the source
// and whether the math was display style ($$…$$).
func (e *Engine) scanMathSource() (string, bool) {
	display := false
	if t, ok := e.getNext(); ok {
		if t.cat == catMath && !t.cs_ { // opening $$ ⇒ display math
			display = true
		} else {
			e.back(t)
		}
	}
	var b strings.Builder
	for {
		t, ok := e.getNext()
		if !ok {
			break
		}
		if t.cat == catMath && !t.cs_ {
			if display { // consume the second $ of the closing $$
				if u, ok := e.getNext(); ok && !(u.cat == catMath && !u.cs_) {
					e.back(u)
				}
			}
			break
		}
		if t.cs_ {
			b.WriteByte('\\')
			b.WriteString(t.cs)
			b.WriteByte(' ')
		} else {
			b.WriteRune(t.ch)
		}
	}
	return b.String(), display
}

// makeMath renders a math source string to a math box.
func (e *Engine) makeMath(src string, display bool) mathNode {
	r := e.mathRenderer()
	if r == nil {
		return mathNode{}
	}
	size := e.mathSize()
	var svg string
	var err error
	if display {
		svg, err = r.RenderDisplaySVG(src, size)
	} else {
		svg, err = r.RenderSVG(src, size)
	}
	if err != nil {
		e.fail("math error in $" + src + "$: " + err.Error())
		return mathNode{}
	}
	w, h := parseSVGSize(svg)
	// No baseline is reported; centre the box on the text baseline.
	half := ptToSP(h) / 2
	return mathNode{svg: svg, width: ptToSP(w), height: half, depth: ptToSP(h) - half}
}

// doMath handles a math-shift token: collect the source, render it, and place the
// math box (inline in horizontal mode, or on the vertical list in vertical mode).
func (e *Engine) doMath() {
	src, display := e.scanMathSource()
	m := e.makeMath(src, display)
	if !e.inPar && !display {
		e.beginParagraph(false) // inline math begins a paragraph in vertical mode
	}
	if e.inPar {
		e.parList = append(e.parList, m)
	} else {
		e.contribute(m) // display math on its own goes to the vertical list
	}
}

// parseSVGSize extracts the width and height (points) from an SVG root element's
// width="…" and height="…" attributes.
func parseSVGSize(svg string) (float64, float64) {
	return attrFloat(svg, `width="`), attrFloat(svg, `height="`)
}

func attrFloat(svg, key string) float64 {
	i := strings.Index(svg, key)
	if i < 0 {
		return 0
	}
	rest := svg[i+len(key):]
	j := strings.IndexByte(rest, '"')
	if j < 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(strings.TrimSuffix(rest[:j], "pt"), 64)
	return v
}
