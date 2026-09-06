// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
)

// This file implements real single-column float placement (figure/table), correcting the
// engine's long-standing simplification of typesetting floats INLINE where they appear.
// Real LaTeX collects a float into a box and places it at the TOP (or bottom, or on a
// float page) of a page, deferring it to a later page when it does not fit with at least
// \textfraction of text; a float-heavy document therefore paginates to MORE pages than
// the inline packing gives (measured: 2606.18084, figure-heavy, gotex 48pp inline vs
// tectonic 56). This attacks that under-pagination while rendering figures where TeXLive
// puts them (page top/bottom, float pages) rather than inline where written.
//
// It is ON by default; GOTEX_FLOATS=0 restores the inline path, where the float macros
// keep their untouched classic inline definitions (LaTeX2eClassLead), the
// FloatPlacementSubstrate is never loaded and Pages() never dispatches here. Measured
// over the 157 arXiv papers with a tectonic reference, placing floats and letting a
// figure state its own size (see figureDeclaredSize) beats setting them inline: Σ page
// error 616 → 586, papers within TWO pages 83 → 93, the median document 2 pages out
// either way. The yardstick is RENDERING and page count against tectonic, not the
// word-position mean: a float moved to the top of its page displaces every word below
// it, which that mean reads as divergence even when the page is now right.
//
// Reference parameters, read from latex.ltx \@xfloat / \@addtocurcol via tectonic
// \meaning: default placement `tbp`; \topfraction .7, \bottomfraction .3, \textfraction
// .2, \floatpagefraction .5; \topnumber 2, \bottomnumber 1, \totalnumber 3; \floatsep
// 12pt, \textfloatsep 20pt, \intextsep 12pt. This increment covers the standard figure/
// table environments (single column); figure*/table* keep the two-column band path
// (twocolumn.go) and [h]/[H] floats stay inline (LaTeX keeps them roughly where written).

// floatsEnabled reports whether the real float placer (this file) is active. It is the
// default; GOTEX_FLOATS=0 opts back out to the inline path.
func floatsEnabled() bool { return os.Getenv("GOTEX_FLOATS") != "0" }

// FloatPlacementSubstrate routes the figure/table environments through the placer. It is
// loaded by LoadLaTeX ONLY when floatsEnabled(), so with the flag off the environments
// keep their untouched LaTeX2eClassLead definitions and the default output is unchanged.
// It redefines only \@float (not \@dblfloat, which keeps its two-column band path, nor
// \end@float, whose group would leak): \@float is the class float mechanism every
// figure/table funnels through and the point that SURVIVES \documentclass — a class
// redefines \figure to \@float{figure} at \documentclass time but leaves \@float itself
// the base one. \gotex@inlinefloat reproduces the classic inline begin-code for the
// two-column / [h] fall-back path (@ a letter, so \@captype / \@ifnextchar are real
// control sequences).
//
// @ is already a letter here (this loads after AMSClassSubstrate, which — like this
// substrate — defines @-names with no \makeatletter), and LoadFormat does NOT save and
// restore catcodes, so this must NOT emit a \makeatother: doing so would flip @ to
// catother for everything loaded afterwards and for a bare New()+LoadLaTeX() document,
// breaking later \@-name control sequences. It leaves @ a letter, exactly as the default
// (substrate-free) end of LoadLaTeX does.
const FloatPlacementSubstrate = `
\long\def\gotex@inlinefloat#1{\par\addvspace\intextsep\begingroup\centering\def\@captype{#1}\@ifnextchar[\@gobbleopt\relax}
\def\@float#1{\gotex@floatbegin{#1}}
`

// floatNode marks a captured float in the main vertical list: its rendered box, the
// placement bits from the optional [tbp!h] argument (default "tbp"), and its anchor
// position (filled by pagesWithFloats). The page builder relocates it to a page top or
// bottom, or defers it to a float page.
type floatNode struct {
	box   *boxNode
	place string // placement bits (h/t/b/p), lower-cased; "" means default "tbp"
	kind  string // caption type: "figure" or "table" — floats of one type keep their order
}

func (*floatNode) isNode() {}

// doFloatBegin implements the redefined \@float (kind = the caption type, "figure" or
// "table"). It runs only when the placer is enabled. The placer is single-column, so
// inside a two-column region, for an [h]/[H] (here) float, or a non-standard \@float box,
// it hands the type to the inline helper — exactly the classic behaviour. Otherwise it
// captures the float environment's body (delimited by \end{\@currenvir}) into a box and
// contributes a floatNode for pagesWithFloats to place.
func (e *Engine) doFloatBegin() {
	kind := e.readBraceNameX() // caption type: "figure" or "table"
	place := "tbp"
	if toks, ok := e.scanOptBracketToks(); ok {
		if s := placementBits(toks); s != "" {
			place = s
		}
	}
	// Capture is only safe when \@float is the body of a STANDARD float environment
	// (\@currenvir is figure/table/figure*/table*): then \end{\@currenvir} bounds the
	// body. A class that calls \@float itself for a non-standard box (neurips's
	// \@float{noticebox}) leaves \@currenvir as the enclosing environment, so capturing
	// would collect to the wrong \end and swallow the document. Such calls, floats asking
	// for h/H (here) placement, and everything in a two-column region stay INLINE — LaTeX
	// keeps an [h]/[ht]/[H] float roughly where written, so floating it would misplace it
	// against the reference. Only a standard-env t/b/p float is captured.
	env := e.currentEnvName()
	if e.inTwoColumnRegion() || strings.ContainsAny(place, "hH") || !isStandardFloatEnv(env) {
		// The [placement] was consumed above, so the inline helper just typesets the body.
		e.push(append([]tok{csTok("gotex@inlinefloat")}, braceNameToks(kind)...))
		return
	}
	e.define("@captype", &meaning{kind: mMacro, body: stringToToks(kind)}, true)
	body := e.collectEnvBody(env)
	// The body is a BOX being built, so what it assigns must stay inside it: \@xfloat
	// sets the float in \vbox\bgroup…\egroup (latex.ltx:12950), and the engine's own
	// inline \@float opens a \begingroup that \end@float closes. Capturing the body
	// and running it with no group of its own let an assignment escape into the
	// document. One real paper shows the cost: a figure holding
	// \put(-0.33\textwidth,0.5\textwidth){…} — \put undefined, so \textwidth reads as
	// the start of an assignment and takes the missing number as zero — left \hsize
	// at 0pt for the remaining 230 pages, which then set ONE WORD PER LINE and ran to
	// 1439 pages against a reference of 333.
	e.beginGroup()
	box := e.typesetGroupToVbox(append([]tok{csTok("centering")}, body...))
	e.endGroup()
	box.width = e.hsize
	e.contribute(&floatNode{box: box, place: place, kind: kind})
}

// inTwoColumnRegion reports whether the material being set right now belongs to a
// two-column region — the single-column placer must leave those to the column pager.
// A \onecolumn region counts as ONE column: merely having regions at all does not make
// the page two-column, and reading it that way sent every float of an ordinary article
// back to the inline path as soon as the column machinery was live.
func (e *Engine) inTwoColumnRegion() bool {
	if e.twoColumn {
		return true
	}
	if n := len(e.colRegions); n > 0 {
		return e.colRegions[n-1].cols > 1
	}
	return false
}

// hasTwoColumnRegion reports whether any region of the document is set in two columns.
func (e *Engine) hasTwoColumnRegion() bool {
	for _, r := range e.colRegions {
		if r.cols > 1 {
			return true
		}
	}
	return false
}

// isStandardFloatEnv reports whether name is one of the standard float environments,
// whose \end bounds a capturable body.
func isStandardFloatEnv(name string) bool {
	switch name {
	case "figure", "figure*", "table", "table*":
		return true
	}
	return false
}

// currentEnvName is the value of \@currenvir (the environment \begin opened), used to end
// the captured float body at the matching \end. Falls back to the caption type's plain
// name when \@currenvir is unset.
func (e *Engine) currentEnvName() string {
	if n, ok := e.currentEnvir(); ok {
		return n
	}
	return "figure"
}

// currentEnvir is \@currenvir — the environment \begin opened — and whether it is
// set at all. Callers that have no sensible fallback (\lrbox reached through a
// class's own environment) need the second result: currentEnvName's "figure" would
// be a wrong guess for them, not a neutral one.
func (e *Engine) currentEnvir() (string, bool) {
	m := e.eq["@currenvir"]
	if m == nil {
		return "", false
	}
	var b []rune
	for _, t := range m.body {
		if !t.cs_ {
			b = append(b, t.ch)
		}
	}
	if len(b) == 0 {
		return "", false
	}
	return string(b), true
}

// kindDeferred reports whether a float of this caption type is still waiting, which
// forbids a later one of the same type from being placed first (\@bitor\@currtype
// \@deferlist in \@addtocurcol): figure 3 must never appear before figure 2.
func kindDeferred(deferred []anchoredFloat, kind string) bool {
	for _, af := range deferred {
		if af.f.kind == kind {
			return true
		}
	}
	return false
}

// mvlHasFloats reports whether the main vertical list carries any captured floatNode.
func (e *Engine) mvlHasFloats() bool {
	for _, n := range e.mvl {
		if _, ok := n.(*floatNode); ok {
			return true
		}
	}
	return false
}

// floatClass is a float's resolved placement preference, derived once from its bits.
type floatClass struct {
	allowTop, allowBot, allowPage bool
}

// classify resolves a float's placement bits (default "tbp") into the areas it may go.
func classifyFloat(place string) floatClass {
	if place == "" {
		place = "tbp"
	}
	return floatClass{
		allowTop:  strings.ContainsRune(place, 't'),
		allowBot:  strings.ContainsRune(place, 'b'),
		allowPage: strings.ContainsRune(place, 'p'),
	}
}

// anchoredFloat pairs a captured float with the text position it was written at.
type anchoredFloat struct {
	f  *floatNode
	c  floatClass
	at int // index into the text stream where the float appeared
}

// pagesWithFloats paginates a main vertical list carrying floatNodes: each page opens
// with the top floats reached so far (subject to \topnumber and \topfraction) and closes
// with any bottom floats (\bottomnumber, \bottomfraction); the text fills the height left
// between them, and floats that do not fit — or ask for [p] — defer to a float page. It
// is the float-aware counterpart of paginateSingleList, built on the same findPageBreak
// cost breaker so text is never over-packed or lost.
func (e *Engine) pagesWithFloats() []*boxNode {
	vsize := e.effectiveVsize()
	topCap := vsize * 7 / 10  // \topfraction
	botCap := vsize * 3 / 10  // \bottomfraction
	textMin := vsize / 5      // \textfraction: a page mixing floats and text keeps this much text
	floatPageMin := vsize / 2 // \floatpagefraction: a float page must be this full

	// Split the floats out of the text stream, remembering each float's anchor.
	var text []node
	var floats []anchoredFloat
	for _, n := range e.mvl {
		if fn, ok := n.(*floatNode); ok {
			floats = append(floats, anchoredFloat{f: fn, c: classifyFloat(fn.place), at: len(text)})
		} else {
			text = append(text, n)
		}
	}

	var pages []*boxNode
	var deferred []anchoredFloat // floats reached but not yet placed (FIFO)
	start, fi := 0, 0
	savedVsize := e.vsize

	fh := func(af anchoredFloat) int { return af.f.box.height + af.f.box.depth }

	emitFloatPage := func(items []anchoredFloat) {
		var vlist []node
		for i, af := range items {
			if i > 0 {
				vlist = append(vlist, glueNode{spec: glueSpec{width: e.floatSep()}})
			}
			vlist = append(vlist, af.f.box)
		}
		pages = append(pages, e.assemblePage(vlist, len(pages)+1))
	}

	for start < len(text) || len(deferred) > 0 || fi < len(floats) {
		// Absorb every float whose anchor has been reached into the deferred queue.
		for fi < len(floats) && floats[fi].at <= start {
			deferred = append(deferred, floats[fi])
			fi++
		}

		// A float taller than any top/bottom area gets its own float page immediately.
		if len(deferred) > 0 && fh(deferred[0]) > topCap && fh(deferred[0]) > botCap {
			emitFloatPage(deferred[:1])
			deferred = deferred[1:]
			if len(pages) >= maxPages {
				e.notePageLimit()
				break
			}
			continue
		}

		// If the text is exhausted, flush the remaining floats onto float pages: pack as
		// many as fit a page (up to \totalnumber), which keeps a trailing run of figures
		// from each taking a near-empty page.
		if start >= len(text) {
			if len(deferred) == 0 {
				break
			}
			var pageFloats []anchoredFloat
			h := 0
			for len(deferred) > 0 && len(pageFloats) < 3 {
				need := fh(deferred[0])
				if len(pageFloats) > 0 {
					need += e.floatSep()
				}
				if len(pageFloats) > 0 && h+need > vsize {
					break
				}
				h += need
				pageFloats = append(pageFloats, deferred[0])
				deferred = deferred[1:]
			}
			emitFloatPage(pageFloats)
			if len(pages) >= maxPages {
				e.notePageLimit()
				break
			}
			continue
		}

		// Choose top floats: reached floats that allow top, within \topfraction and
		// \topnumber (2), each taken from the FRONT of the queue so document order holds.
		var top, bottom []anchoredFloat
		topH := 0
		i := 0
		for i < len(deferred) && len(top) < 2 {
			af := deferred[i]
			if !af.c.allowTop {
				i++
				continue
			}
			need := fh(af)
			if len(top) > 0 {
				need += e.floatSep()
			}
			if topH+need > topCap {
				break
			}
			topH += need
			top = append(top, af)
			deferred = append(deferred[:i], deferred[i+1:]...)
		}

		// Choose bottom floats from what is left: allow bottom, within \bottomfraction and
		// \bottomnumber (1).
		botH := 0
		i = 0
		for i < len(deferred) && len(bottom) < 1 {
			af := deferred[i]
			if !af.c.allowBot {
				i++
				continue
			}
			if botH+fh(af) > botCap {
				i++
				continue
			}
			botH += fh(af)
			bottom = append(bottom, af)
			deferred = append(deferred[:i], deferred[i+1:]...)
		}

		reserve := topH + botH
		if topH > 0 {
			reserve += e.textFloatSep()
		}
		if botH > 0 {
			reserve += e.textFloatSep()
		}

		// Keep at least \textfraction of the page for text: if the floats leave too little
		// room, push the last-added top float (then bottom) back to the queue for a later
		// page rather than starving the text or forcing an overfull page.
		for reserve > vsize-textMin && (len(top) > 0 || len(bottom) > 0) {
			if len(top) > 0 {
				af := top[len(top)-1]
				top = top[:len(top)-1]
				deferred = append([]anchoredFloat{af}, deferred...)
				topH -= fh(af)
				if len(top) > 0 {
					topH -= e.floatSep()
				}
			} else {
				af := bottom[len(bottom)-1]
				bottom = bottom[:len(bottom)-1]
				deferred = append([]anchoredFloat{af}, deferred...)
				botH -= fh(af)
			}
			reserve = topH + botH
			if topH > 0 {
				reserve += e.textFloatSep()
			}
			if botH > 0 {
				reserve += e.textFloatSep()
			}
		}

		// If the deferred backlog is large enough to fill a float page on its own and none
		// of it could ride this page's top, emit a float page to drain it — this is what
		// stops a burst of figures from trailing far past their anchors.
		if len(top) == 0 && len(bottom) == 0 && len(deferred) > 0 {
			h, cnt := 0, 0
			for _, af := range deferred {
				h += fh(af)
				cnt++
				if cnt >= 3 {
					break
				}
			}
			if h >= floatPageMin && deferred[0].c.allowPage {
				var pageFloats []anchoredFloat
				ph := 0
				for len(deferred) > 0 && len(pageFloats) < 3 {
					need := fh(deferred[0])
					if len(pageFloats) > 0 {
						need += e.floatSep()
					}
					if len(pageFloats) > 0 && ph+need > vsize {
						break
					}
					ph += need
					pageFloats = append(pageFloats, deferred[0])
					deferred = deferred[1:]
				}
				emitFloatPage(pageFloats)
				if len(pages) >= maxPages {
					e.notePageLimit()
					break
				}
				continue
			}
		}

		// Break the text at the height left between the floats, using the real page breaker
		// (temporarily narrowing e.vsize so findPageBreak targets that height).
		pageVsize := vsize - reserve
		if pageVsize < textMin {
			pageVsize = textMin
		}
		e.vsize = pageVsize
		end := e.findPageBreak(text, start)
		e.vsize = savedVsize

		// A float written INSIDE this page is a candidate for this page's own top.
		// LaTeX contributes a float at its anchor — \@xfloat fires the output routine
		// there with \@floatpenalty — and \@addtocurcol (latex.ltx:15636) then tests it
		// against the room LEFT in the column: it goes to the top of the page being
		// built whenever \@colroom exceeds the height already set plus \textfraction
		// plus the float. That is why a figure declared halfway down a page comes out
		// at the top of THAT page. Taking only the floats anchored before the page
		// began pushed every one of them a page later, and at the end of the document
		// onto float pages of their own.
		for fi < len(floats) && floats[fi].at < end {
			af := floats[fi]
			fi++
			// \@bitor\@currtype\@deferlist: a float may not overtake an earlier one
			// of its own type that is still waiting.
			if len(top) < 2 && af.c.allowTop && !kindDeferred(deferred, af.f.kind) {
				need := fh(af)
				if len(top) > 0 {
					need += e.floatSep()
				}
				want := topH + need
				wantReserve := want + botH + e.textFloatSep()
				if botH > 0 {
					wantReserve += e.textFloatSep()
				}
				if want <= topCap && wantReserve <= vsize-textMin {
					top = append(top, af)
					topH, reserve = want, wantReserve
					pageVsize = vsize - reserve
					if pageVsize < textMin {
						pageVsize = textMin
					}
					e.vsize = pageVsize
					end = e.findPageBreak(text, start) // less room now: the text stops earlier
					e.vsize = savedVsize
					continue
				}
			}
			deferred = append(deferred, af)
		}
		pageText := trimTrailingGlue(text[start:end])

		// Emit only a non-empty page (as paginateSingleList does): an empty text break with
		// no floats to show would otherwise leave a blank leading/interior page.
		if len(top) > 0 || len(bottom) > 0 || len(pageText) > 0 {
			pages = append(pages, e.assembleMixedPage(top, pageText, bottom, len(pages)+1))
			if len(pages) >= maxPages {
				e.notePageLimit()
				break
			}
		}
		next := skipDiscardable(text, end)
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return pages
}

// assembleMixedPage stacks the page's top floats (separated by \floatsep, and from the
// text by \textfloatsep) above the text, and the bottom floats (separated from the text
// by \textfloatsep, from each other by \floatsep) below it, then hands the result to
// assemblePage for headers/footers/margins.
func (e *Engine) assembleMixedPage(top []anchoredFloat, text []node, bottom []anchoredFloat, pageNum int) *boxNode {
	var vlist []node
	for i, af := range top {
		if i > 0 {
			vlist = append(vlist, glueNode{spec: glueSpec{width: e.floatSep()}})
		}
		vlist = append(vlist, af.f.box)
	}
	if len(top) > 0 && (len(text) > 0 || len(bottom) > 0) {
		vlist = append(vlist, glueNode{spec: glueSpec{width: e.textFloatSep()}})
	}
	vlist = append(vlist, text...)
	if len(bottom) > 0 {
		if len(text) > 0 || len(top) > 0 {
			vlist = append(vlist, glueNode{spec: glueSpec{width: e.textFloatSep()}})
		}
		for i, af := range bottom {
			if i > 0 {
				vlist = append(vlist, glueNode{spec: glueSpec{width: e.floatSep()}})
			}
			vlist = append(vlist, af.f.box)
		}
	}
	return e.assemblePage(vlist, pageNum)
}

// floatSep / textFloatSep are the \floatsep (between stacked floats) and \textfloatsep
// (between the float block and the text) skips, defaulting to LaTeX's 12pt / 20pt.
func (e *Engine) floatSep() int {
	if s := e.namedSkip("floatsep"); s.width > 0 {
		return s.width
	}
	return 12 * unity
}

func (e *Engine) textFloatSep() int {
	if s := e.namedSkip("textfloatsep"); s.width > 0 {
		return s.width
	}
	return 20 * unity
}

// notePageLimit records that pagination hit the maxPages backstop.
func (e *Engine) notePageLimit() {
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS["gotex@pagelimit"]++
}
