// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements a pragmatic subset of the geometry package: the options
// of \usepackage[...]{geometry} and the \geometry{...} command that set the page
// paper size and margins, which in turn drive \hsize (text width), \vsize (text
// height) and the render margin used by the PDF/SVG drivers.
//
// Box-model mapping. The geometry package models a full four-sided margin box on
// a paper of a given size. This engine's renderers, however, take a single
// uniform margin and derive the physical page from the content box: a page's
// paper width is spToPt(content.width) + 2*margin (see pdfdriver.go /
// pagebuilder.go). We map geometry's box model onto that single-margin renderer
// as follows:
//
//   - \hsize = paperwidth  − left − right   (or textwidth,  if given)
//   - \vsize = paperheight − top  − bottom  (or textheight, if given)
//   - the render margin is the LEFT margin, applied uniformly on all sides.
//
// When the horizontal margins are equal (the common margin=<d> / hmargin=<d>
// case) the rendered paper width equals the requested paperwidth exactly, and
// likewise vertically. LIMITATION: with asymmetric margins (e.g. left≠right) the
// single-margin renderer cannot reproduce the off-centre text block — \hsize is
// still the correct text width, but the physical page is centred on the left
// margin, so the right/bottom paper edges are not modelled independently.
//
// Timing. geometry is meant to configure the WHOLE document, so the case that
// matters is applying it in the preamble (before \begin{document}); \hsize and
// \vsize are then in force for every paragraph. \geometry{...} re-applies and the
// later call wins, accumulating onto the previous state. A mid-document change
// takes effect only for material typeset afterwards; it does not retroactively
// reflow already-typeset pages.

import (
	"strconv"
	"strings"
)

// paperSize is a named paper dimension pair, in scaled points.
type paperSize struct{ w, h int }

// inToSP, mmToSP and cmToSP convert physical lengths to scaled points using the
// same TeX point (1in = 72.27pt) as parseDimenStr, so paper-vs-margin arithmetic
// is exact.
func inToSP(in float64) int { return ptToSP(in * 72.27) }
func mmToSP(mm float64) int { return ptToSP(mm * 72.27 / 25.4) }

// paperSizes maps geometry's paper-size keywords to their dimensions. Portrait
// orientation (width < height); the landscape flag swaps them.
var paperSizes = map[string]paperSize{
	"a4paper":        {mmToSP(210), mmToSP(297)},
	"a5paper":        {mmToSP(148), mmToSP(210)},
	"b5paper":        {mmToSP(176), mmToSP(250)},
	"letterpaper":    {inToSP(8.5), inToSP(11)},
	"legalpaper":     {inToSP(8.5), inToSP(14)},
	"executivepaper": {inToSP(7.25), inToSP(10.5)},
}

// geomState is the accumulated geometry layout. It persists on the Engine so a
// later \geometry{...} builds on the earlier \usepackage[...]{geometry}.
type geomState struct {
	paperW, paperH           int  // paper dimensions (sp)
	left, right, top, bottom int  // margins (sp)
	textW, textH             int  // explicit text dimensions (sp), valid iff the has* flag is set
	hasTextW, hasTextH       bool // textwidth / textheight were given (override margin arithmetic)
}

// newGeomState returns the geometry defaults: letterpaper (the geometry package's
// default when the class does not pick one) with 1in margins on every side.
func newGeomState() *geomState {
	p := paperSizes["letterpaper"]
	m := inToSP(1)
	return &geomState{paperW: p.w, paperH: p.h, left: m, right: m, top: m, bottom: m}
}

// geomDimen parses a geometry dimension value ("1in", "2.5cm", "30pt"). It
// reports ok=false for an empty or malformed value so callers can ignore the key
// rather than storing a bogus (zero) length. A missing unit means points, as in
// parseDimenStr.
func geomDimen(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == 0 {
		return 0, false
	}
	if _, err := strconv.ParseFloat(s[:i], 64); err != nil {
		return 0, false
	}
	return parseDimenStr(s), true
}

// applyGeometry parses a geometry option string (the [...] of
// \usepackage[...]{geometry} or the {...} of \geometry{...}) and applies it to
// the engine's layout: it updates the accumulated geomState, then recomputes
// \hsize and \vsize. The render margin is read from the state by renderMargin.
func (e *Engine) applyGeometry(opts string) {
	if e.geom == nil {
		e.geom = newGeomState()
	}
	g := e.geom
	landscape := false

	for _, raw := range strings.Split(opts, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		key, val, hasEq := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		if !hasEq {
			// Bare flag: a paper-size keyword or an orientation.
			switch key {
			case "landscape":
				landscape = true
			case "portrait":
				landscape = false
			default:
				if p, ok := paperSizes[key]; ok {
					g.paperW, g.paperH = p.w, p.h
				}
				// Any other bare flag is silently ignored.
			}
			continue
		}

		d, ok := geomDimen(val)
		if !ok {
			continue // malformed dimension: ignore the key, never panic.
		}
		switch key {
		case "margin":
			g.left, g.right, g.top, g.bottom = d, d, d, d
			g.hasTextW, g.hasTextH = false, false
		case "hmargin":
			g.left, g.right = d, d
			g.hasTextW = false
		case "vmargin":
			g.top, g.bottom = d, d
			g.hasTextH = false
		case "left", "lmargin":
			g.left = d
			g.hasTextW = false
		case "right", "rmargin":
			g.right = d
			g.hasTextW = false
		case "top", "tmargin":
			g.top = d
			g.hasTextH = false
		case "bottom", "bmargin":
			g.bottom = d
			g.hasTextH = false
		case "textwidth":
			g.textW, g.hasTextW = d, true
		case "textheight":
			g.textH, g.hasTextH = d, true
		case "paperwidth":
			g.paperW = d
		case "paperheight":
			g.paperH = d
		default:
			// Unknown key: ignored.
		}
	}

	if landscape {
		g.paperW, g.paperH = g.paperH, g.paperW
	}

	if g.hasTextW {
		e.hsize = g.textW
	} else {
		e.hsize = g.paperW - g.left - g.right
	}
	if g.hasTextH {
		e.vsize = g.textH
	} else {
		e.vsize = g.paperH - g.top - g.bottom
	}
}

// renderMargin returns the page margin (in points) the drivers should use: the
// geometry left margin when geometry is active, otherwise the caller's fallback
// (the compile Option's margin). See the box-model note at the top of this file.
func (e *Engine) renderMargin(fallback float64) float64 {
	if e.geom != nil {
		return spToPt(e.geom.left)
	}
	return fallback
}

// doGeometry handles \geometry{options}, re-applying geometry settings on top of
// any earlier ones (later wins).
func (e *Engine) doGeometry() {
	e.applyGeometry(e.readBraceGroupString())
}
