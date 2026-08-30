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
	// The name serves TWO packages. subcaption/subfig spell it as an ENVIRONMENT,
	// \begin{subfigure}[pos]{width}…\end{subfigure}; the older subfigure package
	// spells it as a COMMAND, \subfigure[caption]{content}, and 5 of the 200 papers
	// in the arXiv reference corpus still do (44 use the environment). Reading the
	// command form as an environment sends collectEnvBody looking for an
	// \end{subfigure} that is not there, and it swallows the rest of the document:
	// one paper fell from 18 pages to 4.
	//
	// \@currenvir is what tells them apart — \begin sets it (see setCurrentEnv), and
	// beamer's \frame is served the same way for the same reason.
	if !e.inEnvironment(captype) {
		e.doSubfigureCommand(captype)
		return
	}
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

// inEnvironment reports whether \@currenvir names env, i.e. whether the macro now
// running was reached through \begin{env} rather than called as a command.
func (e *Engine) inEnvironment(env string) bool {
	m := e.eq["@currenvir"]
	if m == nil || m.kind != mMacro {
		return false
	}
	var b []rune
	for _, t := range m.body {
		if t.cs_ {
			return false
		}
		b = append(b, t.ch)
	}
	return string(b) == env
}

// doSubfigureCommand implements the subfigure package's command form,
// \subfigure[caption]{content}: the panel is set where it stands, followed by its
// lettered caption. It is not boxed to a width the way the environment form is —
// the command states none — so consecutive panels flow as the figure's own
// \centering arranges them.
func (e *Engine) doSubfigureCommand(captype string) {
	capToks, hasCap := e.scanOptBracketToks()
	body := e.readBraceToks()
	if len(body) == 0 && !hasCap {
		return
	}
	ctr := "c@" + captype
	out := []tok{csTok("begingroup"),
		csTok("global"), csTok("advance"), csTok(ctr), chTok(' ', catSpace),
		chTok('b', catLetter), chTok('y', catLetter), chTok('1', catOther), csTok("relax"),
		csTok("edef"), csTok("@currentlabel"), chTok('{', catBegin),
		csTok("p@" + captype), csTok("the" + captype), chTok('}', catEnd),
	}
	out = append(out, body...)
	if hasCap {
		out = append(out, csTok("space"), chTok('(', catOther), csTok("the"+captype),
			chTok(')', catOther), csTok("space"))
		out = append(out, capToks...)
	}
	out = append(out, csTok("endgroup"))
	e.push(out)
}
