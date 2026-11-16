// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \addvspace — "leave at least this much space here", which is a MAXIMUM and not a sum.
//
// ⛔⛔ IT WAS `\def\addvspace#1{\vskip#1}`, a plain skip, so two of them in a row ADDED
// where LaTeX takes the larger. Every place two pieces of vertical furniture meet —
// the end of one list and the start of the next, a \section after a list, a display
// after an environment — got the sum of two skips instead of the bigger one, and the
// document grew. The engine's signed page bias against tectonic is +35 over 154 arXiv
// papers (go-tex/engine#622), in exactly that direction.
//
// Seen in the reference's own node list around `\begin{itemize}` (tectonic, \showlists):
//
//	\penalty -51
//	\glue 8.0 plus 3.0 minus 4.0
//	\glue -8.0 plus -3.0 minus -4.0     ← the previous skip, backed out
//	\glue 4.0 plus 1.0 minus 3.0
//
// That negative glue is \addvspace removing what was there before putting back the
// larger value; a `\vskip` implementation has no such line, and the pair simply stacks.
//
// latex.ltx:6513-6526 and 6527-6539:
//
//	\def\@xaddvskip{%
//	  \ifdim\lastskip<\@tempskipb
//	    \vskip-\lastskip \vskip\@tempskipb
//	  \else
//	    \ifdim\@tempskipb<\z@
//	      \ifdim\lastskip<\z@ \else
//	        \advance\@tempskipb\lastskip \vskip-\lastskip \vskip\@tempskipb
//	      \fi\fi\fi}
//	\def\addvspace#1{\ifvmode \if@minipage\else
//	    \ifdim\lastskip=\z@ \vskip#1 \else \setlength\@tempskipb{#1}\@xaddvskip \fi
//	  \fi\else\@noitemerr\fi}
//
// ⛔ THE NEGATIVE BRANCH IS NOT SYMMETRIC, and dropping it as an edge case would be
// wrong in the direction that loses space: when the NEW skip is negative and the last
// one is not, LaTeX replaces the last by (new + last) — it lets a negative \addvspace
// claw back space that is already there, which is how \addvspace{-\parskip} works in
// the list code (latex.ltx:11564). When both are negative it does nothing at all.

// addVSpaceToPage applies \addvspace's maximum rule to the vertical list.
//
// ⛔ IT COMPARES NATURAL WIDTHS, as \ifdim\lastskip<\@tempskipb does: \lastskip in a
// dimension test yields the glue's natural size, not its stretch or shrink. The
// replacement then carries the WHOLE new spec, stretch and shrink included, because
// \vskip\@tempskipb does.
func (e *Engine) addVSpaceToPage(g glueSpec) {
	last, ok := e.lastGlueOnPage()
	if !ok || last.width == 0 {
		e.mvl = append(e.mvl, glueNode{spec: g})
		return
	}
	switch {
	case last.width < g.width:
		// The new one is bigger: back the old one out and put the new one in.
		e.replaceLastGlue(g)
	case g.width < 0 && last.width >= 0:
		// A negative \addvspace against a non-negative last skip: the reference keeps
		// their SUM, which is how \addvspace{-\parskip} removes space already placed.
		sum := g
		sum.width += last.width
		sum.stretch += last.stretch
		sum.shrink += last.shrink
		e.replaceLastGlue(sum)
	}
	// Otherwise the skip already there is the larger: nothing is added at all.
}

// lastGlueOnPage reports the glue that ends the main vertical list, if it ends with
// glue. It is \lastskip, which is zero when the list ends with anything else.
func (e *Engine) lastGlueOnPage() (glueSpec, bool) {
	if n := len(e.mvl); n > 0 {
		if g, ok := e.mvl[n-1].(glueNode); ok {
			return g.spec, true
		}
	}
	return glueSpec{}, false
}

// replaceLastGlue swaps the glue at the end of the vertical list for another.
//
// ⛔ IT REPLACES RATHER THAN EMITTING A NEGATIVE SKIP. The reference writes
// `\vskip-\lastskip \vskip\@tempskipb` because TeX can only append; the pair leaves a
// node list carrying both, which is what the \showlists dump above shows. This engine's
// list is addressable, so the same state is reached by overwriting — and the two differ
// only in that ours has one node where TeX has three, with identical total height.
func (e *Engine) replaceLastGlue(g glueSpec) {
	if n := len(e.mvl); n > 0 {
		if _, ok := e.mvl[n-1].(glueNode); ok {
			e.mvl[n-1] = glueNode{spec: g}
			return
		}
	}
	e.mvl = append(e.mvl, glueNode{spec: g})
}
