// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "github.com/go-opentype/opentype"

// OpenTypeFont adapts a go-opentype face for the engine: it measures glyphs (in
// points via CharDims and in scaled points via charDimsSP), supplies the interword
// glue, and returns glyph outline paths for the SVG driver.
type OpenTypeFont struct {
	f    *opentype.Font
	fc   *opentype.Face
	upem float64
	// pt is the face's design size in POINTS, and it is fractional on purpose.
	// LaTeX's size ladder is a table of points, not a scale factor, and three of
	// its ten entries are not whole: size10.clo gives \Large 14.4, \LARGE 17.28 and
	// \huge 20.74. Holding this as an int put \Large on 14 — 2.8% narrow on every
	// section heading in every document, measured at 13.95 against tectonic's 14.35
	// (#714, which named this as its residual).
	pt   float64
	data []byte // original font bytes (for PDF embedding)
	// ligCache memoises glyphFor's GSUB fallback, where 0 records "this face
	// cannot ligate that form" — a negative worth caching, since the paragraph
	// builder asks for every f-pair it meets and shaping is not free.
	ligCache map[rune]opentype.GlyphIndex
	// dimCache memoises CharDims per rune. Measuring a glyph's y-extent means
	// running the (CFF Type2) outline interpreter, which is expensive; a face is
	// a fixed size, so a glyph's dimensions never change and are safe to cache.
	// Without it, a document re-interprets the outline of every character it sets
	// (rawAppendChar → charDimsSP → CharDims), which dominates run time on
	// glyph-heavy math papers.
	dimCache map[rune][3]float64
	// pathCache memoises the SVG outline per rune, for the same reason and with the
	// same safety argument as dimCache: the face is a fixed size, so a glyph's path
	// never changes. Without it every OCCURRENCE of a character re-ran the CFF
	// interpreter and re-formatted every coordinate of its outline — measured on a
	// 198-page thesis, 356k glyph paintings over ~100 distinct characters, so the
	// same few outlines were rebuilt thousands of times each. That single omission
	// accounted for most of the render's 4.2 GB of allocation and 25 M objects.
	pathCache map[rune]string
}

// NewOpenTypeFont builds a metrics source from a font and a size in points.
func NewOpenTypeFont(fontBytes []byte, sizePt float64) (*OpenTypeFont, error) {
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	return &OpenTypeFont{f: f, fc: f.NewFaceAt(sizePt), upem: float64(f.UnitsPerEm()), pt: sizePt, data: fontBytes}, nil
}

// fontBytes returns the original font blob (for the PDF driver to embed).
func (o *OpenTypeFont) fontBytes() []byte { return o.data }

// embeddableFont is a fontFace that can also supply its bytes for PDF embedding.
type embeddableFont interface {
	fontFace
	fontBytes() []byte
}

// glyphFor resolves a rune to a glyph, asking the FONT and not only its cmap.
//
// ⛔⛔ THE CMAP IS THE WRONG QUESTION FOR A LIGATURE. The paragraph builder folds
// "fi" into U+FB01 and then everything downstream — this, and the drawing path —
// looks that presentation form up. A font that files its f-ligatures in GSUB
// alone, which is ordinary modern practice, has no cmap entry for U+FB01 and so
// answered "no glyph", though it has the glyph and go-opentype can reach it:
// Shape("fi", "liga") returns exactly one.
//
// Measured on LibertinusSerif at 10pt against XeTeX, per occurrence:
//
//	ff   6.20 against 5.82   lost 0.38pt
//	fi   5.81 against 5.60   lost 0.21pt
//	fl   6.14 against 5.40   lost 0.74pt
//	ffi  8.91 against 8.29   lost 0.62pt
//	ffl  9.24 against 8.15   lost 1.09pt
//
// and "fi" is in first, find, file, specific, definition, significant,
// efficient. TeXGyreTermesX is the control that kept this invisible: it DOES name
// its ligatures in cmap, so the old question worked there and matched XeTeX
// exactly.
//
// ⛔ The two halves of the same glyph disagreed about how to find it: the drawing
// path already shapes (pdfdriver.go's TextShaped with "liga"), only measuring
// went through the cmap — so on a font like Libertinus the fold never happened
// and the drawing path was never reached.
//
// The components come from expandLigature, which already holds this mapping for
// the text layer; a second table would be a second thing to keep in step.
func (o *OpenTypeFont) glyphFor(r rune) (opentype.GlyphIndex, bool) {
	if gid, ok := o.f.GlyphIndex(r); ok {
		return gid, true
	}
	if o.ligCache == nil {
		o.ligCache = map[rune]opentype.GlyphIndex{}
	} else if gid, ok := o.ligCache[r]; ok {
		return gid, gid != 0
	}
	comps := expandLigature(r)
	if len(comps) < 2 {
		return 0, false // not a form this engine folds into: the cmap was the answer
	}
	// Exactly one glyph, and not .notdef: a face that cannot ligate returns the
	// components unchanged, which is the "no" this has to report.
	gids := o.fc.Shape(string(comps), "liga")
	if len(gids) != 1 || gids[0] == 0 {
		o.ligCache[r] = 0
		return 0, false
	}
	o.ligCache[r] = gids[0]
	return gids[0], true
}

// hasGlyphFor reports whether this face can set r as a single glyph — the
// question the ligature program needs, and which glyphPathAt could not answer
// for a GSUB-only ligature.
func (o *OpenTypeFont) hasGlyphFor(r rune) bool {
	_, ok := o.glyphFor(r)
	return ok
}

// CharDims returns a glyph's advance width and its ink height above and depth
// below the baseline, in pixels.
func (o *OpenTypeFont) CharDims(r rune) (float64, float64, float64) {
	if d, ok := o.dimCache[r]; ok {
		return d[0], d[1], d[2]
	}
	gid, ok := o.glyphFor(r)
	if !ok {
		return 0, 0, 0
	}
	// Unrounded advance: AdvanceIndexUnits returns the font-unit advance, which we
	// scale to points ourselves. AdvanceIndex would round to a whole pixel first,
	// which at a 10pt design size zeroes sub-pixel fractions and inflates text width
	// (~+1.7% on kern-rich runs), throwing off the first line-breaks of a paragraph.
	w := o.fc.AdvanceIndexUnits(gid) * o.fc.Scale()
	segs, _ := o.fc.GlyphOutline(gid)
	// The face's own scale, not one recomputed from a rounded size: the face is
	// built at a fractional point size, and float64(px)/upem would quietly put the
	// outline back on the whole-point ladder the advance above has already left.
	scale := o.fc.Scale()
	var minY, maxY float64
	for _, s := range segs {
		for _, p := range s.P {
			y := -p.Y * scale
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if o.dimCache == nil {
		o.dimCache = map[rune][3]float64{}
	}
	o.dimCache[r] = [3]float64{w, -minY, maxY}
	return w, -minY, maxY
}

// glyphPath returns the SVG path ("d") for a rune's glyph at the face's size,
// baseline-relative in SVG (Y-down) coordinates. Empty if the font lacks it.
func (o *OpenTypeFont) glyphPath(r rune) string {
	if d, ok := o.pathCache[r]; ok {
		return d
	}
	gid, ok := o.f.GlyphIndex(r)
	if !ok {
		if o.pathCache == nil {
			o.pathCache = map[rune]string{}
		}
		o.pathCache[r] = "" // a missing glyph is worth caching too: it is looked up as often
		return ""
	}
	d, _ := o.fc.GlyphSVGPath(gid)
	if o.pathCache == nil {
		o.pathCache = map[rune]string{}
	}
	o.pathCache[r] = d
	return d
}

// Space returns TeX-like interword glue derived from the space advance.
func (o *OpenTypeFont) Space() (float64, float64, float64) {
	w := float64(o.fc.Advance(' '))
	if w == 0 {
		w = o.pt * 0.25
	}
	return w, w * 0.5, w / 3
}

// fontFace is the engine's current-font abstraction: glyph metrics in scaled
// points plus an optional outline path for rendering. OpenTypeFont satisfies it;
// tests use a deterministic mock. The face's pixel size is interpreted as its
// point size, so 1px = 1pt and glyph coordinates match the sp box geometry.
type fontFace interface {
	charDimsSP(r rune) (w, h, d int)
	spaceSP() glueSpec
	glyphPathAt(r rune) string // "" if the font lacks the glyph
	kernSP(prev, cur rune) int // inter-character kern in sp (0 if none)
	// hasGlyphFor reports whether the face can set r as ONE glyph. It is not
	// glyphPathAt(r) != "": a ligature filed in GSUB alone has no cmap entry for
	// its presentation form, so the path lookup says no about a glyph the font
	// has (see glyphFor).
	hasGlyphFor(r rune) bool
	sizePt() float64           // the face's design size in points (fractional: \Large is 14.4)
}

// faceProgram identifies the font PROGRAM behind a face, independently of the size
// it is faced at: \large and \normalsize of one family share it. A PDF embeds one
// subset per program, so a driver that keyed its fonts on the face would embed the
// same family once per size a document uses.
type faceProgram interface{ faceProgram() any }

func (o *OpenTypeFont) faceProgram() any { return o.f }

// programOf is a face's cache key: its program when it can name one, and the face
// itself otherwise (a mock, nullFont — each its own key, which is correct if
// wasteful, and neither is ever embedded).
func programOf(f fontFace) any {
	if p, ok := f.(faceProgram); ok {
		return p.faceProgram()
	}
	return f
}

// charFace is the face a glyph must be DRAWN with, and the scale its outline needs
// relative to that face.
//
// A node that names its own face is drawn with it at scale 1: the face carries the
// size the glyph was measured at, so there is nothing left to scale. A node with
// none falls back to the driver's single face, scaled to the glyph's recorded size
// — which is how every glyph was drawn before charNode carried a face.
func charFace(c charNode, driver fontFace) (fontFace, float64) {
	if c.face != nil {
		return c.face, 1
	}
	if driver == nil {
		return nil, 1
	}
	return driver, glyphScale(c, driver)
}

// ptToSP rounds points to scaled points.
func ptToSP(pt float64) int { return int(pt*float64(unity) + 0.5) }

func (o *OpenTypeFont) charDimsSP(r rune) (int, int, int) {
	w, h, d := o.CharDims(r)
	return ptToSP(w), ptToSP(h), ptToSP(d)
}

func (o *OpenTypeFont) spaceSP() glueSpec {
	w, st, sh := o.Space()
	return glueSpec{width: ptToSP(w), stretch: ptToSP(st), shrink: ptToSP(sh)}
}

func (o *OpenTypeFont) glyphPathAt(r rune) string { return o.glyphPath(r) }

// kernSP returns the inter-character kern in scaled points. It uses KernUnits (the
// unrounded font-unit kern) scaled to points rather than Kern, which rounds to a
// whole pixel first — at a 10pt design size that whole-pixel round drops sub-pixel
// kerns to zero, widening kern-rich text and diverging from real LaTeX line-breaks.
func (o *OpenTypeFont) kernSP(prev, cur rune) int {
	return ptToSP(o.fc.KernUnits(prev, cur) * o.fc.Scale())
}

func (o *OpenTypeFont) sizePt() float64 { return o.pt }

// SetFont sets the current font used to measure and render characters in
// horizontal mode. Passing an *OpenTypeFont (or any fontFace) is the Go-level
// stand-in for TeX's \font primitive until font-file loading via \font lands.
func (e *Engine) SetFont(f fontFace) {
	e.curFont = f
	e.baseFont = f // the size set here is \normalsize — the 100% for \large/\small/…
	if f != nil {
		e.baseFontPt = f.sizePt()
	}
}

// loadSelectFont installs \gotex@selectbasefont, the primitive \selectfont is
// defined in terms of: it makes the base (roman, \normalsize) face current
// again. It is scoped like any other font switch, so a group restores whatever
// was in force before.
func (e *Engine) loadSelectFont() {
	e.prim("gotex@selectbasefont", func(e *Engine) {
		if e.baseFont != nil {
			e.selectFont(e.baseFont)
		}
	})
}

// renderFont is the font the drivers use as the glyph source and size reference:
// the \normalsize base (so per-glyph sizes scale relative to a fixed 100%), or the
// current font when no base was set.
func (e *Engine) renderFont() fontFace {
	if e.baseFont != nil {
		return e.baseFont
	}
	return e.curFont
}

// doFontSize implements the \gotexsize primitive: it reads a permille factor and
// switches the current font to the base (\normalsize) font scaled by that factor,
// so \large (1200) is 1.2× the base size. selectFont scopes the change to the
// current group. A font that can't rescale leaves the size unchanged.
func (e *Engine) doFontSize() {
	permille := e.scanInt()
	if e.baseFont == nil || e.baseFontPt == 0 {
		return
	}
	sf, ok := e.baseFont.(scalableFont)
	if !ok {
		return
	}
	pt := e.baseFontPt * float64(permille) / 1000
	if pt < 1 {
		pt = 1
	}
	e.selectFont(sf.atSize(pt))
}

// scaleClassFontsToBase re-faces every bound TEXT face — the \normalsize base
// (which \selectfont/\normalfont/\rmfamily reselect and the drivers render from)
// and the roman/bold/italic/typewriter/sans bindings — to the class's base point
// size, then makes the base face current. A [11pt]/[12pt] document class scales
// the 10pt design by 110%/120% (permille), so \normalsize body text is set at
// 11/12pt and wraps like real LaTeX; \large/\small/… follow, scaling off the new
// base through \gotexsize. This drives the class base size entirely through the
// font system, so it is robust against every font-selection command and needs no
// change to \@setfontsize's tokens. A 10pt class (permille 1000) is a no-op, so
// 10pt documents stay byte-identical. Non-scalable faces (mocks) are left as-is.
func (e *Engine) scaleClassFontsToBase(permille int) {
	if permille == 0 || permille == 1000 || e.baseFontPt == 0 {
		return
	}
	pt := e.baseFontPt * float64(permille) / 1000
	if pt < 1 {
		pt = 1
	}
	e.baseFontPt = pt
	if sf, ok := e.baseFont.(scalableFont); ok {
		e.baseFont = sf.atSize(pt)
		e.curFont = e.baseFont
	}
	// Re-face the family bindings (\rm/\bf/\it/\tt/\sf and their NFSS aliases) so a
	// switch inside 11/12pt body text stays at the class size rather than snapping
	// back to the 10pt design.
	for _, m := range e.eq {
		if m != nil && m.kind == mFont && m.font != nil {
			if sf, ok := m.font.(scalableFont); ok {
				m.font = sf.atSize(pt)
			}
		}
	}
}

// scalableFont is a font that can produce a copy of itself at a different point
// size (same shapes, scaled metrics) — the basis for \large/\small/… . Only real
// fonts implement it; a mock or a font that can't rescale simply keeps its size.
type scalableFont interface {
	atSize(pt float64) fontFace
}

// atSize returns the same font re-faced at pt points, sharing the parsed tables
// and embeddable bytes (a new face is cheap; no re-parse).
//
// NewFaceAt and not NewFace: the latter takes a whole number, so every fractional
// entry of LaTeX's size table was truncated on the way in. go-opentype v0.17.1
// added the fractional constructor for exactly this caller; the face still
// rasterises at a whole ppem, because hinting is whole-pixel, while its metrics
// scale by the true size (go-opentype/opentype#62).
func (o *OpenTypeFont) atSize(pt float64) fontFace {
	return &OpenTypeFont{f: o.f, fc: o.f.NewFaceAt(pt), upem: o.upem, pt: pt, data: o.data}
}

// faceAtCurrentSize re-faces a FAMILY switch at the size already in force.
//
// ⛔⛔ \rm, \bf, \it, \tt and \sf change the family; they do not change the size.
// Each is bound to a face built once, at the document's base size (api.go's
// bindOptionalFont), so selecting one as bound silently undid any size switch
// before it. LaTeX's own \section states its style as
//
//	{\normalfont\Large\bfseries}
//
// — size first, series LAST — so every section heading in every document came out
// at body size. Measured against tectonic on a two-word witness:
//
//	{\Large …}            ours 13.95   reference 14.35
//	{\Large\bfseries …}   ours  9.96   reference 14.35   ← the defect
//	{\large …}            ours 11.96   reference 11.96
//
// The size the switch must keep is the CURRENT face's, not the base: a heading
// inside a \small environment is \Large relative to that, and reading e.baseFontPx
// here would quietly re-anchor it.
//
// ⚠ A face that cannot rescale — a test mock, a bitmap — keeps its own size rather
// than being dropped: a family switch that silently did nothing would be worse than
// one that ignores the size.
func (e *Engine) faceAtCurrentSize(f fontFace) fontFace {
	if f == nil || e.curFont == nil {
		return f
	}
	pt := e.curFont.sizePt()
	if pt <= 0 || pt == f.sizePt() {
		return f
	}
	sf, ok := f.(scalableFont)
	if !ok {
		return f
	}
	return sf.atSize(pt)
}

// doFontSizeAt implements the \gotex@fontsizeat primitive: it reads the size a
// class size table states for one of \tiny…\Huge and puts the font there.
//
// LaTeX's ladder is a TABLE, not a scale factor. Every class size option file
// redefines all ten commands (size11.clo:80-86, and our byte-identical copies in
// texmf/):
//
//	\DeclareRobustCommand\large{\@setfontsize\large\@xiipt{14}}
//
// so \large is 12pt in a 10pt AND in an 11pt document, and size12.clo caps \Huge
// at \@xxvpt — the same 24.88pt as \huge. \@setfontsize is the hook they all go
// through (latex.ltx:10003-10007: \fontsize{#2}{#3}\selectfont), and ours threw
// the size away, so every one of those redefinitions landed as a no-op: a document
// that loaded article.cls came out with ONE font size for \tiny through \Huge —
// a probe of all ten set at 10pt, \Huge 57% too narrow.
//
// The table's points are taken RELATIVE to the class's own \normalsize, not as
// absolute points: Options.Size lets a caller pick the base size, and everything
// scales off it. The ratio is what the table actually states — \Large is 14.4/10
// of the body in a 10pt class and 14.4/10.95 in an 11pt one. The switch is scoped
// like any other, so a group restores it.
func (e *Engine) doFontSizeAt() {
	size, ok := e.scanPtArg()
	if !ok || size <= 0 || e.baseFontPt == 0 {
		return
	}
	norm := e.classNormalsizePt
	if norm <= 0 {
		norm = 10 // the design size the kernel's number macros are stated against
	}
	f := e.curFont
	if f == nil {
		f = e.baseFont
	}
	sf, sok := f.(scalableFont)
	if !sok {
		return
	}
	// No rounding: the table's own number is the size. \Large is 14.4 of a 10pt
	// body, and rounding it here was the whole defect.
	pt := e.baseFontPt * size / norm
	if pt < 1 {
		pt = 1
	}
	e.selectFont(sf.atSize(pt))
}

// doClassNormalsize implements \gotex@classnormalsize: it records the size the
// class states for \normalsize, which is what the rest of its table is measured
// against. \@setfontsize reports it when the command being (re)defined is
// \normalsize, the same test that already reports the body leading.
func (e *Engine) doClassNormalsize() {
	if v, ok := e.scanPtArg(); ok && v > 0 {
		e.classNormalsizePt = v
	}
}

// scanPtArg reads one brace group, expands it and reads a point value from it.
// The class tables state a size as one of the kernel's number macros (\@xivpt →
// 14.4), so the argument has to be expanded before it can be read.
func (e *Engine) scanPtArg() (float64, bool) {
	return parseFloatArg(e.toksToString(e.expandList(e.grabUndelimited())))
}
