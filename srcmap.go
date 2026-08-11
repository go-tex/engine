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
