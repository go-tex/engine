// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
)

// This file adds the SELECTABLE TEXT LAYER to the engine's SVG output.
//
// The glyphs themselves are emitted as <path> outlines (boxrender.go): that is
// what makes the page look right at any zoom and independent of the reader's
// installed fonts. But an outline is a shape, not a character — a page made only
// of paths cannot be searched, selected, copied or read aloud, whichever way it
// is displayed. Rasterising it to a bitmap does not lose that information; the
// SVG never carried it.
//
// So alongside each run of glyphs this file emits the run's actual characters as
// an invisible <text> positioned over them, the way a PDF viewer overlays a text
// layer on a rendered page. Each word is a <tspan> pinned at the x where its
// glyphs start and forced to their measured width (textLength +
// lengthAdjust="spacingAndGlyphs"), so a selection highlight lands on the words
// it covers; the literal spaces between the tspans (kept by xml:space="preserve")
// are what let a phrase search match across a space.
//
// The layer is inert for a rasteriser — go-gfx/gfx/svg draws shapes and skips
// <text> — so the bitmap path is byte-for-byte unchanged. It only comes alive
// when the SVG is handed to something that lays out text: a browser's DOM.

// textWord is one word of the layer: the characters, the x where the first
// glyph's origin sits and the total advance the glyphs occupy (both in points).
type textWord struct {
	x, width float64
	runes    []rune
}

// textRun accumulates the characters painted along one baseline so they can be
// emitted as a single <text>. Words are split on inter-word glue; a kern stays
// inside the word it belongs to (it is a fit adjustment, not a space).
type textRun struct {
	baseline float64
	size     float64 // largest character size in the run, in points
	words    []textWord
	space    bool // an inter-word space is owed before the next character
}

// textCursor remembers where the last run ended, so the next one can tell
// whether a word boundary belongs between them.
//
// A run ends whenever anything that is not a glyph interrupts it — a formula, a
// table cell, an inline box, the end of a line — and each becomes its own
// <text>. Adjacent elements concatenate with NOTHING between them, so without a
// boundary the page read "The mass isexactly" around every formula and
// "alphabeta" across every table cell: a phrase search spanning the
// interruption found nothing, silently. A page that claims to be searchable and
// is wrong about it is worse than one that makes no claim.
//
// The rule is the one a PDF text extractor uses, and it is geometric rather
// than structural: a boundary exists where the reader can SEE one. Deciding
// from the kind of node that interrupted, or from the kind of glue around it,
// gets it wrong in both directions — a list label is centred with the same
// infinitely stretchable glue that separates two table cells.
type textCursor struct {
	baseline float64
	endX     float64 // where the last run's last glyph stopped
	size     float64
	lastRune rune
	live     bool
}

// wantsSpace reports whether a run starting at x on the given baseline needs a
// leading space to separate it from what came before.
func (c *textCursor) wantsSpace(x, baseline, size float64) bool {
	if !c.live {
		return false // the first text on the page begins nothing
	}
	if baseline != c.baseline && x < c.endX {
		// The text went back to the left on a new baseline: a line ended, which is
		// a word boundary however the words wrapped. A baseline change that keeps
		// moving RIGHT is not a new line — it is a superscript or a subscript,
		// and a footnote marker belongs to the word it hangs off.
		return true
	}
	em := size
	if c.size > em {
		em = c.size
	}
	// A quarter of an em: wider than any kern or italic correction, narrower
	// than any inter-word space.
	return x-c.endX > 0.25*em
}

// addChar extends the run with one character whose glyph origin is at x and
// which advances by width (points), setting the character at size points.
func (t *textRun) addChar(ch rune, x, width, size float64) {
	if size > t.size {
		t.size = size
	}
	if t.space || len(t.words) == 0 {
		t.words = append(t.words, textWord{x: x})
		t.space = false
	}
	w := &t.words[len(t.words)-1]
	w.runes = append(w.runes, expandLigature(ch)...)
	w.width = x + width - w.x
}

// addSpace records that the next character starts a new word. Repeated glue
// still yields a single space: the layer describes what the reader would type to
// search for the text, not the typesetter's spacing. Glue arriving before the
// run has any word is ignored — the boundary in front of a run is the cursor's
// decision, not this one's (see [textCursor]).
func (t *textRun) addSpace() {
	if len(t.words) > 0 {
		t.space = true
	}
}

// empty reports whether the run has nothing worth emitting.
func (t *textRun) empty() bool {
	for _, w := range t.words {
		if len(w.runes) > 0 {
			return false
		}
	}
	return true
}

// reset clears the run for the next baseline.
func (t *textRun) reset() { t.words, t.space, t.size = nil, false, 0 }

// emit writes the run as one invisible <text> and clears it. Nothing is written
// for an empty run, so a page of pure rules or images gains no elements.
func (t *textRun) emit(sb *strings.Builder, cur *textCursor) {
	if t.empty() {
		t.reset()
		return
	}
	size := t.size
	if size <= 0 {
		size = 10
	}
	fmt.Fprintf(sb, `<text x="%s" y="%s" font-size="%s" fill-opacity="0" xml:space="preserve">`,
		f(t.words[0].x), f(t.baseline), f(size))
	if cur.wantsSpace(t.words[0].x, t.baseline, size) && !endsWithHyphen(cur.lastRune) {
		sb.WriteString(" ")
	}
	wrote := false
	for _, w := range t.words {
		if len(w.runes) == 0 {
			continue
		}
		if wrote {
			sb.WriteString(" ")
		}
		wrote = true
		// textLength pins the word to the advance its glyphs actually occupy, so
		// the invisible text tracks the visible outlines whatever font the reader's
		// browser substitutes. A non-positive width (a zero-advance combining run)
		// is emitted unpinned rather than with a meaningless textLength.
		if w.width > 0 {
			fmt.Fprintf(sb, `<tspan x="%s" textLength="%s" lengthAdjust="spacingAndGlyphs">%s</tspan>`,
				f(w.x), f(w.width), escapeXMLText(string(w.runes)))
		} else {
			fmt.Fprintf(sb, `<tspan x="%s">%s</tspan>`, f(w.x), escapeXMLText(string(w.runes)))
		}
	}
	sb.WriteString(`</text>`)
	t.advance(cur)
	t.reset()
}

// advance records where this run left the cursor: the end of its last glyph, on
// its own baseline.
func (t *textRun) advance(cur *textCursor) {
	for i := len(t.words) - 1; i >= 0; i-- {
		w := t.words[i]
		if len(w.runes) == 0 {
			continue
		}
		cur.baseline, cur.endX, cur.size, cur.live = t.baseline, w.x+w.width, t.size, true
		cur.lastRune = w.runes[len(w.runes)-1]
		return
	}
}

// endsWithHyphen reports whether the previous run ended on a hyphen TeX inserted
// to break a word across lines. Separating there would turn "hyphen-ated" into
// "hyphen- ated", moving the damage rather than repairing it.
func endsWithHyphen(r rune) bool {
	switch r {
	case '-', 0x2010, 0x2011: // hyphen-minus, hyphen, non-breaking hyphen
		return true
	}
	return false
}

// expandLigature returns the characters a reader would type for a glyph the
// ligature program folded. "office" is set with one ﬃ glyph; a search for
// "office" must still match it, so the layer carries the three letters. Every
// other rune passes through unchanged — the dashes and curly quotes are what the
// author asked for and what a copy should yield.
func expandLigature(ch rune) []rune {
	switch ch {
	case ligFF:
		return []rune{'f', 'f'}
	case ligFI:
		return []rune{'f', 'i'}
	case ligFL:
		return []rune{'f', 'l'}
	case ligFFI:
		return []rune{'f', 'f', 'i'}
	case ligFFL:
		return []rune{'f', 'f', 'l'}
	}
	return []rune{ch}
}

// escapeXMLText escapes the three characters that cannot appear literally in XML
// character data. Attribute escaping (escapeXMLAttr) is a different set.
func escapeXMLText(s string) string {
	if !strings.ContainsAny(s, "&<>") {
		return s
	}
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// charSizePt is the size a character renders at, in points: its own size when
// \large & co. set one, else the render font's design size.
func charSizePt(c charNode, font fontFace) float64 {
	if c.size > 0 {
		return float64(c.size)
	}
	return float64(font.sizePt())
}
