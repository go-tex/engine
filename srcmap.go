// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements source-position tracking: the mouth records where in the
// input each token began, the engine keeps the "current source line" as it reads,
// and every glyph/box placed on a list is stamped with that line. That stamp is
// what turns a rendered page back into a source location — the foundation for
// precise error reporting (line:col of a failure) and for source ↔ output
// navigation (click a glyph → jump to its line, à la SyncTeX), which the SVG and
// PDF drivers surface downstream.
//
// Granularity is line-level and follows TeX/SyncTeX: text typed directly in the
// document maps exactly; text produced by a macro maps to the line where the macro
// was invoked (the last base-input position the mouth reached), which is what an
// editor's "jump to source" should land on.

import "fmt"

// buildLineStarts precomputes the rune offset at which each source line begins, so
// a token's offset resolves to (line, column) by binary search. Called by Run when
// the base input is (re)set.
func (e *Engine) buildLineStarts() {
	e.lineStarts = e.lineStarts[:0]
	e.lineStarts = append(e.lineStarts, 0)
	for i, r := range e.base {
		if r == '\n' {
			e.lineStarts = append(e.lineStarts, i+1)
		}
	}
	e.srcPos, e.curSrcLine, e.curSrcCol = 0, 1, 0
}

// setSrcPos records that the current token began at rune offset pos in the base
// input, updating the cached (line, column). Line is 1-based, column 0-based.
func (e *Engine) setSrcPos(pos int) {
	e.srcPos = pos
	e.curSrcLine, e.curSrcCol = e.lineColAt(pos)
}

// lineColAt converts a base-input rune offset into a 1-based line and 0-based
// column using the precomputed line-start table.
func (e *Engine) lineColAt(pos int) (line, col int) {
	// Binary search for the greatest line start ≤ pos.
	lo, hi := 0, len(e.lineStarts)
	for lo < hi {
		mid := (lo + hi) / 2
		if e.lineStarts[mid] <= pos {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return 1, pos
	}
	return lo, pos - e.lineStarts[lo-1]
}

// Position returns the 1-based line and 0-based column of the input the engine is
// currently reading — the location a diagnostic should point at.
func (e *Engine) Position() (line, col int) {
	return e.curSrcLine, e.curSrcCol
}

// SourceError is an engine error carrying the source location it occurred at, so a
// caller (a CLI, loom's compile panel) can point the user at the exact line.
type SourceError struct {
	Line, Col int // 1-based line, 0-based column (0/0 = unknown)
	Msg       string
}

func (s SourceError) Error() string {
	if s.Line > 0 {
		return fmt.Sprintf("texengine: %d:%d: %s", s.Line, s.Col+1, s.Msg)
	}
	return "texengine: " + s.Msg
}

// SourceSpan is one rendered glyph's bounding box on a page (points, SVG
// coordinates: origin top-left, Y is the box top) tagged with the source line it
// came from. It is the programmatic form of the SVG's data-l groups: a caller maps
// a click (x, y) → line, or a line → its output rectangles.
type SourceSpan struct {
	Line       int
	X, Y, W, H float64
}

// SourceSpans returns, for each page of the built document, the glyph spans that
// tie output back to source — the data behind click-to-source and jump-to-line.
// margin matches the value passed to the SVG/PDF renderers so coordinates align.
func (e *Engine) SourceSpans(margin float64) [][]SourceSpan {
	pages := e.Pages()
	out := make([][]SourceSpan, len(pages))
	for i, p := range pages {
		var spans []SourceSpan
		collectBoxSpans(&spans, p, margin, margin+spToPt(p.height))
		out[i] = spans
	}
	return out
}

// LineAt returns the source line of the last glyph span whose box contains (x, y),
// or 0 when the point is over no glyph. Last-wins so nested/overlapping content
// (a table cell over its row) resolves to the innermost glyph painted there.
func LineAt(spans []SourceSpan, x, y float64) int {
	line := 0
	for _, s := range spans {
		if x >= s.X && x <= s.X+s.W && y >= s.Y && y <= s.Y+s.H {
			line = s.Line
		}
	}
	return line
}

// RectsForLine returns every glyph span originating from the given source line —
// the boxes an editor highlights when the cursor sits on that line.
func RectsForLine(spans []SourceSpan, line int) []SourceSpan {
	var out []SourceSpan
	for _, s := range spans {
		if s.Line == line {
			out = append(out, s)
		}
	}
	return out
}

// collectBoxSpans mirrors paintBoxSP's geometry, recording a SourceSpan for every
// glyph instead of drawing it (see boxrender.go).
func collectBoxSpans(out *[]SourceSpan, b *boxNode, x, baseline float64) {
	if b.kind == hbox {
		collectHSpans(out, b, x, baseline)
	} else {
		collectVSpans(out, b, x, baseline-spToPt(b.height))
	}
}

func collectHSpans(out *[]SourceSpan, b *boxNode, x, baseline float64) {
	cx := x
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cx += spToPt(c.width)
		case glueNode:
			cx += spToPt(b.setWidth(c.spec))
		case charNode:
			w := spToPt(c.width)
			*out = append(*out, SourceSpan{
				Line: c.srcLine, X: cx, Y: baseline - spToPt(c.height),
				W: w, H: spToPt(c.height + c.depth),
			})
			cx += w
		case mathNode:
			cx += spToPt(c.width)
		case ruleNode:
			cx += spToPt(c.width)
		case *boxNode:
			collectBoxSpans(out, c, cx, baseline+spToPt(c.shift))
			cx += spToPt(c.width)
		}
	}
}

func collectVSpans(out *[]SourceSpan, b *boxNode, x, top float64) {
	cy := top
	for _, n := range b.list {
		switch c := n.(type) {
		case kernNode:
			cy += spToPt(c.width)
		case glueNode:
			cy += spToPt(b.setWidth(c.spec))
		case ruleNode:
			cy += spToPt(c.height + c.depth)
		case *boxNode:
			collectBoxSpans(out, c, x+spToPt(c.shift), cy+spToPt(c.height))
			cy += spToPt(c.height + c.depth)
		case mathNode:
			cy += spToPt(c.height + c.depth)
		}
	}
}

// firstSrcLine returns the source line of the first stamped node in a list (a glyph
// or an already-stamped box), or 0 when nothing carries one. Used to give a packed
// box the line of its leading content.
func firstSrcLine(list []node) int {
	for _, n := range list {
		switch c := n.(type) {
		case charNode:
			if c.srcLine > 0 {
				return c.srcLine
			}
		case *boxNode:
			if c.srcLine > 0 {
				return c.srcLine
			}
		}
	}
	return 0
}
