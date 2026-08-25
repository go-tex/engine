// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
)

// This file carries the page's characters in a SELECTABLE TEXT LAYER.
//
// The glyphs themselves are emitted as <path> outlines (boxrender.go): that is
// what makes a page look right at any zoom and independent of the reader's
// installed fonts. But an outline is a shape, not a character — a page made
// only of paths cannot be searched, selected, copied or read aloud. Rasterising
// it to a bitmap does not lose that information; the SVG never carried it.
//
// So the characters are emitted a second time, invisibly, positioned over their
// own outlines, the way a PDF viewer overlays a text layer on a rendered page.
//
// ⚠ THE WHOLE PAGE IS ONE <text>, and that is the point. A browser's find does
// NOT match a phrase spanning two <text> elements — measured, in Chrome: the
// same words in two <text> elements are unfindable while the same words as two
// <tspan> of ONE <text> are found. Emitting a <text> per line, or per run of
// glyphs, therefore caps search at whatever fits in one of them, which is how
// "The rest mass is $E=mc^2$ exactly" stayed unsearchable even after its word
// boundaries were correct: the boundaries fixed textContent, which screen
// readers and copy read, and did nothing for find.
//
// So every run becomes a <tspan> carrying its own absolute x and y inside a
// single page-level <text>. The one thing that splits it is a TRANSFORM: text
// inside \rotatebox or \scalebox is positioned by that matrix and cannot be
// hoisted out of it, so it gets its own <text> inside the same <g>. A document
// with no rotated text — nearly all of them — is one chunk from corner to
// corner, and a phrase is findable across formulas, table cells and line
// breaks alike.
//
// The layer is inert to a rasteriser — go-gfx/gfx/svg draws shapes and skips
// <text> — so the bitmap path is byte-for-byte unchanged. It only comes alive
// when the SVG is handed to something that lays out text: a browser's DOM.

// textWord is one word of the layer: the characters, the x where the first
// glyph's origin sits and the total advance the glyphs occupy (both in points).
type textWord struct {
	x, width float64
	runes    []rune
}

// textRun accumulates the characters painted along one baseline until something
// that is not a glyph interrupts them. Words are split on inter-word glue; a
// kern stays inside the word it belongs to (it is a fit adjustment, not a
// space).
type textRun struct {
	baseline float64
	size     float64 // largest character size in the run, in points
	words    []textWord
	space    bool // an inter-word space is owed before the next character
}

// textChunk is one <text> element under construction: everything written while
// the same transform was in force.
type textChunk struct {
	transform string // the enclosing <g transform="…">, empty when there is none
	body      strings.Builder
	any       bool
}

// textLayer collects the whole page's chunks, in document order, and the cursor
// that decides where word boundaries go.
type textLayer struct {
	chunks []*textChunk
	stack  []string // enclosing transforms, innermost last
	cur    textCursor
}

// newTextLayer starts a layer with one untransformed chunk open.
func newTextLayer() *textLayer {
	l := &textLayer{}
	l.open("")
	return l
}

// open starts a new chunk under the given transform and makes it current.
func (l *textLayer) open(transform string) {
	l.chunks = append(l.chunks, &textChunk{transform: transform})
}

// chunk is the chunk being written to.
func (l *textLayer) chunk() *textChunk { return l.chunks[len(l.chunks)-1] }

// pushTransform enters a transformed context: its text needs its own <text>
// inside the same <g>, because the matrix is what places it.
//
// The cursor is reset, so no word boundary is inferred across the boundary — a
// rotated caption is not a continuation of the paragraph beside it.
func (l *textLayer) pushTransform(transform string) {
	l.stack = append(l.stack, transform)
	l.open(strings.Join(l.stack, " "))
	l.cur = textCursor{}
}

// popTransform leaves it, resuming the enclosing context in a fresh chunk so
// the document order of the output is preserved.
func (l *textLayer) popTransform() {
	l.stack = l.stack[:len(l.stack)-1]
	l.open(strings.Join(l.stack, " "))
	l.cur = textCursor{}
}

// String renders every non-empty chunk. An empty layer renders nothing, so a
// page of pure rules or images gains no elements.
func (l *textLayer) String() string {
	var sb strings.Builder
	for _, c := range l.chunks {
		if !c.any {
			continue
		}
		if c.transform != "" {
			fmt.Fprintf(&sb, `<g transform="%s">`, c.transform)
		}
		sb.WriteString(`<text fill-opacity="0" xml:space="preserve">`)
		sb.WriteString(c.body.String())
		sb.WriteString(`</text>`)
		if c.transform != "" {
			sb.WriteString(`</g>`)
		}
	}
	return sb.String()
}

// textCursor remembers where the last run ended, so the next one can tell
// whether a word boundary belongs between them.
//
// A run ends whenever anything that is not a glyph interrupts it — a formula, a
// table cell, an inline box, the end of a line. Without a boundary the page
// read "The mass isexactly" around every formula and "alphabeta" across every
// table cell, and a phrase search spanning the interruption found nothing while
// reporting nothing wrong. A page that claims to be searchable and is wrong
// about it is worse than one that makes no claim.
//
// The rule is the one a PDF text extractor uses, and it is GEOMETRIC rather
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
	// owed records that inter-word glue was pending when the last run ended —
	// the run did not stop because something interrupted it, it stopped holding a
	// space. That is EXACT information and it outranks the geometric guess below,
	// which cannot see a space narrow enough to be mistaken for a kern.
	owed bool
}

// wantsSpace reports whether a run starting at x on the given baseline needs a
// leading space to separate it from what came before.
func (c *textCursor) wantsSpace(x, baseline, size float64) bool {
	if !c.live {
		return false // the first text on the page begins nothing
	}
	if c.owed {
		// A space was typed and the run ended before writing it. This is the
		// common case that no gap threshold can catch: a source-line break closes
		// the <g data-l> group mid-output-line, and JUSTIFICATION can shrink the
		// space between the two runs to less than a kern is wide — 2.5pt against a
		// 2.75pt threshold, in the paragraph that found this.
		return true
	}
	if baseline != c.baseline && x < c.endX {
		// The text went back to the left on a new baseline: a line ended, which is
		// a word boundary however the words wrapped. A baseline change that keeps
		// moving RIGHT is not a new line — it is a superscript or a subscript, and
		// a footnote marker belongs to the word it hangs off.
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
// which advances by width (points), set at size points.
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
// decision, not this one's.
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

// reset clears the run for the next stretch of glyphs.
func (t *textRun) reset() { t.words, t.space, t.size = nil, false, 0 }

// flush appends the run to the layer's current chunk as one <tspan> per word,
// then clears it.
func (t *textRun) flush(l *textLayer) {
	if t.empty() {
		t.reset()
		return
	}
	size := t.size
	if size <= 0 {
		size = 10
	}
	c := l.chunk()
	if l.cur.wantsSpace(t.words[0].x, t.baseline, size) && !endsWithHyphen(l.cur.lastRune) {
		c.body.WriteString(" ")
	}
	wrote := false
	for _, w := range t.words {
		if len(w.runes) == 0 {
			continue
		}
		if wrote {
			c.body.WriteString(" ")
		}
		wrote = true
		// textLength pins the word to the advance its glyphs actually occupy, so
		// the invisible text tracks the visible outlines whatever font the reader's
		// browser substitutes. A non-positive width (a zero-advance combining run)
		// is emitted unpinned rather than with a meaningless textLength.
		if w.width > 0 {
			fmt.Fprintf(&c.body, `<tspan x="%s" y="%s" font-size="%s" textLength="%s" lengthAdjust="spacingAndGlyphs">%s</tspan>`,
				f(w.x), f(t.baseline), f(size), f(w.width), escapeXMLText(string(w.runes)))
		} else {
			fmt.Fprintf(&c.body, `<tspan x="%s" y="%s" font-size="%s">%s</tspan>`,
				f(w.x), f(t.baseline), f(size), escapeXMLText(string(w.runes)))
		}
	}
	c.any = true
	t.advance(&l.cur)
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
		cur.owed = t.space
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

// addPhrase appends a stretch of text that is not a run of glyphs — a formula's
// LaTeX source over the formula it was typeset from — as one word occupying
// [x, x+width) on the baseline.
//
// It goes through the ordinary run machinery, so it inherits the boundary rules
// exactly: a space in front of it when one is owed or the gap warrants it, and
// a cursor left at its end for whatever follows.
func (l *textLayer) addPhrase(s string, x, width, baseline, size float64) {
	s = collapseSpace(s)
	if s == "" || width <= 0 {
		return
	}
	r := textRun{baseline: baseline, size: size}
	r.words = []textWord{{x: x, width: width, runes: []rune(s)}}
	r.flush(l)
}

// collapseSpace squeezes runs of whitespace to a single space, trims the ends,
// and drops the spaces the math tokeniser INSERTED rather than the ones the
// author typed — see [dropSyntheticSpaces].
func collapseSpace(s string) string {
	return dropSyntheticSpaces(squeezeSpace(s))
}

// dropSyntheticSpaces removes the space the math tokeniser writes after every
// control sequence, but ONLY where TeX did not need it: a control-sequence name
// ends at the first non-letter, so "\sum _{i=1}" is the tokeniser talking and
// "\alpha b" is not. The author's own spacing is untouched — the space in
// "E = mc^2" does not follow a control sequence, and removing it would make a
// search for what they typed fail.
func dropSyntheticSpaces(s string) string {
	var b strings.Builder
	r := []rune(s)
	for i := 0; i < len(r); i++ {
		if r[i] == ' ' && i+1 < len(r) && !isLetterRune(r[i+1]) && endsControlSequence(r[:i]) {
			continue
		}
		b.WriteRune(r[i])
	}
	return b.String()
}

// endsControlSequence reports whether r ends with a backslash followed by one or
// more letters — the shape whose trailing space the tokeniser supplies.
func endsControlSequence(r []rune) bool {
	i := len(r)
	for i > 0 && isLetterRune(r[i-1]) {
		i--
	}
	return i < len(r) && i > 0 && r[i-1] == '\\'
}

func isLetterRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// squeezeSpace collapses runs of whitespace to a single space and trims the ends.
func squeezeSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}
