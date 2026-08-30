// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements the subcaption/subfig package's subfigure (and subtable)
// ENVIRONMENT form: \begin{subfigure}[pos]{width} … \caption{…} … \end{subfigure}.
// It is a sub-panel of a figure — a minipage of the given width holding the panel
// content (typically an \includegraphics) and its own \caption, which prints as the
// lettered sub-caption "(a)". Several panels set side by side make the multi-part
// figures that are pervasive on arXiv; without this the whole panel is an undefined
// environment and dropped, losing both the image and the vertical space it holds.
//
// It deliberately reuses doMinipage's building blocks (scanOptBracketPos,
// readBraceDimen, collectEnvBody, typesetGroupToVbox, alignParbox) rather than
// touching doMinipage, so the widely-used minipage path is unchanged. The only
// addition is a prefix, injected at the head of the collected body, that (1) makes
// \caption inside the panel a SUB-caption by setting \@captype to subfigure, and
// (2) points \linewidth/\columnwidth at the panel width so \includegraphics
// [width=\linewidth] fills the panel rather than the outer text block.
func (e *Engine) doSubfigure(captype string) {
	pos := e.scanOptBracketPos() // t / c / b (default c)
	width := e.readBraceDimen()
	body := e.collectEnvBody(captype)

	// A \begingroup…\endgroup around the panel keeps \@captype (and \linewidth/
	// \columnwidth) local — typesetGroupToVbox saves only the vertical-list state,
	// not the macro table, so an unscoped \def\@captype{subfigure} would leak and turn
	// the ENCLOSING figure's \caption into another "(c)" sub-caption.
	prefix := []tok{
		csTok("begingroup"), csTok("noindent"),
		csTok("def"), csTok("@captype"), chTok('{', catBegin),
	}
	for _, r := range captype {
		prefix = append(prefix, chTok(r, catLetter))
	}
	prefix = append(prefix,
		chTok('}', catEnd),
		csTok("linewidth"), csTok("hsize"),
		csTok("columnwidth"), csTok("hsize"),
	)
	content := append(prefix, body...)
	content = append(content, csTok("par"), csTok("endgroup"))

	savedHsize := e.hsize
	e.hsize = width
	vbox := e.typesetGroupToVbox(content)
	e.hsize = savedHsize

	vbox.width = width
	// Panels flow inline: enter horizontal mode so consecutive \begin{subfigure}…
	// \end{subfigure} sit side by side (as \subcaptionbox's do) rather than stacking
	// on the figure's vertical list.
	if !e.inPar {
		e.beginParagraph(false)
	}
	e.place(alignParbox(vbox, pos))
}
