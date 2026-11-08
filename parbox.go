// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements \parbox[pos]{width}{content}: it typesets content as a
// paragraph broken to the given width and returns it as a vbox placed inline, so
// several \parboxes sit side by side. The optional [pos] sets the box's vertical
// reference point — t (top line's baseline), b (bottom line's baseline) or c (the
// default, vertically centred). minipage (the environment form) is future work.

// doParbox implements \parbox[pos]{width}{content}. The content is run through the
// paragraph builder at the requested \hsize (saved/restored around the build) and
// packed into a vbox whose width is fixed to the requested measure.
func (e *Engine) doParbox() *boxNode {
	// latex.ltx:\@iiiparbox opens with \leavevmode, so a \parbox met in VERTICAL
	// mode starts a paragraph and the next one joins it on the same line. Without
	// it each box became its own paragraph and two panels stacked (#398).
	e.leaveVMode()
	pos := e.scanOptBracketVPos() // t / c / b (default c)
	width := e.readBraceDimen()
	// \noindent prefix: a parbox's paragraph has no \parindent box (which would
	// overflow a narrow box and defeat line breaking).
	content := append([]tok{csTok("noindent")}, e.grabUndelimited()...)

	savedHsize := e.hsize
	e.hsize = width
	body := e.typesetGroupToVbox(content) // breaks the paragraph to e.hsize == width
	e.hsize = savedHsize

	body.width = width
	return alignParbox(body, pos, e.axisHeight())
}

// axisHeight is TeX's math axis, the line \vcenter centres a box on. tex.web
// §1200 centres by axis_height(cur_size), which is \fontdimen22 of the symbol
// family; in Computer Modern that is a QUARTER of the design size. Measured
// against tectonic on a 40pt panel: at 10pt the box comes back 22.5pt/17.5pt, at
// 12pt 23.0pt/17.0pt, at 24.88pt 26.22pt/13.78pt — 2.5, 3.0 and 6.22, exactly a
// quarter of each size.
// ⛔ THE QUARTER IS COMPUTER MODERN'S NUMBER, NOT EVERY FONT'S. An OpenType font
// states its own axis in the MATH table (axisHeight, MathConstants), and
// STIXTwoMath — this engine's own built-in face — says 25.8% of the em, not 25%:
//
//	10pt      axis 2.5800   quarter 2.5000   +0.0800pt
//	12pt      axis 3.0960   quarter 3.0000   +0.0960pt
//	24.88pt   axis 6.4190   quarter 6.2200   +0.1990pt
//
// It is a ratio, so it grows with the size, and it is on every \vcenter and every
// [c]-aligned parbox of every document set in that face.
//
// ⛔ The fallback is not a guess and it is why the tectonic figures above still
// hold: a TEXT font carries no MATH table at all — measured on lmroman10,
// LibertinusSerif and TeXGyreTermesX, none of them has one — so the quarter
// remains the only answer available for them, and their renders are unchanged.
// The validation this comment used to rest on was made with Computer Modern,
// where \fontdimen22 IS a quarter; it was right about CM and silent about the
// rest.
func (e *Engine) axisHeight() int {
	if e.curFont == nil {
		return 0
	}
	if f, ok := e.curFont.(mathAxisFont); ok {
		if axis, ok := f.mathAxisSP(); ok {
			return axis
		}
	}
	return ptToSP(e.curFont.sizePt() / 4)
}

// mathAxisFont is a face that can state its own math axis.
//
// ⛔ An OPTIONAL interface here, and a required method for hasGlyphFor, which is
// a distinction worth stating rather than a mood: every face must be able to say
// whether it can set a glyph, so a silent fallback there would hide a wrong
// answer. A math axis is a capability a face genuinely lacks — no text font has a
// MATH table — and for exactly those faces the quarter is the measured-correct
// answer. The precedent is embeddableFont (font.go), which the PDF driver asks
// the same way.
type mathAxisFont interface {
	mathAxisSP() (int, bool)
}

// alignParbox re-anchors a parbox's vertical reference point per [pos]. vpack
// leaves the reference at the last line's baseline (that is [b]); [t] moves it to
// the first line's baseline, [c] centres the box on the MATH AXIS.
func alignParbox(body *boxNode, pos byte, axis int) *boxNode {
	total := body.height + body.depth
	switch pos {
	case 't':
		firstH := 0
		for _, n := range body.list {
			if lb, ok := n.(*boxNode); ok {
				firstH = lb.height
				break
			}
		}
		body.height = firstH
		body.depth = total - firstH
	case 'b':
		// leave as packed: reference at the last line's baseline
	default: // 'c'
		// latex.ltx's \@iiiparbox sets [c] with `\@pboxswtrue $\vcenter`, so the
		// box is centred on the math axis and not on the baseline: half of it sits
		// ABOVE the axis, which is itself above the baseline. Centring on the
		// baseline instead split a 40pt panel 20/20 where TeX gives 22.5/17.5 —
		// the same total, the wrong reference point, and every side-by-side panel
		// sat 2.5pt low against its neighbours' text.
		body.height = total/2 + axis
		body.depth = total - body.height
	}
	return body
}
