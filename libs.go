// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"github.com/go-typeset/hyphenation"
	"github.com/go-typeset/linebreak"
)

// Two pieces of this engine are algorithms in their own right, useful to anyone
// laying out text and not only to a TeX engine, so they now live in their own
// repositories and this file is the seam:
//
//   - github.com/go-typeset/linebreak — Knuth–Plass optimal line breaking: the
//     box/glue/penalty model and the paragraph builder.
//   - github.com/go-typeset/hyphenation — Liang's algorithm: where a hyphen may fall
//     in a word, reading TeX's own pattern files.
//
// The aliases below keep the engine's own code reading as it did (Item, Line,
// KnuthPlass, …) while the definitions live upstream. What stays HERE is what is
// genuinely TeX's rather than the algorithm's: the \patterns primitive that loads
// a pattern file, and the walk that turns a node list into discretionary breaks.
type (
	Item = linebreak.Item
	Line = linebreak.Line
	// LineBreakParams carries TeX's demerit parameters to the optimiser.
	LineBreakParams = linebreak.Params
	// Measure is the per-line width source a shaped paragraph is broken on.
	Measure = linebreak.Measure
	// Shape is one \parshape pair: a left margin and a width.
	Shape = linebreak.Shape
	// LineBackground is \leftskip + \rightskip as the breaker sees them: the glue
	// every line carries before its own material (tex.web §16307-16315).
	LineBackground = linebreak.Background
)

const (
	InfPenalty  = linebreak.InfPenalty
	maxBadRatio = linebreak.MaxBadRatio
)

var (
	Box            = linebreak.Box
	Glue           = linebreak.Glue
	GlueOrder      = linebreak.GlueOrder
	Glyph          = linebreak.Glyph
	Penalty        = linebreak.Penalty
	KnuthPlass     = linebreak.KnuthPlass
	KnuthPlassWith = linebreak.KnuthPlassWith

	// The SHAPED breaker, for a paragraph whose lines are not all the same
	// width: \hangindent with \hangafter, and \parshape when this engine grows
	// the primitive. tex.web §847-850 takes one measure per line, and the width
	// is an input to every candidate break's badness — so a shape is a
	// line-breaking parameter and not a placement one.
	KnuthPlassShaped = linebreak.KnuthPlassShaped
	Uniform          = linebreak.Uniform
	HangIndent       = linebreak.HangIndent
	ParShape         = linebreak.ParShape
)

// glueOrder maps this engine's glue order onto the library's. They are the same
// numbers — 0 points, 1 fil, 2 fill, 3 filll — and the clamp is here so that a
// spec built from a document's own \hskip cannot index past the library's four
// columns.
func glueOrder(order int) uint8 {
	switch {
	case order <= 0:
		return linebreak.OrderPt
	case order >= int(linebreak.OrderFilll):
		return linebreak.OrderFilll
	default:
		return uint8(order)
	}
}

// hyphenator is the engine's name for the upstream Hyphenator.
type hyphenator = hyphenation.Hyphenator

func newHyphenator() *hyphenator { return hyphenation.New() }
