// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import _ "embed"

// The default text family is Latin Modern, which is what tectonic sets for a plain
// article and for fontspec without \setmainfont (the PDF's BaseFont reads
// LMRoman10-Regular). The engine used to set text in the maths face of go-tex/math
// (STIX Two Math), whose space is 2.00pt where Computer Modern's is 3.33pt; see the
// default text face report (#370).
//
// The four files are the unmodified OpenType files of the Latin Modern 2.004
// distribution, which tectonic's own bundle carries. Their licence is the GUST Font
// License v1.0, which grants the LaTeX Project Public License 1.3c's terms for
// distribution and modification; the files are embedded unchanged, so its request
// to rename derived works does not arise. The licence text is fonts/lm/GUST-FONT-LICENSE.txt
// and each font names it in its own copyright field ("released under the GUST Font
// License -- see http://tug.org/fonts/licenses/GUST-FONT-LICENSE.txt").

//go:embed fonts/lm/lmroman10-regular.otf
var lmRomanRegular []byte

//go:embed fonts/lm/lmroman10-bold.otf
var lmRomanBold []byte

//go:embed fonts/lm/lmroman10-italic.otf
var lmRomanItalic []byte

//go:embed fonts/lm/lmmono10-regular.otf
var lmMonoRegular []byte
