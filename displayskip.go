// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file gives a displayed equation the vertical glue TeX puts around it.
//
// A display ($$…$$, \[…\], equation, align, gather, multline) is not spaced like
// an ordinary line: TeX removes the normal interline (baselineskip) glue above
// and below it and substitutes \abovedisplayskip before the display and
// \belowdisplayskip after it (10pt plus 2pt minus 5pt each at 10pt, set by
// size10.clo). The engine already loads those registers with the right values —
// \the\abovedisplayskip reads 10pt plus 2pt minus 5pt, byte-for-byte what a real
// LaTeX reports — but the layout code contributed the display box through the
// ordinary interline path, so every display was set with a ~baselineskip gap
// (≈2–4pt after the box's own height is subtracted) instead of a full 10pt above
// and below. A display therefore saved on the order of ten points of vertical
// space, and a math-heavy article — hundreds of displays — packed several extra
// pages of them, under-paginating against tectonic. placeDisplay restores the two
// skips so a display advances the page the way TeX does.

// namedSkip returns the glue held in the \skip register a \newskip-defined control
// sequence names (\abovedisplayskip, \belowdisplayskip, \topsep, …). A name that
// is not a skip register — undefined, or bound to something else — reads back as
// the zero glue, so a caller gets a harmless 0pt rather than a wrong value.
func (e *Engine) namedSkip(name string) glueSpec {
	if m := e.eq[name]; m != nil && m.kind == mSkipRef && m.code >= 0 && m.code < len(e.skip) {
		return e.skip[m.code]
	}
	return glueSpec{}
}

// placeDisplay contributes the boxes of one displayed equation to the main
// vertical list with TeX's display spacing: \abovedisplayskip above the first
// box, ordinary interline glue between the boxes of a multi-line display
// (align/gather/multline rows), and \belowdisplayskip below the last box. The
// interline glue that would otherwise sit above the first box and below the last
// is suppressed (prevDepth set to ignoreDepth at both ends) so the display skips
// are the whole gap, as in TeX's after_math. A caller must have ended the current
// paragraph (endParagraph) before calling; an empty box list is a no-op.
func (e *Engine) placeDisplay(boxes []*boxNode) {
	if len(boxes) == 0 {
		return
	}
	// \abovedisplayskip replaces the interline glue above the display.
	e.mvl = append(e.mvl, glueNode{spec: e.namedSkip("abovedisplayskip")})
	e.prevDepth = ignoreDepth // no interline glue on top of \abovedisplayskip
	// The rows of a multi-line display are set \jot FURTHER apart than ordinary
	// lines (3pt by default), which LaTeX does by adding \jot to \baselineskip for
	// the duration of the display. Without it the rows sat a plain \baselineskip
	// apart: measured against real LaTeX on an align of 1..4 rows, our cost rose
	// 13.6pt per row where the reference rises 16.5 — the difference is \jot to the
	// tenth of a point. It is ADDED to the interline glue, not put in its place.
	savedSkip := e.baselineskip
	e.baselineskip += e.jotSkip()
	for _, b := range boxes {
		if b != nil {
			e.appendToPage(b) // first box: no glue (prevDepth ignored); rows: interline glue
		}
	}
	e.baselineskip = savedSkip
	// \belowdisplayskip replaces the interline glue below the display; the next
	// paragraph's first line then follows it with no additional interline glue.
	e.mvl = append(e.mvl, glueNode{spec: e.namedSkip("belowdisplayskip")})
	e.prevDepth = ignoreDepth
	// The text after the display resumes the same paragraph in TeX, so the fresh
	// paragraph this engine starts for it must not carry a \parskip (see
	// beginParagraph). An explicit \par before that text clears the flag.
	e.suppressParskip = true
}

// jotSkip is \jot, the extra leading between the rows of a multi-line display.
// latex.ltx sets it to 3pt (l.11173) and the kernel substrate now does the same,
// so a zero reading means a document asked for zero and is honoured.
func (e *Engine) jotSkip() int {
	if v, ok := e.namedDimen("jot"); ok {
		return v
	}
	return 3 * unity
}
