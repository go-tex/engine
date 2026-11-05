// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// \addpenalty — LaTeX's "offer a breakpoint here, even if glue is already in the way".
//
// ⛔⛔ IT WAS A NO-OP STUB, `\def\addpenalty#1{}`, so every page-break hint the kernel
// and the classes give was silently discarded. The three a list contributes
// (latex.ltx:11523, :11562, :11567) are \@beginparpenalty, \@itempenalty and
// \@endparpenalty, all NEGATIVE — they ENCOURAGE a break at a list boundary. Without
// them the page builder has no reason to prefer those spots, carries a list it could
// have split, and spends a page it did not need: 40 itemize blocks cost 7 pages
// against the reference's 6, with every height agreeing to a tenth of a point
// (go-tex/engine#685). Sectioning and \@endparenv call it too, as do all four base
// classes and amsart.
//
// ⭐ WHY THIS DOES NOT PORT THE REFERENCE'S GLUE DANCE. latex.ltx:6549-6563 lifts the
// trailing glue, drops the penalty, and puts the glue back:
//
//	\@tempskipb\lastskip
//	\vskip -\@tempskipb  \penalty#1
//	\ifdim\@tempskipa=\@tempskipb\else \advance\@tempskipb -\@tempskipa \vskip\@tempskipb \fi
//	\vskip \@tempskipa
//
// The three skips sum to ZERO. That contortion exists because TeX can only APPEND to a
// vertical list, and a penalty appended after glue is not a legal breakpoint — so the
// only way to get it BEFORE the glue is to remove the glue and re-add it, with a
// \prevdepth correction to keep the arithmetic exact. This engine's vertical list is a
// slice, so inserting reaches the same state directly.
//
// ⛔ AND THAT IS THE SAFER FORM, on the kernel's own evidence. ltspace.dtx:999-1003
// records that the dance had to be fixed twice: "Plamen Tanovski discovered that this
// fix wasn't good enough either as the \vskip kept getting bigger if several
// \addpenalty commands followed each other." Consecutive calls GREW the skip. An
// insertion never adds or removes glue, so it cannot have that bug — which is the
// reason to prefer it, not merely its convenience.
//
// ⚠ The guards stay in TeX (classkernel.go), where the reference puts them:
//
//	\def\addpenalty#1{\ifvmode\if@minipage\else\if@nobreak\else
//	  \gotex@addpenalty{#1}\fi\fi\fi}
//
// ⚠ \ifvmode is the 2015/01/01 kernel's guard (ltspace.dtx:1057-1060), and it is the
// one the tectonic bundle's latex.ltx carries — so it is the one this engine's oracle
// runs. Current LaTeX (2024/11/01) instead writes
// `\ifhmode \ifinner \@LRmoderr \else \par \fi \fi`, ending the paragraph in outer
// horizontal mode rather than doing nothing. Matching the oracle is deliberate here,
// not an oversight.

// addPenaltyToPage places a penalty where LaTeX's \addpenalty would: before the glue
// that already ends the vertical list, so the breakpoint is in front of the space
// rather than behind it, where TeX would never take it.
//
// ⛔ A PENALTY AFTER GLUE IS NOT A BREAKPOINT. TeX breaks at glue only when the item
// before it is not discardable, and at a penalty wherever it stands; a penalty sitting
// after the glue offers a break at a place the page builder has already passed. That is
// the whole reason the reference goes to such lengths, and appending here would
// reproduce the stub's silence with extra steps.
func (e *Engine) addPenaltyToPage(p int) {
	n := len(e.mvl)
	if n == 0 {
		e.mvl = append(e.mvl, penaltyNode{penalty: p})
		return
	}
	// Walk back over the run of trailing glue, so the penalty lands before ALL of it:
	// \addvspace and \parskip often leave two or three glue nodes in a row, and a
	// breakpoint buried between them is as unreachable as one behind them.
	i := n
	for i > 0 {
		if _, ok := e.mvl[i-1].(glueNode); !ok {
			break
		}
		i--
	}
	if i == n {
		e.mvl = append(e.mvl, penaltyNode{penalty: p})
		return
	}
	e.mvl = append(e.mvl, nil)
	copy(e.mvl[i+1:], e.mvl[i:])
	e.mvl[i] = penaltyNode{penalty: p}
}
