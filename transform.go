// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"math"
	"strings"
)

// This file implements the graphicx box transformations \scalebox, \reflectbox,
// \resizebox and \rotatebox. Each wraps its {content}, packed as an hbox, in a
// transformNode carrying an affine (linear, translation-free) map expressed in
// math coordinates (x to the right, y up from the baseline): a content point
// (x, y) maps to (a*x + c*y, b*x + d*y). \scalebox/\reflectbox/\resizebox are
// diagonal scales; \rotatebox is a rotation. These are a natural fit for the
// engine's SVG/PDF output, which both expose native affine transforms — no glyph
// rework is needed, only a change of coordinate system around the inner box.
//
// The wrapper reserves the axis-aligned bounding box of the *transformed* inner
// box so packing and line breaking leave the right room. Because the map is
// linear, the content origin (the inner box's reference point at the left of the
// baseline) always maps to the origin; the transformed box therefore keeps its
// baseline through that point, and only a horizontal offset refDX — the distance
// from the transformed box's left edge to that origin — is needed to position the
// content. A negative horizontal scale (\reflectbox) mirrors the content and puts
// the origin at the right edge (refDX == width); a rotation offsets it by the
// amount the rotated box overhangs to the left of the reference point.

// transformNode wraps an inner box in a math-space linear map (x'=a*x+c*y,
// y'=b*x+d*y) and the axis-aligned bounding box that map produces.
type transformNode struct {
	inner            *boxNode
	a, b, c, d       float64 // math-space linear map coefficients
	boxW, boxH, boxD int     // reserved bounding box (sp): width, height, depth
	refDX            int     // sp offset of the content origin from the left edge
	angle            float64 // rotation in degrees CCW (0 for pure scales); kept for reference
}

func (transformNode) isNode() {}

// width, height and depth are the transformed box's reserved dimensions (sp).
func (tn transformNode) width() int  { return tn.boxW }
func (tn transformNode) height() int { return tn.boxH }
func (tn transformNode) depth() int  { return tn.boxD }

// newTransform wraps inner in the math-space linear map [a b c d] and precomputes
// the axis-aligned bounding box the transformed box occupies. angle (degrees CCW)
// is recorded for reference only; the geometry is driven entirely by the matrix.
func newTransform(inner *boxNode, a, b, c, d, angle float64) transformNode {
	w := float64(inner.width)
	h := float64(inner.height)
	dp := float64(inner.depth)
	// The inner box spans x in [0, w] and y in [-dp, h] (math, y-up); its four
	// corners bound the transformed box. The reference point (0,0) maps to (0,0)
	// and lies on the left edge (0 in [-dp, h]), so it is inside the transformed
	// hull: minX<=0<=maxX and minY<=0<=maxY, giving non-negative dimensions.
	var minX, maxX, minY, maxY float64
	first := true
	for _, x := range [2]float64{0, w} {
		for _, y := range [2]float64{h, -dp} {
			px := a*x + c*y
			py := b*x + d*y
			if first {
				minX, maxX, minY, maxY, first = px, px, py, py, false
				continue
			}
			minX = math.Min(minX, px)
			maxX = math.Max(maxX, px)
			minY = math.Min(minY, py)
			maxY = math.Max(maxY, py)
		}
	}
	return transformNode{
		inner: inner, a: a, b: b, c: c, d: d, angle: angle,
		boxW:  int(math.Round(maxX - minX)),
		boxH:  int(math.Round(maxY)),
		boxD:  int(math.Round(-minY)),
		refDX: int(math.Round(-minX)),
	}
}

// doScalebox implements \scalebox{h-scale}[v-scale]{content}: it scales the
// content by h-scale horizontally and v-scale vertically (v defaults to h). A
// negative factor mirrors along that axis.
func (e *Engine) doScalebox() transformNode {
	sx := e.scanFactorArg()
	sy, ok := e.scanOptFactorArg()
	if !ok {
		sy = sx
	}
	return newTransform(e.grabInnerHbox(), sx, 0, 0, sy, 0)
}

// doReflectbox implements \reflectbox{content} = \scalebox{-1}[1]{content}: a
// horizontal mirror that preserves the box's magnitude.
func (e *Engine) doReflectbox() transformNode {
	return newTransform(e.grabInnerHbox(), -1, 0, 0, 1, 0)
}

// doResizebox implements \resizebox{width}{height}{content}: it scales the content
// so its natural box becomes the requested width and height. A '!' for either
// dimension keeps the aspect ratio derived from the other (both '!' leaves the
// content unscaled). The height refers to the box height above the baseline.
func (e *Engine) doResizebox() transformNode {
	wLen, wBang := e.scanDimenOrBang()
	hLen, hBang := e.scanDimenOrBang()
	inner := e.grabInnerHbox()
	sx, sy := resizeFactors(inner.width, inner.height, wLen, wBang, hLen, hBang)
	return newTransform(inner, sx, 0, 0, sy, 0)
}

// resizeFactors turns \resizebox's target dimensions into horizontal/vertical
// scale factors relative to a natural size (natW × natH). A '!' target adopts the
// other axis's factor (aspect-preserving); a non-positive natural size or a '!'
// on both axes yields the identity factor 1 for that axis.
func resizeFactors(natW, natH, wLen int, wBang bool, hLen int, hBang bool) (sx, sy float64) {
	sx, sy = 1, 1
	if !wBang && natW > 0 {
		sx = float64(wLen) / float64(natW)
	}
	if !hBang && natH > 0 {
		sy = float64(hLen) / float64(natH)
	}
	switch {
	case wBang && !hBang:
		sx = sy
	case hBang && !wBang:
		sy = sx
	}
	return sx, sy
}

// doRotatebox implements \rotatebox[options]{angle}{content}: it rotates the
// content counter-clockwise by angle degrees about the reference point (the left
// end of the baseline). Any leading optional [options] argument is accepted and
// ignored (the rotation origin is fixed at the reference point for this milestone).
func (e *Engine) doRotatebox() transformNode {
	e.skipOptBracketGroup()
	angle := e.scanFactorArg()
	inner := e.grabInnerHbox()
	rad := angle * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	// CCW rotation in math (y-up) space: x'=cos*x - sin*y, y'=sin*x + cos*y.
	return newTransform(inner, cos, sin, -sin, cos, angle)
}

// grabInnerHbox reads a {content} group and packs it as a natural-width hbox, the
// inner box every transform wraps.
func (e *Engine) grabInnerHbox() *boxNode {
	list, _ := e.grabHboxList()
	return hpackSP(list, packNatural, 0)
}

// scanFactorArg reads a mandatory {factor} group holding a signed decimal number
// (e.g. {2}, {1.5}, {-1}) and returns it as a float64. A missing group backs the
// token out and returns 0.
func (e *Engine) scanFactorArg() float64 {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return 0
	}
	v := e.scanFactor()
	if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
		e.back(c)
	}
	return v
}

// scanOptFactorArg reads an optional [factor] argument, returning its value and
// whether one was present (an absent bracket backs out and returns false).
func (e *Engine) scanOptFactorArg() (float64, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, false
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return 0, false
	}
	v := e.scanFactor()
	if c, ok := e.getXToken(); ok && !(!c.cs_ && c.ch == ']') {
		e.back(c)
	}
	return v, true
}

// scanFactor reads a signed decimal number (no unit) as a float64, reusing TeX's
// sign scanning and 16-bit fraction decimal parser.
func (e *Engine) scanFactor() float64 {
	sign := e.scanSign()
	intPart, frac := e.scanDecimalSP()
	return float64(sign) * (float64(intPart) + float64(frac)/float64(unity))
}

// scanDimenOrBang reads a {..} group that is either a dimension (e.g. {40pt},
// {\width}) or the single character {!} meaning "keep the aspect ratio". It
// returns the dimension in sp and whether the group was '!'. A missing group
// backs out and returns (0, false).
func (e *Engine) scanDimenOrBang() (int, bool) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return 0, false
	}
	u, ok := e.getXToken()
	if ok && !u.cs_ && u.ch == '!' {
		if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
			e.back(c)
		}
		return 0, true
	}
	if ok {
		e.back(u)
	}
	d := e.scanDimen()
	if c, ok := e.getXToken(); ok && !(c.cat == catEnd && !c.cs_) {
		e.back(c)
	}
	return d, false
}

// skipOptBracketGroup consumes and discards an optional [..] argument if one is
// next (used to tolerate \rotatebox's key-value option list), leaving the input
// untouched when no bracket follows.
func (e *Engine) skipOptBracketGroup() {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return
	}
	for {
		u, ok := e.getXToken()
		if !ok || (!u.cs_ && u.ch == ']') {
			return
		}
	}
}

// paintTransformSP paints a transformed box for the SVG driver. The inner box is
// drawn in a local coordinate system whose origin — the content reference point —
// sits at (x+refDX, baseline), through an SVG matrix realising the math-space
// linear map. SVG y grows downward, so a math map (x'=a*x+c*y, y'=b*x+d*y over a
// y-up frame) becomes matrix(a, -b, -c, d, ox, baseline): a local SVG point
// (lx, ly) = (x_m, -y_m) maps to (a*x_m - c*ly, -b*x_m + d*ly) = (x'_m, -y'_m),
// i.e. the transformed point re-expressed y-down, then shifted to the origin.
func paintTransformSP(sb *strings.Builder, tn transformNode, x, baseline float64, font fontFace, tc *textCursor) {
	ox := x + spToPt(tn.refDX)
	fmt.Fprintf(sb, `<g transform="matrix(%s,%s,%s,%s,%s,%s)">`,
		f(zeroSafe(tn.a)), f(zeroSafe(-tn.b)), f(zeroSafe(-tn.c)), f(zeroSafe(tn.d)), f(ox), f(baseline))
	paintBoxSP(sb, tn.inner, 0, 0, font, tc)
	sb.WriteString(`</g>`)
}

// zeroSafe maps IEEE-754 negative zero (produced by negating +0) to positive
// zero so the SVG matrix never prints an "-0" component.
func zeroSafe(v float64) float64 {
	if v == 0 {
		return 0
	}
	return v
}
