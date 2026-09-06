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
// display skips are ADDED to the ordinary interline glue, not put in its place —
// that is what after_math does (tex.web:22602 and 22614-22615) — so a display
// advances the page by the skip plus the space any box of that height would get.
// A caller must have ended the current paragraph (endParagraph) before calling;
// an empty box list is a no-op.
func (e *Engine) placeDisplay(boxes []*boxNode) {
	if len(boxes) == 0 {
		return
	}
	// \abovedisplayskip goes above the display, and the display box then follows
	// through the ORDINARY interline path. TeX does not suppress that glue: after
	// appending the skip it calls append_to_vlist for the display box like any
	// other (tex.web:22602, "shift_amount(b):=s+d; append_to_vlist(b)"), and
	// append_to_vlist adds baselineskip-prev_depth-height(b) — or \lineskip when
	// that is too small (§679). prev_depth is never set to ignore_depth around a
	// display. Suppressing it here cost a whole \baselineskip per display:
	// measured against tectonic on one \[…\] between two lines of text, baseline
	// to baseline across the display, reference 43.95pt against our 32.02pt.
	e.mvl = append(e.mvl, glueNode{spec: e.namedSkip("abovedisplayskip")})
	// The rows of a multi-line display are set \jot FURTHER apart than ordinary
	// lines (3pt by default), which LaTeX does by adding \jot to \baselineskip for
	// the duration of the display. Without it the rows sat a plain \baselineskip
	// apart: measured against real LaTeX on an align of 1..4 rows, our cost rose
	// 13.6pt per row where the reference rises 16.5 — the difference is \jot to the
	// tenth of a point. It is ADDED to the interline glue, not put in its place.
	//
	// It applies BETWEEN the rows only. TeX packs the whole display into one box
	// and contributes that box with the OUTER \baselineskip (tex.web:22602); \jot
	// is what \displ@y's \openup adds inside it. So the glue above the first row
	// is the ordinary one.
	savedSkip := e.baselineskip
	first := true
	for _, b := range boxes {
		if b == nil {
			continue
		}
		if !first {
			e.baselineskip = savedSkip + e.jotSkip()
		}
		e.appendToPage(b)
		first = false
	}
	e.baselineskip = savedSkip
	// \belowdisplayskip goes below the display; the next paragraph's first line
	// then follows it through the ordinary interline path, prev_depth being the
	// depth of the last display box (tex.web:22614-22615, then resume_after_display
	// — again, nothing resets prev_depth).
	e.mvl = append(e.mvl, glueNode{spec: e.namedSkip("belowdisplayskip")})
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
