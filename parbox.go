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
func (e *Engine) axisHeight() int {
	if e.curFont == nil {
		return 0
	}
	return e.curFont.sizePt() * unity / 4
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
