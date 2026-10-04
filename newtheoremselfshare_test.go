// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ \newtheorem{X}[X]{…} asks X to share ITS OWN counter.
//
// latex.ltx's \@othm tests \@ifundefined{c@#2} and errors — "No theorem environment #2
// defined" — because that counter does not exist yet, so real LaTeX numbers nothing and
// carries on. Here the alias was built anyway:
//
//	\the<env>  :=  \the<shared>   which, when shared == env, is \the<env> again
//
// a self-referential macro the engine spun 400 times into the runaway guard. Two corpus
// papers write it, one directly and one through a wrapper of its own:
//
//	\newtheorem{corollary}[corollary]{Corollary}          2607.21390:  3 ->  11 pages
//	\newtheorem{#1vArIAblE}[#1vArIAblE]{#3}               2606.14675:  2 ->  37 pages
//
// Dropping the self-share leaves the environment a counter of its own, which is what the
// code already does when no [shared] is given.

func TestNewtheoremSharingItsOwnCounterDoesNotSelfReference(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+
		`\newtheorem{corollary}[corollary]{Corollary}`+
		`\message{[\meaning\thecorollary]}`+
		`\begin{document}`+
		`\begin{corollary}Body of the first.\end{corollary}`+
		`\section{S}Prose after it.`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if d := e.Diagnostics(); d.Runaway {
		t.Error("the runaway guard tripped: \\thecorollary was defined as itself")
	}
	// The self-reference reads back as a macro whose body is the same name again.
	if got := e.Diagnostics().Messages; strings.Contains(got, `->\thecorollary`) {
		t.Errorf("\\thecorollary is self-referential: %s", got)
	}
	if len(e.RenderPages(e.renderMargin(0))) == 0 {
		t.Fatal("no page produced")
	}
}

// And a counter it shares with ANOTHER environment must still delegate, or the fix would
// have cured the loop by breaking what [shared] is for. \theexample must print through
// \thetheorem, which carries the section (latex.ltx:12728, \@othm).
func TestNewtheoremSharingAnotherCounterStillDelegates(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+
		`\newtheorem{theorem}{Theorem}[section]`+
		`\newtheorem{example}[theorem]{Example}`+
		`\message{[\meaning\theexample]}`+
		`\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().Messages; !strings.Contains(got, `->\thetheorem`) {
		t.Errorf("\\theexample no longer delegates to \\thetheorem: %s", got)
	}
}
