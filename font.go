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
	px   int
	data []byte // original font bytes (for PDF embedding)
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

// NewOpenTypeFont builds a metrics source from a font and a pixel size.
func NewOpenTypeFont(fontBytes []byte, sizePx int) (*OpenTypeFont, error) {
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	return &OpenTypeFont{f: f, fc: f.NewFace(sizePx), upem: float64(f.UnitsPerEm()), px: sizePx, data: fontBytes}, nil
}

// fontBytes returns the original font blob (for the PDF driver to embed).
func (o *OpenTypeFont) fontBytes() []byte { return o.data }

// embeddableFont is a fontFace that can also supply its bytes for PDF embedding.
type embeddableFont interface {
	fontFace
	fontBytes() []byte
}

// CharDims returns a glyph's advance width and its ink height above and depth
// below the baseline, in pixels.
func (o *OpenTypeFont) CharDims(r rune) (float64, float64, float64) {
	if d, ok := o.dimCache[r]; ok {
		return d[0], d[1], d[2]
	}
	gid, ok := o.f.GlyphIndex(r)
	if !ok {
		return 0, 0, 0
	}
	// Unrounded advance: AdvanceIndexUnits returns the font-unit advance, which we
	// scale to points ourselves. AdvanceIndex would round to a whole pixel first,
	// which at a 10pt design size zeroes sub-pixel fractions and inflates text width
	// (~+1.7% on kern-rich runs), throwing off the first line-breaks of a paragraph.
	w := o.fc.AdvanceIndexUnits(gid) * o.fc.Scale()
	segs, _ := o.fc.GlyphOutline(gid)
	scale := float64(o.px) / o.upem
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
		w = float64(o.px) * 0.25
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
	sizePt() int               // the face's design size in points
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

func (o *OpenTypeFont) sizePt() int { return o.px }

// SetFont sets the current font used to measure and render characters in
// horizontal mode. Passing an *OpenTypeFont (or any fontFace) is the Go-level
// stand-in for TeX's \font primitive until font-file loading via \font lands.
func (e *Engine) SetFont(f fontFace) {
	e.curFont = f
	e.baseFont = f // the size set here is \normalsize — the 100% for \large/\small/…
	if f != nil {
		e.baseFontPx = f.sizePt()
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
	if e.baseFont == nil || e.baseFontPx == 0 {
		return
	}
	sf, ok := e.baseFont.(scalableFont)
	if !ok {
		return
	}
	px := (e.baseFontPx*permille + 500) / 1000
	if px < 1 {
		px = 1
	}
	e.selectFont(sf.atSizePx(px))
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
	if permille == 0 || permille == 1000 || e.baseFontPx == 0 {
		return
	}
	px := (e.baseFontPx*permille + 500) / 1000
	if px < 1 {
		px = 1
	}
	e.baseFontPx = px
	if sf, ok := e.baseFont.(scalableFont); ok {
		e.baseFont = sf.atSizePx(px)
		e.curFont = e.baseFont
	}
	// Re-face the family bindings (\rm/\bf/\it/\tt/\sf and their NFSS aliases) so a
	// switch inside 11/12pt body text stays at the class size rather than snapping
	// back to the 10pt design.
	for _, m := range e.eq {
		if m != nil && m.kind == mFont && m.font != nil {
			if sf, ok := m.font.(scalableFont); ok {
				m.font = sf.atSizePx(px)
			}
		}
	}
}

// scalableFont is a font that can produce a copy of itself at a different pixel
// size (same shapes, scaled metrics) — the basis for \large/\small/… . Only real
// fonts implement it; a mock or a font that can't rescale simply keeps its size.
type scalableFont interface {
	atSizePx(px int) fontFace
}

// atSizePx returns the same font re-faced at px, sharing the parsed tables and
// embeddable bytes (a new face is cheap; no re-parse).
func (o *OpenTypeFont) atSizePx(px int) fontFace {
	return &OpenTypeFont{f: o.f, fc: o.f.NewFace(px), upem: o.upem, px: px, data: o.data}
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
	if !ok || size <= 0 || e.baseFontPx == 0 {
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
	px := int(float64(e.baseFontPx)*size/norm + 0.5)
	if px < 1 {
		px = 1
	}
	e.selectFont(sf.atSizePx(px))
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
