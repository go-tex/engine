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
	pos := e.scanOptBracketPos() // t / c / b (default c)
	width := e.readBraceDimen()
	// \noindent prefix: a parbox's paragraph has no \parindent box (which would
	// overflow a narrow box and defeat line breaking).
	content := append([]tok{csTok("noindent")}, e.grabUndelimited()...)

	savedHsize := e.hsize
	e.hsize = width
	body := e.typesetGroupToVbox(content) // breaks the paragraph to e.hsize == width
	e.hsize = savedHsize

	body.width = width
	return alignParbox(body, pos)
}

// alignParbox re-anchors a parbox's vertical reference point per [pos]. vpack
// leaves the reference at the last line's baseline (that is [b]); [t] moves it to
// the first line's baseline, [c] centres the box on its own height.
func alignParbox(body *boxNode, pos byte) *boxNode {
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
	default: // 'c' — centre the box vertically about its reference point
		body.height = total / 2
		body.depth = total - body.height
	}
	return body
}
