// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements a pragmatic subset of the geometry package: the options
// of \usepackage[...]{geometry} and the \geometry{...} command that set the page
// paper size and margins, which in turn drive \hsize (text width), \vsize (text
// height) and the render margin used by the PDF/SVG drivers.
//
// Box-model mapping. The geometry package models a full four-sided margin box on
// a paper of a given size. This engine's renderers derive the physical page from
// the content box plus a margin: a page's paper width is spToPt(content.width) +
// 2*margin (see pdfdriver.go / pagebuilder.go). We map geometry's box model onto
// that as follows:
//
//   - \hsize = paperwidth  − left − right                        (or textwidth,  if given)
//   - \vsize = paperheight − top  − bottom − head − headsep − foot (or textheight, if given)
//   - the horizontal render margin is the LEFT margin;
//   - the vertical render margin is the TOP margin plus the running-head band
//     (top + head + headsep) — the whole distance from the paper edge to the first
//     line.
//
// The two render margins are separate because a class may spend its vertical
// budget quite differently from its horizontal one: beamer's slide is 1cm at the
// sides but 0.5cm top and bottom, all of it in the head and foot bands. When the
// margins are uniform the two agree and the rendered paper equals the requested
// paper exactly, in both directions.
//
// LIMITATION: with asymmetric margins (left≠right, or top≠bottom) the renderer
// still centres the text block on the leading margin, so the trailing paper edge
// is not modelled independently — \hsize and \vsize are right, the page is
// symmetric.
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

// paperSize is a named paper size, held as the dimensions the paper is DEFINED by
// so they are converted once, by the engine's own scanner, exactly as TeX converts
// them. Holding scaled points computed in floating point instead put a second
// conversion in this file, a scaled point or two away from the first.
type paperSize struct{ w, h string }

// paperSizes maps geometry's paper-size keywords to their dimensions. Portrait
// orientation (width < height); the landscape flag swaps them.
var paperSizes = map[string]paperSize{
	"a4paper":        {"210mm", "297mm"},
	"a5paper":        {"148mm", "210mm"},
	"b5paper":        {"176mm", "250mm"},
	"letterpaper":    {"8.5in", "11in"},
	"legalpaper":     {"8.5in", "14in"},
	"executivepaper": {"7.25in", "10.5in"},
}

// paper returns a named paper size in scaled points.
func (e *Engine) paper(name string) (w, h int, ok bool) {
	p, ok := paperSizes[name]
	if !ok {
		return 0, 0, false
	}
	w, _ = e.geomEval(p.w)
	h, _ = e.geomEval(p.h)
	return w, h, true
}

// geomState is the accumulated geometry layout. It persists on the Engine so a
// later \geometry{...} builds on the earlier \usepackage[...]{geometry}.
type geomState struct {
	paperW, paperH           int  // paper dimensions (sp)
	left, right, top, bottom int  // margins (sp)
	textW, textH             int  // explicit text dimensions (sp), valid iff the has* flag is set
	hasTextW, hasTextH       bool // textwidth / textheight were given (override margin arithmetic)
	// head, headsep and foot are the running-head and running-foot bands geometry
	// reserves between the top/bottom margins and the text block:
	//
	//	paperheight = top + head + headsep + textheight + foot + bottom
	//
	// They default to zero — a document that never names them keeps the plain
	// top/bottom arithmetic — and matter for a class that spends its vertical
	// budget there instead of on margins. beamer is the case in point: vmargin=0
	// with head=0.5cm and foot=0.5cm, so ignoring the bands overstated its text
	// height by the whole 1cm.
	head, headsep, foot int
}

// newGeomState returns the geometry defaults: letterpaper (the geometry package's
// default when the class does not pick one) with 1in margins on every side.
func (e *Engine) newGeomState() *geomState {
	w, h, _ := e.paper("letterpaper")
	m, _ := e.geomEval("1in")
	return &geomState{paperW: w, paperH: h, left: m, right: m, top: m, bottom: m}
}

// geomLooksDimen reports whether a geometry option value could be a dimension at
// all: a signed decimal, or something naming a control sequence. It is the guard
// that lets a malformed value be ignored instead of stored as a bogus zero.
func geomLooksDimen(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "\\") {
		return true
	}
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	if i == 0 {
		return false
	}
	_, err := strconv.ParseFloat(s[:i], 64)
	return err == nil
}

// splitOptsTopLevel splits a comma-separated option list on the commas OUTSIDE any
// brace group, so a value that is itself a list survives as one item. geometry's
// papersize={<width>,<height>} is exactly that shape, and a naive split tore it
// into two malformed halves — which is how beamer's own paper size was lost.
func splitOptsTopLevel(s string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// unbrace strips one enclosing {...} from a value.
func unbrace(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// geomToks retokenises a geometry option value under the engine's CURRENT catcode
// table. The document's own catcodes are what matters here: a class sets the option
// while @ is a letter, so \beamer@paperwidth has to tokenise as the one control
// sequence it is rather than \beamer followed by four other characters.
func (e *Engine) geomToks(s string) []tok {
	rs := []rune(s)
	ts := make([]tok, 0, len(rs))
	for i := 0; i < len(rs); i++ {
		if rs[i] != '\\' {
			ts = append(ts, chTok(rs[i], e.catcode[rs[i]]))
			continue
		}
		j := i + 1
		if j >= len(rs) {
			break // a trailing backslash names nothing
		}
		if e.catcode[rs[j]] != catLetter {
			ts = append(ts, csTok(string(rs[j])))
			i = j
			continue
		}
		k := j
		for k < len(rs) && e.catcode[rs[k]] == catLetter {
			k++
		}
		ts = append(ts, csTok(string(rs[j:k])))
		i = k - 1
	}
	return ts
}

// geomEval reads a geometry option value as a dimension, through the engine's OWN
// dimension scanner — the one that converts units by TeX's exact integer
// arithmetic. That is what a literal ("1cm", "30pt") deserves, and it is the only
// thing that can read a value naming a length: beamer passes its paper size as
// \beamer@paperwidth, not as a number.
//
// Parsing the literals separately, in floating point, put a second and slightly
// different conversion in the same file: 12.8cm came out 5sp short of what TeX
// makes of it, so a beamer page missed its reference width by a rounding step for
// no reason at all.
//
// The scan runs on an ISOLATED input stack: whatever the value leaves unread is
// dropped with it, and nothing can escape into the document being processed.
func (e *Engine) geomEval(s string) (int, bool) {
	s = unbrace(s)
	if !geomLooksDimen(s) {
		return 0, false
	}
	saved := e.lists
	e.lists = nil
	e.push(append(e.geomToks(s), csTok("relax")))
	d := e.scanDimen()
	e.lists = saved
	return d, true
}

// applyGeometry parses a geometry option string (the [...] of
// \usepackage[...]{geometry} or the {...} of \geometry{...}) and applies it to
// the engine's layout: it updates the accumulated geomState, then recomputes
// \hsize and \vsize. The render margin is read from the state by renderMargin.
func (e *Engine) applyGeometry(opts string) {
	if e.geom == nil {
		e.geom = e.newGeomState()
	}
	g := e.geom
	landscape := false

	for _, raw := range splitOptsTopLevel(opts) {
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
				if w, h, ok := e.paper(key); ok {
					g.paperW, g.paperH = w, h
				}
				// Any other bare flag is silently ignored.
			}
			continue
		}

		if key == "papersize" {
			// papersize={<width>,<height>}, or papersize=<size> for a square page.
			parts := splitOptsTopLevel(unbrace(val))
			w, wok := e.geomEval(parts[0])
			h, hok := w, wok
			if len(parts) > 1 {
				h, hok = e.geomEval(parts[1])
			}
			if wok && hok && w > 0 && h > 0 {
				g.paperW, g.paperH = w, h
			}
			continue
		}

		d, ok := e.geomEval(val)
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
		case "head", "headheight":
			g.head = d
		case "headsep":
			g.headsep = d
		case "foot", "footskip":
			g.foot = d
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
		e.vsize = g.paperH - g.top - g.bottom - g.head - g.headsep - g.foot
	}
	e.publishGeometry(g)
}

// applyAmsartGeometry gives the emulated amsart class its real text-block size.
//
// amsart (amslatex) sizes the text block itself, unlike article which the engine
// serves from its embedded, real .cls (that .cls computes \textwidth=345pt and
// \textheight=550pt via a size1x.clo, and the engine honours it). amsart is served
// from the built-in emulation instead — its real class runs, but is gated because
// its \newtheorem machinery could once run away — so the assignments amsart.cls makes
//
//	\textwidth=30pc            % 360pt
//	\textheight=50.5pc         % 606pt (54.5pc, 654pt, on a4paper)
//	\headheight=8pt \headsep=14pt
//	\def\calclayout{\advance\textheight -\headheight \advance\textheight -\headsep …}
//
// never run. Without them \hsize/\vsize keep the plain-TeX defaults set in New
// (6.5in × 8.9in = 469.8pt × 643.2pt): a text block far larger than amsart's real
// 360pt × 584pt. That over-large budget packs ~1.4× more material onto every page,
// which roughly halved amsart's page count versus the reference. Reproduce amsart's
// own geometry here so the emulated page budget matches the real class.
//
// The size options (10/11/12pt) do NOT change amsart's text block — only its leading
// — so this is size-independent; the paper option does (letterpaper vs a4paper).
func (e *Engine) applyAmsartGeometry(opts []string) {
	const pica = 12.0 // 1pc = 12pt
	textHeightPc := 50.5
	for _, o := range opts {
		if strings.TrimSpace(o) == "a4paper" {
			textHeightPc = 54.5
		}
	}
	// \calclayout removes \headheight (8pt) and \headsep (14pt) from \textheight;
	// the remainder is the text area, i.e. the page builder's \vsize.
	const headAllowance = 8.0 + 14.0 // \headheight + \headsep, in pt
	e.hsize = ptToSP(30 * pica)      // \textwidth = 30pc = 360pt
	e.vsize = ptToSP(textHeightPc*pica - headAllowance)
}

// publish writes the layout back into the LaTeX length registers a class reads.
//
// \hsize and \vsize are engine parameters (\textwidth and \textheight are \let to
// them), so those follow from the assignments above. \paperwidth and \paperheight
// are ORDINARY registers, allocated by the class kernel and never written until
// now — and a class that sizes itself from the paper rather than from \textwidth
// read zero. beamer is that class: it derives every frame dimension from
// \paperwidth, so an unpublished paper size collapsed the whole slide, which is
// how a 4:3 talk came out 78pt wide.
//
// The assignment is global: geometry configures the document, not a group.
func (e *Engine) publishGeometry(g *geomState) {
	e.setNamedDimen("paperwidth", g.paperW)
	e.setNamedDimen("paperheight", g.paperH)
	e.setNamedDimen("Gm@lmargin", g.left)
	e.setNamedDimen("Gm@rmargin", g.right)
	e.setNamedDimen("Gm@tmargin", g.top)
	e.setNamedDimen("Gm@bmargin", g.bottom)
}

// setNamedDimen assigns a value to an allocated \newdimen / \newlength by name,
// doing nothing when the name is not one. It is how the Go side writes a length
// the TeX side declared.
func (e *Engine) setNamedDimen(name string, v int) {
	m := e.eq[name]
	if m == nil {
		return
	}
	switch m.kind {
	case mDimenRef:
		e.setDimen(m.code, v, true)
	case mSkipRef:
		sp := e.skip[m.code]
		sp.width = v
		e.setSkip(m.code, sp, true)
	}
}

// renderMargin returns the page margin (in points) the drivers should use: the
// geometry left margin when geometry is active, otherwise the caller's fallback
// (the compile Option's margin). See the box-model note at the top of this file.
func (e *Engine) renderMargin(fallback float64) float64 {
	if e.geom != nil {
		return spToPt(e.geom.left)
	}
	if d, ok := e.classLeftMargin(); ok {
		return spToPt(d)
	}
	return fallback
}

// classLeftMargin and classTopMargin give the text block's position on the paper as
// the CLASS states it, for a document that never loaded geometry.
//
// classes.dtx, "Page Layout": "All margin dimensions are measured from a point one
// inch from the top and lefthand side of the page." So
//
//	left = 1in + \hoffset + \oddsidemargin
//	top  = 1in + \voffset + \topmargin + \headheight + \headsep
//
// and the same file computes \oddsidemargin as .5(\paperwidth - \textwidth) - 1in,
// which is the check: for article at 10pt on letterpaper that is 62pt, so the text
// starts 134.3pt from the left edge of a 614.3pt page.
//
// This only matters now that a page IS the paper. While the page was derived from
// the content, the block filled it by construction and its position could not be
// wrong; on a real sheet, it can.
func (e *Engine) classLeftMargin() (int, bool) {
	if _, _, ok := e.paperSizePt(); !ok {
		return 0, false
	}
	osm, ok := e.namedDimen("oddsidemargin")
	if !ok {
		return 0, false
	}
	off, _ := e.namedDimen("hoffset")
	return parseDimenStr("1in") + off + osm, true
}

func (e *Engine) classTopMargin() (int, bool) {
	if _, _, ok := e.paperSizePt(); !ok {
		return 0, false
	}
	tm, ok := e.namedDimen("topmargin")
	if !ok {
		return 0, false
	}
	off, _ := e.namedDimen("voffset")
	hh, _ := e.namedDimen("headheight")
	hs, _ := e.namedDimen("headsep")
	return parseDimenStr("1in") + off + tm + hh + hs, true
}

// paperSizePt returns the page the drivers should draw, in points, when the document
// has STATED one — that is, when geometry ran and knows the paper.
//
// TeX's page is the PAPER; the text block sits inside it at the margins, and material
// that overruns simply overfulls. Deriving the page from the content instead — content
// plus a uniform margin — is right only when the two coincide, and for a class that
// spends its vertical budget on running heads rather than on margins they do not.
// beamer is that class: its text block is \paperheight minus a 4pt footline allowance,
// so content-plus-margin made every slide 297.6pt tall where the paper is 273.147.
//
// The paper is read from \paperwidth / \paperheight, which is where LaTeX keeps it:
// classes.dtx has article, report and book run \ExecuteOptions{letterpaper,10pt,…} so
// the registers hold 8.5in x 11in unless an option says otherwise, [a4paper] sets
// 210mm x 297mm, and geometry publishes its own paper into the same two registers
// (see publishGeometry). One source of truth, whether or not geometry ran.
//
// The OFFSETS were already right (renderMargin, renderVMargin); only the extent was
// being computed from the wrong thing.
func (e *Engine) paperSizePt() (w, h float64, ok bool) {
	pw, pwOK := e.namedDimen("paperwidth")
	ph, phOK := e.namedDimen("paperheight")
	if !pwOK || !phOK || pw <= 0 || ph <= 0 {
		return 0, 0, false
	}
	return spToPt(pw), spToPt(ph), true
}

// namedDimen reads an allocated \newdimen / \newlength by name. It is the read
// counterpart of setNamedDimen.
func (e *Engine) namedDimen(name string) (int, bool) {
	m := e.eq[name]
	if m == nil {
		return 0, false
	}
	switch m.kind {
	case mDimenRef:
		return e.dimen[m.code], true
	case mSkipRef:
		return e.skip[m.code].width, true
	}
	return 0, false
}

// renderVMargin returns the margin the drivers should leave above and below the
// text block. It is the top margin PLUS the running-head band, because the head
// sits between the paper edge and the text: what the renderer needs is the whole
// distance from the paper edge down to the first line.
//
// A document with equal margins all round gets the same number as renderMargin, so
// nothing changes for it. A class that spends its vertical budget differently from
// its horizontal one does not: beamer's page is 1cm at the sides and 0.5cm top and
// bottom, and taking the side margin vertically as well made every slide 28pt too
// tall.
func (e *Engine) renderVMargin(fallback float64) float64 {
	if e.geom != nil {
		return spToPt(e.geom.top + e.geom.head + e.geom.headsep)
	}
	if d, ok := e.classTopMargin(); ok {
		return spToPt(d)
	}
	return fallback
}

// doGeometry handles \geometry{options}, re-applying geometry settings on top of
// any earlier ones (later wins).
func (e *Engine) doGeometry() {
	e.applyGeometry(e.readBraceGroupString())
}
