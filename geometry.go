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
	// hasLeft..hasBottom record that the option list NAMED a margin. geometry.sty
	// decides each direction from which of its three parts (margin, body, margin)
	// are fixed (\Gm@detall, geometry.dtx:2237-2271); when none is, the body is
	// 0.7 of the paper and the margins split the rest by marginratio. The state
	// holds 1in in left..bottom from the start, so "the value is 1in" cannot say
	// whether anybody asked for it.
	hasLeft, hasRight, hasTop, hasBottom bool
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
	// inclHead/inclFoot say whether those bands are part of the BODY. geometry's
	// vertical equation is paperheight = top + height + bottom, and by default
	// height is \textheight alone: the running head sits INSIDE the top margin, so
	// naming headheight/headsep does not move the first line. Only includehead /
	// includefoot / includeheadfoot fold a band into height, which is when it costs
	// the body its space.
	//
	// Adding the bands unconditionally cost five lines on every page of a document
	// whose style sets them — automl.sty's \newgeometry{textheight=9in, top=1in,
	// headheight=12\p@, headsep=20\p@, footskip=0.5in} lost 32pt at the top and
	// 36pt at the bottom, 44 lines a page where the reference sets 49.
	inclHead, inclFoot bool
	// hasHead/hasHeadsep/hasFoot record that the OPTION LIST named the band, which
	// "the value is zero" cannot: headsep=0pt is a legitimate instruction to fold in
	// no separation at all, and must not be refilled from the class.
	hasHead, hasHeadsep, hasFoot bool
	// heightRounded is geometry's `heightrounded`: bring the text height to a whole
	// number of \baselineskip above \topskip (geometry.sty:513, 794-809).
	heightRounded bool
	// beamerBands keeps the old unconditional folding for beamer alone. beamer asks
	// for head=0.5cm/foot=0.5cm WITHOUT includeheadfoot, so real geometry hands it
	// \textheight = \paperheight and beamer's own frame machinery carves the two
	// bands out again. This engine does not run that machinery, so folding them here
	// is what keeps a slide the right height.
	beamerBands bool
}

// geomDefaultBody is geometry's default body, 0.7 of the paper in that direction
// (\Gm@Dhscale/\Gm@Dvscale, geometry.dtx:1928-1929). The product is TeX's own for
// `0.7\paperwidth`: the factor read as a 16-bit fraction (round(0.7*65536) =
// 45875) and applied by xn_over_d, so it lands on the sp tectonic does.
func geomDefaultBody(paper int) int {
	q, _ := xnOverD(paper, 45875, 65536)
	return q
}

// geomSplitMargins divides what the body leaves of the paper between two margins
// by the ratio a:b as \Gm@detiiandiii does (geometry.dtx:2221-2226): the first is
// (total / (a+b)) * a with the division done FIRST, on integers, and the second
// is the remainder. Multiplying first lands up to a few sp away.
func geomSplitMargins(total, a, b int) (first, second int) {
	first = total / (a + b) * a
	return first, total - first
}

// newGeomState returns the geometry defaults: the class's paper size (a4paper/…), or
// letterpaper when the class named none, with 1in margins on every side. geometry
// inherits the class paper size, so \documentclass[a4paper]+\usepackage[margin]{geometry}
// lays out on A4, not US letter.
func (e *Engine) newGeomState() *geomState {
	size := "letterpaper"
	if e.classPaperSize != "" {
		size = e.classPaperSize
	}
	w, h, _ := e.paper(size)
	m, _ := e.geomEval("1in")
	return &geomState{paperW: w, paperH: h, left: m, right: m, top: m, bottom: m,
		beamerBands: e.beamerBands}
}

// parseGeomFloat reads a bare decimal factor (the geometry `scale` value, e.g. 0.775).
func parseGeomFloat(s string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
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
	// twocol is the LAST twocolumn/onecolumn key of the list (1 on, -1 off, 0 none),
	// applied once at the end: pushed token lists would run last-first.
	twocol := 0
	defer func() {
		if twocol != 0 {
			e.geomSetTwocolumn(twocol > 0)
		}
	}()

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
			case "heightrounded":
				g.heightRounded = true
			case "twocolumn", "onecolumn":
				// \define@key{Gm}{twocolumn}[true] sets the class's \@twocolumn
				// switch and nothing else (geometry.sty:576-579): the columns
				// themselves come from the \twocolumn the class runs afterwards
				// (sn-jnl.cls:803-807 branches on \if@twocolumn at \maketitle).
				twocol = onOff(key == "twocolumn")
			case "includehead", "includefoot", "includeheadfoot":
				// By default (includehead=false) the header/footer sit in the margins and
				// the text height is paperH−top−bottom (the engine's head/headsep/foot are
				// zero, so its vsize formula already gives that). These flags fold the head
				// and/or foot INTO the body, so the text height loses headheight+headsep
				// (12pt+25pt) and/or footskip (30pt) — the standard 10–12pt class values.
				//
				// The band a flag folds in is the CLASS's, read from \headheight,
				// \headsep and \footskip — \Gm@@process subtracts those very
				// registers (geometry.sty:784-791), and a head=/headsep=/foot= key
				// works by assigning them first. Hardcoding 12/25/30 instead billed
				// acmart's sigconf block, which names head=13pt and nothing else,
				// 13+0+30 = 43pt where its class asks for 13+14+12 = 39
				// (go-tex/engine#307).
				if key == "includehead" || key == "includeheadfoot" {
					g.inclHead = true
				}
				if key == "includefoot" || key == "includeheadfoot" {
					g.inclFoot = true
				}
			default:
				if w, h, ok := e.paper(key); ok {
					g.paperW, g.paperH = w, h
				}
				// Any other bare flag is silently ignored.
			}
			continue
		}

		if key == "text" || key == "body" || key == "total" {
			// text={<w>,<h>} (body is its alias) sets the text block directly; total is
			// the same block including head/foot, which default to zero here — so treat
			// all three as textwidth/textheight={w,h}.
			parts := splitOptsTopLevel(unbrace(val))
			w, wok := e.geomEval(parts[0])
			if wok && w > 0 {
				g.textW, g.hasTextW = w, true
			}
			if len(parts) > 1 {
				if h, hok := e.geomEval(parts[1]); hok && h > 0 {
					g.textH, g.hasTextH = h, true
				}
			}
			continue
		}

		if key == "scale" {
			// scale=<f> or scale={<fw>,<fh>}: the text body is that fraction of the paper
			// (the margins are auto-centred). scale=0.7 is a common one-liner layout.
			parts := splitOptsTopLevel(unbrace(val))
			fw, wok := parseGeomFloat(parts[0])
			fh, hok := fw, wok
			if len(parts) > 1 {
				fh, hok = parseGeomFloat(parts[1])
			}
			if wok && fw > 0 {
				g.textW, g.hasTextW = int(float64(g.paperW)*fw), true
			}
			if hok && fh > 0 {
				g.textH, g.hasTextH = int(float64(g.paperH)*fh), true
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

		if key == "twocolumn" || key == "onecolumn" {
			// twocolumn=false is \@twocolumnfalse; onecolumn=true is the same.
			if v := strings.ToLower(val); v == "true" || v == "false" {
				twocol = onOff((v == "true") == (key == "twocolumn"))
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
			g.hasLeft, g.hasRight, g.hasTop, g.hasBottom = true, true, true, true
			g.hasTextW, g.hasTextH = false, false
		case "hmargin":
			g.left, g.right = d, d
			g.hasLeft, g.hasRight = true, true
			g.hasTextW = false
		case "vmargin":
			g.top, g.bottom = d, d
			g.hasTop, g.hasBottom = true, true
			g.hasTextH = false
		// inner/outer are the binding-side and outside margins of a two-sided
		// layout, and geometry defines them as plain ALIASES of the left and right
		// margins — "\let\KV@Gm@inner\KV@Gm@lmargin", "\let\KV@Gm@outer\KV@Gm@rmargin"
		// (geometry.sty:521-525), with innermargin/outermargin the same again. They
		// carried no meaning here at all, so acmart's per-format geometry
		// (acmart.cls:614-620 for sigconf: inner=54pt outer=54pt) left \textwidth at
		// article's letterpaper default of 469.755pt instead of 506.3pt, and a
		// column held 49 lines where the reference holds 57 (go-tex/engine#307).
		case "left", "lmargin", "inner", "innermargin":
			g.left, g.hasLeft = d, true
			if g.hasRight {
				g.hasTextW = false // over-specified: the margins win
			}
		case "right", "rmargin", "outer", "outermargin":
			g.right, g.hasRight = d, true
			if g.hasLeft {
				g.hasTextW = false
			}
		// ONE margin beside a width/height does not cancel it: geometry fixes the
		// body and derives the other margin (\Gm@detall, geometry.dtx:2237-2271).
		// Only a margin pair plus the body is over-specified, and then the body
		// is the one dropped.
		case "top", "tmargin":
			g.top, g.hasTop = d, true
			if g.hasBottom {
				g.hasTextH = false
			}
		case "bottom", "bmargin":
			g.bottom, g.hasBottom = d, true
			if g.hasTop {
				g.hasTextH = false
			}
		case "textwidth", "width":
			g.textW, g.hasTextW = d, true
		case "textheight", "height":
			g.textH, g.hasTextH = d, true
		case "paperwidth":
			g.paperW = d
		case "paperheight":
			g.paperH = d
		case "columnsep":
			// \define@key{Gm}{columnsep}{\Gm@setlength\columnsep{#1}} (geometry.sty:569).
			// acmart asks for columnsep=2pc for every sigconf-family format
			// (acmart.cls:614-620); ignoring it left the engine's 10pt default and made
			// each column 3.5pt too wide.
			e.columnsep = d
			e.setNamedDimen("columnsep", d)
		case "marginparwidth", "marginpar":
			e.setNamedDimen("marginparwidth", d)
		case "head", "headheight":
			// The key works by ASSIGNING \headheight (\Gm@setlength), which the
			// vertical arithmetic then reads back (geometry.sty:784-791).
			g.head, g.hasHead = d, true
			e.setNamedDimen("headheight", d)
		case "headsep":
			g.headsep, g.hasHeadsep = d, true
			e.setNamedDimen("headsep", d)
		case "foot", "footskip":
			g.foot, g.hasFoot = d, true
			e.setNamedDimen("footskip", d)
		default:
			// Unknown key: ignored.
		}
	}

	if landscape {
		g.paperW, g.paperH = g.paperH, g.paperW
	}

	if !g.hasTextW && !g.hasLeft && !g.hasRight {
		// Nothing of the horizontal partition is fixed (\Gm@detall case 000):
		// width = \Gm@Dhscale\paperwidth, the margins split the rest by the
		// default hmarginratio, 1:1 one-sided and 2:3 inner:outer two-sided
		// (geometry.dtx:1918-1920, 2267).
		w := geomDefaultBody(g.paperW)
		a, b := 1, 1
		if e.twoside() {
			a, b = 2, 3
		}
		g.left, g.right = geomSplitMargins(g.paperW-w, a, b)
		e.hsize = w
	} else if g.hasTextW {
		e.hsize = g.textW
		// The body is fixed: the margin geometry did not get is what the paper
		// leaves, shared 1:1 (2:3 two-sided) when neither was named. Left as it
		// stood it kept the default split of an earlier layout, so a
		// \newgeometry{textwidth=...} sat off-centre (\Gm@detall, dtx:2237-2271).
		switch {
		case !g.hasLeft && !g.hasRight:
			a, b := 1, 1
			if e.twoside() {
				a, b = 2, 3
			}
			g.left, g.right = geomSplitMargins(g.paperW-g.textW, a, b)
		case g.hasLeft && !g.hasRight:
			g.right = g.paperW - g.textW - g.left
		case !g.hasLeft && g.hasRight:
			g.left = g.paperW - g.textW - g.right
		}
	} else {
		e.hsize = g.paperW - g.left - g.right
	}
	// What geometry computes is the ONE-COLUMN text width. In a two-column document
	// the column measure has to be taken again from it — see retakeColumnMeasure.
	e.retakeColumnMeasure()
	// A band the option list did not name is the CLASS's, read from \headheight,
	// \headsep and \footskip: \Gm@@process subtracts those very registers
	// (geometry.sty:784-791), and a head=/headsep=/foot= key works by assigning them
	// first. Filled here, once the whole list is read, because includeheadfoot may
	// come before or after the keys. Hardcoded 12/25/30 billed acmart's sigconf
	// block — which names head=13pt and nothing else — 13+0+30 = 43pt where its
	// class asks for 13+14+12 = 39 (go-tex/engine#307).
	if g.inclHead {
		if !g.hasHead && g.head == 0 {
			g.head = e.classDimen("headheight", ptToSP(12))
		}
		if !g.hasHeadsep && g.headsep == 0 {
			g.headsep = e.classDimen("headsep", ptToSP(25))
		}
	}
	if g.inclFoot && !g.hasFoot && g.foot == 0 {
		g.foot = e.classDimen("footskip", ptToSP(30))
	}
	if !g.hasTextH && !g.hasTop && !g.hasBottom {
		// The same default in the vertical direction (geometry.dtx:2269): the
		// body, head and foot included when includehead/includefoot say so, is
		// \Gm@Dvscale\paperheight, and the margins split the rest 2:3 top:bottom.
		h := geomDefaultBody(g.paperH)
		g.top, g.bottom = geomSplitMargins(g.paperH-h, 2, 3)
		e.vsize = h - g.bodyBands()
	} else if g.hasTextH {
		e.vsize = g.textH
		// As horizontally: the body is fixed, so the missing margin is derived
		// (2:3 top:bottom when neither was named).
		switch {
		case !g.hasTop && !g.hasBottom:
			g.top, g.bottom = geomSplitMargins(g.paperH-g.textH, 2, 3)
		case g.hasTop && !g.hasBottom:
			g.bottom = g.paperH - g.textH - g.top
		case !g.hasTop && g.hasBottom:
			g.top = g.paperH - g.textH - g.bottom
		}
	} else {
		e.vsize = g.paperH - g.top - g.bottom - g.bodyBands()
	}
	if g.heightRounded {
		e.vsize = e.roundHeight(e.vsize)
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

// classGeometry is the single-column-equivalent text block and base leading a
// class format lays its body out with. inkedW is the width of the actual inked
// text — for a two-column format the two columns' widths summed, WITHOUT the
// gutter, since that is the width the same amount of body text would occupy in
// one column and so drives the per-page character budget. textH is the text
// height and leading the baseline-to-baseline body skip. All three are in points.
type classGeometry struct {
	inkedW, textH, leading float64
	// paperW/paperH are the class's PAPER, in TeX points, when it is not US letter.
	// Zero leaves the engine's default. A class that sets its own sheet does it
	// through geometry — acmart.cls:670-672 gives acmsmall
	// \geometry{…, paperwidth=6.75in, paperheight=10in, …} — and this emulation
	// loads no class file, so nothing else publishes it.
	paperW, paperH float64
	// fontPermille is the class's BASE BODY SIZE as a permille of 10pt (900 = 9pt),
	// for a class that states one its size options cannot express. Zero leaves
	// whatever the size option chose.
	//
	// acmart needs it: its format decides the body size, not a 10pt/11pt/12pt option
	// (acmart.cls:237-256 is an \ifcase over \ACM@format@nr setting \ACM@fontsize,
	// then \LoadClass[\ACM@fontsize, reqno]{amsart}), and setPtsize reads only the
	// three named options — so every acmart paper was set at 10pt. Measured on corpus
	// paper 2304.01951 (sigconf, so 9pt), page 4 against its reference: our glyphs
	// were 11.35pt tall and 4.49pt wide against 10.22 and 3.90, at the SAME line pitch
	// and the SAME column width. 11% too large in the same measure is ~11% more lines.
	fontPermille int
}

// applyClassGeometry installs a single-column-equivalent text block and base
// leading as a persistent floor on the engine, the way applyAmsartGeometry does
// for amsart. It is the shared tail of applyAcmartGeometry and
// applyIEEEtranGeometry: both classes are served by the article-shaped emulation
// (they are neither embedded nor, for the papers that need this, bundled), so
// nothing else sizes their page. Setting \hsize/\vsize gives the page builder the
// class's real text block, and setting \baselineskip (and the setspace 1.0
// reference \baseBaselineskip with it) gives it the class's real body leading —
// which the emulation would otherwise leave at the size-option default, packing
// the wrong number of lines onto every page. A later \usepackage{geometry} still
// wins, because \documentclass runs before it.
func (e *Engine) applyClassGeometry(g classGeometry) {
	e.hsize = ptToSP(g.inkedW)
	e.vsize = ptToSP(g.textH)
	// leading 0 means "the class states none": elsarticle's journal types keep
	// \baselinestretch at 1, so the size option's own leading is already right and
	// overwriting it with zero would collapse every page.
	if g.leading > 0 {
		e.baselineskip = ptToSP(g.leading)
		e.baseBaselineskip = e.baselineskip
	}
	if g.paperW > 0 && g.paperH > 0 {
		e.setNamedDimen("paperwidth", ptToSP(g.paperW))
		e.setNamedDimen("paperheight", ptToSP(g.paperH))
	}
	// After the leading, because setPtsize set both from the size option and this is
	// the class overriding it.
	if g.fontPermille > 0 {
		e.scaleClassFontsToBase(g.fontPermille)
	}
}

// acmartFormats maps each acmart format option to its single-column-equivalent
// geometry. The values are the real class's, read from acmart.cls's per-format
// \geometry{...} block (\RequirePackage{geometry}) and its base size:
//
//   - The paper is US letter (612×792pt) except acmsmall/acmcp, which acmart sets
//     to 6.75in×10in (486×720pt), and acmlarge, still letter.
//   - inkedW = \textwidth for a single-column format, or \textwidth−\columnsep
//     (the two columns' combined text width) for a two-column one.
//   - textH and leading are measured from the reference PDFs the class produces,
//     which fold in geometry's includeheadfoot/heightrounded rounding and
//     setspace's \onehalfspacing (manuscript) that a bare arithmetic would miss.
//
// acmart's body size is 9pt for manuscript/acmtog/sigconf/siggraph/sigchi/acmcp
// and 10pt for acmsmall/acmlarge/sigplan/acmengage (acmart.cls's \ACM@fontsize
// table); the leading below already reflects that. The default format, when the
// document names none, is manuscript — acmart.cls's own default.
var acmartFormats = map[string]classGeometry{
	// manuscript: single column, letterpaper, 9pt body under \onehalfspacing —
	// a wide-spaced review layout. acmart.cls hands geometry letterpaper, head=13pt,
	// marginparwidth=6pc, heightrounded and NOTHING ELSE, so the body is geometry's
	// default 0.7 of the paper: \textwidth 430.00462pt, and 0.7 of 11in = 556.48pt
	// rounded to 50 lines at the 11pt baselineskip in force + \topskip = 560pt.
	// tectonic 0.17.0 prints TW=430.00462pt TH=560.0pt for it. This row said
	// 465x585, measured off a PDF and never read from the class's own arithmetic.
	"manuscript": {inkedW: 430.00462, textH: 560, leading: 13.5, fontPermille: 900},
	// Single-column journal formats. acmsmall/acmcp: 6.75in×10in paper,
	// \textwidth=486−2·46=394pt. acmlarge: letter, \textwidth=612−2·81=450pt. 10pt.
	"acmsmall": {inkedW: 394, textH: 588, leading: 12, paperW: 6.75 * 72.27, paperH: 10 * 72.27, fontPermille: 1000},
	"acmcp":    {inkedW: 394, textH: 588, leading: 12, paperW: 6.75 * 72.27, paperH: 10 * 72.27},
	"acmlarge": {inkedW: 450, textH: 600, leading: 12, fontPermille: 1000},
	// Two-column formats: inkedW = \textwidth−\columnsep. sigconf/siggraph/sigchi/
	// acmtog set a 9pt body (11pt leading); sigplan/acmengage a 10pt body (12pt).
	"acmtog":    {inkedW: 484, textH: 645, leading: 11, fontPermille: 900}, // 508−24
	"sigconf":   {inkedW: 480, textH: 644, leading: 11, fontPermille: 900}, // 504−24
	"siggraph":  {inkedW: 480, textH: 644, leading: 11, fontPermille: 900},
	"sigchi":    {inkedW: 480, textH: 635, leading: 11, fontPermille: 900},
	"sigplan":   {inkedW: 480, textH: 648, leading: 12, fontPermille: 1000},
	"acmengage": {inkedW: 480, textH: 644, leading: 12},
	// sigchi-a is a landscape, wide-left-margin single-text-column oddity; the
	// two-column budget is the closest bounded approximation.
	"sigchi-a": {inkedW: 480, textH: 644, leading: 11, fontPermille: 1000},
}

// acmartTwoColumnFormat reports whether any acmart format option selects a
// two-column format (as opposed to the single-column manuscript/acmsmall/…). The
// format is given bare ([sigconf]) or as format=… ([format=sigconf]).
func acmartTwoColumnFormat(opts []string) bool {
	for _, o := range opts {
		o = strings.TrimPrefix(strings.TrimSpace(o), "format=")
		switch strings.TrimSpace(o) {
		case "sigconf", "siggraph", "sigchi", "sigplan", "acmtog", "acmengage", "sigchi-a":
			return true
		}
	}
	return false
}

// applyAcmartGeometry gives the emulated acmart class its real text block and
// base leading. acmart is neither embedded nor (for the papers that need this)
// bundled, so \documentclass{acmart} falls to the article-shaped emulation, which
// keeps the plain-TeX 6.5in×8.9in block and the 12pt size-default leading — the
// wrong geometry for every acmart format, and the measured driver of acmart's
// page-count divergence. The format is selected by a bare option keyword
// (manuscript, sigconf, …); acmart's own default when none is given is manuscript.
// Two-column formats are still rendered single-column (columns are separately
// scoped), but the single-column-equivalent block above makes the page count
// right regardless.
func (e *Engine) applyAcmartGeometry(opts []string) {
	g := acmartFormats["manuscript"] // acmart.cls's default format
	for _, o := range opts {
		// The format is given bare ([sigconf]) or as format=… ([format=sigconf]),
		// and only the bare spelling was matched here — so a paper writing
		// \documentclass[format=sigconf]{acmart} silently got the MANUSCRIPT
		// geometry, single-column and 13.5pt-leaded, where it asked for a
		// two-column 9pt journal page. acmartTwoColumnFormat already strips the
		// prefix; this did not, and the two disagreed about the same option.
		o = strings.TrimPrefix(strings.TrimSpace(o), "format=")
		if f, ok := acmartFormats[strings.TrimSpace(o)]; ok {
			g = f
		}
	}
	e.applyClassGeometry(g)
	e.applyAcmartFigureName(opts)
}

// applyAcmartFigureName heads an ACM JOURNAL paper's figures "Fig." instead of
// "Figure", which the real class does with
//
//	\if@ACM@journal \captionsetup[figure]{name={Fig.}} \fi   (acmart.cls:931-934)
//
// and only there: the sigconf family keeps "Figure". The journal formats are
// acmsmall, acmlarge, acmtog and acmcp (acmart.cls:219-247). A bundled acmart.cls
// does this through \captionsetup itself (see caption.go) and does not reach here;
// this covers the 7 corpus papers whose acmart is EMULATED, of which one —
// 2406.01525, \documentclass[acmsmall]{acmart} — has 15 "Fig." in its reference
// where we printed "Figure".
//
// ⛔ manuscript is NOT in the list although acmart counts it as a journal and
// makes it the DEFAULT. Two corpus papers write \documentclass[STYLE]{acmart} —
// an unsubstituted placeholder, so a formatted real acmart would fall back to
// manuscript — and both references print "Figure", not "Fig.". Whatever the real
// class does with an invalid format, it is not the journal branch, so the name is
// changed only when the document NAMES a journal format.
func (e *Engine) applyAcmartFigureName(opts []string) {
	for _, o := range opts {
		switch strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(o), "format=")) {
		case "acmsmall", "acmlarge", "acmtog", "acmcp":
			e.define("figurename", &meaning{kind: mMacro, body: stringToToks("Fig.")}, true)
			return
		}
	}
}

// applyAchemsoGeometry gives the emulated achemso class its real text block and
// base leading. achemso is the ACS journals' class; its MANUSCRIPT mode — what a
// paper asks for with [manuscript=article] — is the double-spaced review layout
// ACS requires for submission, and it is the mode arXiv preprints use.
//
// The values are measured from the reference PDF the real class produces: letter
// paper with 1in margins (\textwidth 468pt, \textheight 648pt) and a 12pt body
// set double-spaced, which puts consecutive baselines 23.9pt apart. Without this
// the emulation kept the size-default 12pt leading and packed twice the text onto
// every page: corpus paper 2209.13121 came out in 23 pages against the real
// class's 42, with 98% of its words present — compressed, not lost.
//
// The journal layout (no manuscript option) is single-spaced and two-column; it is
// left to the article emulation, whose leading is already the right order.
func (e *Engine) applyAchemsoGeometry(opts []string) {
	if !achemsoManuscript(opts) {
		return
	}
	e.applyClassGeometry(classGeometry{inkedW: 468, textH: 648, leading: 23.9})
}

// achemsoManuscript reports whether the class options ask for the double-spaced
// manuscript layout. The option carries a value (manuscript=article,
// manuscript=note, …) naming the article type, and any of them is a manuscript.
func achemsoManuscript(opts []string) bool {
	for _, o := range opts {
		k, _, _ := strings.Cut(strings.TrimSpace(o), "=")
		if strings.TrimSpace(k) == "manuscript" {
			return true
		}
	}
	return false
}

// elsarticleFormats maps elsarticle's journal types to the geometry the class
// installs for them, read from elsarticle.cls's \ifnum\jtype blocks (1222-1270).
// Each of the three requires geometry and states its own sheet:
//
//	\RequirePackage{geometry}
//	\geometry{twoside, paperwidth=210mm, paperheight=297mm,
//	          textheight=622pt, textwidth=468pt, …}          % jtype=3
//
// so the paper is A4 and geometry writes it to the media box. WITHOUT one of
// 1p/3p/5p the class requires geometry at all, nothing writes a media box, and
// tectonic leaves the sheet at US letter even though \paperwidth is A4 —
// `elsarticle[final,12pt]` comes out 612x792 and `elsarticle[preprint,11pt,3p]`
// 595x842, which is the pair this table now tells apart.
var elsarticleFormats = map[string]classGeometry{
	"1p": {inkedW: 384, textH: 562, paperW: a4W, paperH: a4H},
	"3p": {inkedW: 468, textH: 622, paperW: a4W, paperH: a4H},
	"5p": {inkedW: 522, textH: 682, paperW: a4W, paperH: a4H},
}

// a4W/a4H are A4 in TeX points: 210mm x 297mm at 72.27pt to the inch.
const (
	a4W = 210 / 25.4 * 72.27
	a4H = 297 / 25.4 * 72.27
)

// applyElsarticleGeometry installs the text block for an elsarticle JOURNAL type
// (1p/3p/5p) and reports whether it did. The leading is left alone: those types
// keep \baselinestretch at 1, so the size option's own leading is already right.
//
// The default type is preprint, which sets no geometry at all — the caller loads
// the embedded article.cls for it instead, which is what elsarticle.cls itself
// does (\LoadClass{article}).
func (e *Engine) applyElsarticleGeometry(opts []string) bool {
	for _, o := range opts {
		if g, ok := elsarticleFormats[strings.TrimSpace(o)]; ok {
			e.applyClassGeometry(g)
			return true
		}
	}
	return false
}

// applyRevtexGeometry gives the emulated revtex classes their real text block.
// revtex is not embedded, so an unbundled paper falls to the article-shaped
// emulation, whose block is narrower: measured on corpus paper 2407.11498 (aps,
// prx, reprint) our columns were 226pt wide against the reference's 245.
//
// The values are the class's own, from aps10pt4-2.rtx and aps11pt4-2.rtx:
//
//	reprint   \textwidth 42.5pc = 510pt, \columnsep 1.5pc = 18pt,
//	          \textheight 56pc = 672pt  → columns 246pt each,
//	          \normalsize 10pt on 11.5 (aps10pt4-2.rtx \@setfontsize\normalsize\@xpt{11.5})
//	preprint  \textwidth 468pt, \textheight 665.5pt, single column,
//	          \normalsize 12pt on 14 (aps12pt4-2.rtx)
//
// inkedW follows the acmartFormats convention: for a two-column format it is
// \textwidth minus \columnsep, the two columns' combined inked width.
func (e *Engine) applyRevtexGeometry(reprint bool, opts []string) {
	g := classGeometry{inkedW: 468, textH: 665.5, leading: 14}
	if reprint {
		g = classGeometry{inkedW: 510 - 18, textH: 672, leading: 11.5}
	}
	g.paperW, g.paperH = revtexPaper(opts)
	e.applyClassGeometry(g)
}

// revtexPaper reads the sheet from revtex's paper option. The class declares the
// five standard ones and executes NONE of them (revtex4-2.cls:5928-5951), so the
// sheet is letter unless the document asks, which is why only the papers that
// write a4paper come out wrong:
//
//	\DeclareOption{a4paper}{\setlength\paperheight{297mm}\setlength\paperwidth{210mm}}
//
// Measured on corpus paper 2308.02700, \documentclass[a4paper,twocolumn]{revtex4-2}:
// the reference sheet is 595.28x841.89 and ours was 612x792. Both engines set it
// on 12 pages, so this is the OUTPUT being wrong, not the pagination (#419).
//
// Zero leaves the engine's letter default, so letterpaper needs no entry.
func revtexPaper(opts []string) (w, h float64) {
	const inch = 72.27
	for _, o := range opts {
		switch strings.TrimSpace(o) {
		case "a4paper":
			return a4W, a4H
		case "a5paper":
			return 148 / 25.4 * inch, 210 / 25.4 * inch
		case "b5paper":
			return 176 / 25.4 * inch, 250 / 25.4 * inch
		case "legalpaper":
			return 8.5 * inch, 14 * inch
		case "executivepaper":
			return 7.25 * inch, 10.5 * inch
		}
	}
	return 0, 0
}

// applyIEEEtranGeometry gives the emulated IEEEtran class its real text block and
// base leading. Like acmart, IEEEtran is served by the article emulation when the
// paper does not bundle IEEEtran.cls, so its compact two-column block is otherwise
// lost to the 6.5in×8.9in default and it over-paginates.
//
// The default mode is journal: \textwidth=43pc with \columnsep=1pc, so the two
// columns' combined inked width is 42pc=504pt; \textheight is 58 lines × 12pt =
// 696pt (IEEEtran.cls quantises the height to a whole number of lines per column);
// \normalsize at 10pt is 10pt/12pt. The conference mode uses a 9.25in (668pt) text
// height; technote is a 9pt-bodied journal, so its leading is tighter. The paper
// size (a4paper/letterpaper) does not change \textwidth, which IEEEtran fixes at
// 43pc, so it is not read here.
func (e *Engine) applyIEEEtranGeometry(opts []string) {
	g := classGeometry{inkedW: 504, textH: 696, leading: 12} // journal, the default
	for _, o := range opts {
		switch strings.TrimSpace(o) {
		case "conference":
			g.textH = 668 // 9.25in conference text height
		case "technote":
			g.leading = 11 // 9pt body
		}
	}
	e.applyClassGeometry(g)
}

// classFileResolvable reports whether name's .cls can be found on the search path
// (embedded set or a file the paper bundles). It gates the emulation-geometry
// floors for acmart/IEEEtran: when the real class IS resolvable it is loaded and
// sizes its own page, so the floor must not pre-empt it.
func (e *Engine) classFileResolvable(name string) bool {
	_, _, ok := e.findTeXFile(name, []string{".cls"})
	return ok
}

// publishGeometry writes the layout back into the LaTeX length registers a class reads.
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
	// What a document reads back are the CLASS registers, which geometry sets
	// from the margins at the end of \Gm@process (geometry.dtx:3280-3292):
	// \oddsidemargin = left - 1in, \evensidemargin = right - 1in when two-sided
	// and the same as odd otherwise, \topmargin = top - 1in less the head band
	// when the head is not part of the body. They kept the class's values, so
	// \the\oddsidemargin read 62pt after geometry had put the text elsewhere.
	// 1in as TeX scans it: xn_over_d(65536, 7227, 100) = 4736286 sp. parseDimenStr
	// rounds the product to 4736287, one sp too many, which shows in the fifth
	// decimal of the register.
	inch, _ := xnOverD(65536, 7227, 100)
	e.setNamedDimen("oddsidemargin", g.left-inch)
	if e.twoside() {
		e.setNamedDimen("evensidemargin", g.right-inch)
	} else {
		e.setNamedDimen("evensidemargin", g.left-inch)
	}
	// The bands a key named are asserted again every time the layout is processed:
	// \Gm@process starts from \Gm@restore@org and re-applies the key list
	// (geometry.sty:750-752), so a later \geometry{left=…} puts head=/headsep=/foot=
	// back over whatever was assigned in between. beamer depends on it: it asks for
	// head=0.5cm and headsep=0pt (beamer.cls:355-362), then \input's LaTeX's
	// size11.clo (beamer.cls:375), which sets \headheight=12pt and \headsep=25pt, and
	// only its own \geometry{left=…} (beamerbaseframecomponents.sty:39) restores
	// 14.23pt and 0pt. Left at 12 and 25, every frame's body sat 14pt low
	// (go-tex/engine#924).
	if g.hasHead {
		e.setNamedDimen("headheight", g.head)
	}
	if g.hasHeadsep {
		e.setNamedDimen("headsep", g.headsep)
	}
	if g.hasFoot {
		e.setNamedDimen("footskip", g.foot)
	}
	tm := g.top - inch
	if !g.inclHead {
		tm -= e.classDimen("headheight", 0) + e.classDimen("headsep", 0)
	}
	e.setNamedDimen("topmargin", tm)
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
		sp := e.getSkip(m.code)
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
		if !e.hasClass {
			return 0, 0, false // a bare snippet: sized to its own content
		}
		// US letter, the size a TeX engine falls back to when nothing set one.
		// tectonic writes 612x792 for revtex4-1, and for [a4paper]{article} too,
		// since article.cls sets \paperwidth but never the media box.
		//
		// A class this engine EMULATES loads no class file, so nothing publishes
		// these registers, and the page used to collapse onto its own CONTENT: a
		// revtex two-column paper came out on pages 356pt tall instead of 792, and
		// a one-line scrartcl document on a page 151pt tall.
		return 8.5 * 72.27, 11 * 72.27, true
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
		return e.getDimen(m.code), true
	case mSkipRef:
		return e.getSkip(m.code).width, true
	}
	return 0, false
}

// bodyBands is how much of the page height the running-head and running-foot bands
// take FROM THE BODY: nothing unless the document asked for includehead/includefoot
// (or it is beamer, whose frame machinery this engine does not run). See geomState.
func (g *geomState) bodyBands() int {
	n := 0
	if g.inclHead || g.beamerBands {
		n += g.head + g.headsep
	}
	if g.inclFoot || g.beamerBands {
		n += g.foot
	}
	return n
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
		m := e.geom.top
		switch {
		case e.geom.beamerBands && e.realBeamer():
			// Real beamer resizes its head band at \begin{document}: \headheight
			// becomes the headline's height (beamerbaseframecomponents.sty:169-180),
			// not geometry's head=0.5cm, so the live registers are the band.
			m += e.bandDimen("headheight", 0) + e.bandDimen("headsep", 0)
		case e.geom.inclHead || e.geom.beamerBands:
			m += e.geom.head + e.geom.headsep
		}
		return spToPt(m - e.headBand())
	}
	if d, ok := e.classTopMargin(); ok {
		return spToPt(d - e.headBand())
	}
	return fallback - spToPt(e.headBand())
}

// headBand is the height a running head takes ABOVE the text block: \headheight
// plus \headsep, or zero when no head is drawn.
//
// Both margins above answer where the TEXT starts — classes.dtx, "Page Layout":
// 1in + \voffset + \topmargin + \headheight + \headsep. A page assembled with a
// head begins with the head, so positioned at that margin the head sat where the
// text belongs and everything went down with it: measured against the reference on
// 2405.18549 page 3, our first ink was at 72pt where the reference has 37, and the
// gap under the head 10pt against 31.
//
// The page is therefore lifted by the band it carries. The decision is per
// DOCUMENT rather than per page, which is what the renderer can see; a fancy page
// style draws its head on every page it governs, so the two coincide wherever this
// applies.
func (e *Engine) headBand() int {
	// The FIELDS are tested, never fancyHeader(): that one TYPESETS the line, and a
	// margin accessor must not. Calling it here moved a corpus paper from 55 pages
	// to 60 — a geometry question answered by running the typesetter.
	fancy := len(e.fancyHF[fldHL])+len(e.fancyHF[fldHC])+len(e.fancyHF[fldHR]) > 0
	if !fancy && !e.hasLatexHead() {
		return 0
	}
	if e.pageStyle == "empty" {
		return 0
	}
	return e.bandDimen("headheight", 12*unity) + e.bandDimen("headsep", 25*unity)
}

// bandDimen reads a running-head band register (\headheight, \headsep) for the
// page assembly: the value \end{document} recorded when geometry ran (see
// recordShipBands), ZERO INCLUDED, since geometry publishes those registers and a
// zero there is the layout. beamer runs with \headsep=0pt and sets \headheight to
// its headline's height, 0pt for the default theme
// (beamerbaseframecomponents.sty:169); classDimen reads a zero as "never set" and
// substituted article's 12pt and 25pt, which put a 37pt band above every frame and
// its body 14pt low (go-tex/engine#924). Without geometry, the class default still
// stands in for an unset register, as before.
func (e *Engine) bandDimen(name string, def int) int {
	if v, ok := e.shipBands[name]; ok {
		return v
	}
	return e.classDimen(name, def)
}

// onOff is 1 for true and -1 for false, so that 0 can mean "not stated".
func onOff(b bool) int {
	if b {
		return 1
	}
	return -1
}

// geomSetTwocolumn is geometry's twocolumn / onecolumn key: \@twocolumntrue or
// \@twocolumnfalse, run as TeX so the \newif the class reads is the one changed.
func (e *Engine) geomSetTwocolumn(on bool) {
	name := "@twocolumnfalse"
	if on {
		name = "@twocolumntrue"
	}
	e.push([]tok{csTok(name)})
}

// geomSnap is the page layout \restoregeometry returns to: geometry's
// \Gm@restore@pkg (geometry.sty:1106-1107), which \AtBeginDocument fills with the
// lengths as the preamble left them.
type geomSnap struct {
	g            geomState
	hsize, vsize int // the ONE-column measure and the text height
	columnsep    int
	bands        map[string]int // \headheight, \headsep, \footskip, \marginparwidth
}

// geomBandNames are the registers a head=/foot=/marginparwidth= key assigns.
var geomBandNames = []string{"headheight", "headsep", "footskip", "marginparwidth"}

// recordShipBands keeps \headheight and \headsep as they stand at \end{document},
// for the page assembly. TeX's \@outputpage reads them at each \shipout, inside
// the document environment's group; this engine assembles its pages after the run,
// when that group has closed and undone every local assignment the body made.
// beamer makes exactly such an assignment: \beamer@calculateheadfoot sets
// \headheight to its headline's height at \begin{document}
// (beamerbaseframecomponents.sty:169), 0pt for the default theme, and the
// assembly read geometry's global 14.23pt back, so every frame's body sat 14pt
// low (go-tex/engine#924). Only when geometry ran: it is what makes a zero in these
// registers a real layout and not an unset one (see bandDimen).
func (e *Engine) recordShipBands() {
	if e.geom == nil {
		return
	}
	e.shipBands = map[string]int{}
	for _, n := range []string{"headheight", "headsep"} {
		if v, ok := e.namedDimen(n); ok {
			e.shipBands[n] = v
		}
	}
}

// saveGeometryAtBody records the preamble layout when \begin{document} runs.
func (e *Engine) saveGeometryAtBody() {
	if e.geom == nil {
		return
	}
	s := &geomSnap{g: *e.geom, hsize: e.hsize, vsize: e.vsize, columnsep: e.columnsep,
		bands: map[string]int{}}
	if e.oneColHsize > 0 {
		s.hsize = e.oneColHsize
	}
	for _, n := range geomBandNames {
		if v, ok := e.namedDimen(n); ok {
			s.bands[n] = v
		}
	}
	e.geomPkg = s
}

// doNewGeometry is \newgeometry{opts}: \clearpage, then the options applied. The page
// is cleared FIRST so that the paragraph in progress is broken at the old measure
// (geometry.sty:1131); before \begin{document} there is no page to clear.
func (e *Engine) doNewGeometry() {
	// Before the body \clearpage is a no-op (measured: preamble \newgeometry starts no
	// page), so one path serves both.
	// The argument stays in the input; \gotex@newgeom reads it after \clearpage.
	e.push([]tok{csTok("clearpage"), csTok("gotex@newgeom")})
}

// doRestoreGeometry is \restoregeometry: \clearpage, then back to the layout the
// preamble left (geometry.sty:1141-1144).
func (e *Engine) doRestoreGeometry() {
	if e.inBody {
		e.push([]tok{csTok("clearpage"), csTok("gotex@restoregeom")})
		return
	}
	e.restoreGeometrySnap()
}

// restoreGeometrySnap puts the saved layout back, the way applyGeometry's own tail
// establishes one: measure, column measure, text height, published lengths.
//
// The page grid is still one layout for the whole run (the paginator and the
// renderer read the final \vsize and margins), so the title pages that precede a
// \restoregeometry take the restored text height and margins too; what is exact is
// the line measure of every paragraph set after it, and the page height of the pages
// that follow. This is the A/B that measured 203 pages against 206 on 2402.04711.
func (e *Engine) restoreGeometrySnap() {
	s := e.geomPkg
	if s == nil {
		return
	}
	g := s.g
	e.geom = &g
	e.hsize = s.hsize
	e.retakeColumnMeasure()
	e.vsize = s.vsize
	e.columnsep = s.columnsep
	e.setNamedDimen("columnsep", s.columnsep)
	for n, v := range s.bands {
		e.setNamedDimen(n, v)
	}
	e.publishGeometry(e.geom)
}

// doGeometry handles \geometry{options}, re-applying geometry settings on top of
// any earlier ones (later wins).
func (e *Engine) doGeometry() {
	e.applyGeometry(e.readBraceGroupString())
}

// classDimen reads a dimension the CLASS set, falling back to def when the class
// declared none. geometry's own vertical arithmetic works this way: \Gm@@process
// subtracts \headheight, \headsep and \footskip as they stand (geometry.sty:784-791).
func (e *Engine) classDimen(name string, def int) int {
	if v, ok := e.namedDimen(name); ok && v > 0 {
		return v
	}
	return def
}

// roundHeight implements geometry's `heightrounded`: bring the text height to a
// whole number of \baselineskip above \topskip, to the NEAREST (geometry.sty:794-809):
//
//	d = textheight - topskip;  n = d / baselineskip;  rem = d - n*baselineskip
//	if 2*rem > baselineskip then n++
//	textheight = n*baselineskip + topskip
//
// Without it a sigconf acmart page came out 621.97pt where the reference is 626pt.
func (e *Engine) roundHeight(h int) int {
	bls, top := e.baselineskip, e.classDimen("topskip", ptToSP(10))
	if bls <= 0 {
		return h
	}
	d := h - top
	if d <= 0 {
		return h
	}
	n := d / bls
	if rem := d - n*bls; 2*rem > bls {
		n++
	}
	return n*bls + top
}
