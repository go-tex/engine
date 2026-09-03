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
)

const (
	InfPenalty  = linebreak.InfPenalty
	maxBadRatio = linebreak.MaxBadRatio
)

var (
	Box            = linebreak.Box
	Glue           = linebreak.Glue
	Glyph          = linebreak.Glyph
	Penalty        = linebreak.Penalty
	KnuthPlass     = linebreak.KnuthPlass
	KnuthPlassWith = linebreak.KnuthPlassWith
)

// hyphenator is the engine's name for the upstream Hyphenator.
type hyphenator = hyphenation.Hyphenator

func newHyphenator() *hyphenator { return hyphenation.New() }
