// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "github.com/go-opentype/opentype"

// FontMetrics supplies box dimensions to the typesetter: the advance/height/
// depth of a glyph, and the interword glue.
type FontMetrics interface {
	CharDims(r rune) (w, h, d float64)
	Space() (w, stretch, shrink float64)
}

// OpenTypeFont adapts a go-opentype face to FontMetrics (pdftex uses TFM; a
// future TFM backend will satisfy the same interface).
type OpenTypeFont struct {
	f    *opentype.Font
	fc   *opentype.Face
	upem float64
	px   int
}

// NewOpenTypeFont builds a metrics source from a font and a pixel size.
func NewOpenTypeFont(fontBytes []byte, sizePx int) (*OpenTypeFont, error) {
	f, err := opentype.Parse(fontBytes)
	if err != nil {
		return nil, err
	}
	return &OpenTypeFont{f: f, fc: f.NewFace(sizePx), upem: float64(f.UnitsPerEm()), px: sizePx}, nil
}

// CharDims returns a glyph's advance width and its ink height above and depth
// below the baseline, in pixels.
func (o *OpenTypeFont) CharDims(r rune) (float64, float64, float64) {
	gid, ok := o.f.GlyphIndex(r)
	if !ok {
		return 0, 0, 0
	}
	w := float64(o.fc.AdvanceIndex(gid))
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
	return w, -minY, maxY
}

// Space returns TeX-like interword glue derived from the space advance.
func (o *OpenTypeFont) Space() (float64, float64, float64) {
	w := float64(o.fc.Advance(' '))
	if w == 0 {
		w = float64(o.px) * 0.25
	}
	return w, w * 0.5, w / 3
}

// Typeset runs the gullet over src (expanding macros), builds a horizontal list
// with the given font's metrics (glyph boxes and interword glue), and breaks it
// into a paragraph box with Knuth–Plass. It stops at \par, end of input, or an
// undefined control sequence.
func (e *Engine) Typeset(src string, m FontMetrics, lineWidth, tolerance, linePenalty, baselineskip float64) (Paragraph, bool) {
	e.base = []rune(src)
	e.bpos = 0
	var hl []Item
	for {
		t, ok := e.getXToken()
		if !ok {
			break
		}
		if t.cs_ {
			mm := e.meaningOf(t)
			if mm == nil {
				break // undefined ends the run (simplified)
			}
			if mm.kind == mPrim && !isExpandable(mm.name) {
				if mm.name == "par" {
					break
				}
				mm.prim(e)
			}
			continue
		}
		switch t.cat {
		case catSpace:
			w, st, sh := m.Space()
			hl = append(hl, Glue(w, st, sh))
		case catLetter, catOther:
			w, h, d := m.CharDims(t.ch)
			hl = append(hl, Glyph(t.ch, w, h, d))
		case catBegin:
			e.beginGroup()
		case catEnd:
			e.endGroup()
		}
	}
	// \parfillskip (fills the last line) + a forced final break.
	hl = append(hl, Glue(0, 1e6, 0), Penalty(0, -InfPenalty, false))
	return BuildParagraph(hl, lineWidth, tolerance, linePenalty, baselineskip)
}
