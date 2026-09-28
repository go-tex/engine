// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ LaTeX's one-token look-ahead was invisible to the maths layer. \@ifnextchar is a
// native primitive of the STOMACH, and the maths path is a separate string-level
// expander that never runs the gullet — so an \@ifnextchar carried into a formula by a
// macro body arrived at go-tex/math as a bare name and dropped the equation whole.
//
// Measured on 999 papers of the arXiv corpus: 290 equations over 6 papers, the
// fifth-largest trigger in the dropped-equation census (#466). The callers have nothing
// in common — a paper's own \widebar, springer's \@spnthm, a bra-ket \k@t, \@slashbox —
// so what is fixed here is the mechanism, not six documents.
//
// These tests observe WHICH BRANCH was taken, not merely that nothing dropped: each
// branch carries a distinct undefined command, and the census keys the drop on the
// first unknown name it meets. A test that only counted drops would pass for a resolver
// that always answered "no".

// The look-ahead matches, so the yes branch is expanded — and the peeked token is NOT
// consumed, which is the whole point of a look-ahead: \widebar{B}^H must still carry
// its superscript into the branch that was chosen because of it.
func TestIfNextCharTakesTheYesBranchWhenTheNextTokenMatches(t *testing.T) {
	d := mathDropRun(t, `\newcommand\wb[1]{\@ifnextchar^{\gotexYes{#1}}{\gotexNo{#1}}}$\wb{x}^2$`)
	if n := d[`\gotexYes`]; n != 1 {
		t.Errorf("the yes branch was not taken: %v", d)
	}
}

func TestIfNextCharTakesTheNoBranchWhenTheNextTokenDiffers(t *testing.T) {
	d := mathDropRun(t, `\newcommand\wb[1]{\@ifnextchar^{\gotexYes{#1}}{\gotexNo{#1}}}$\wb{x}+1$`)
	if n := d[`\gotexNo`]; n != 1 {
		t.Errorf("the no branch was not taken: %v", d)
	}
}

// The formula the mechanism was costing us. Both branches are renderable, so the claim
// is the one the census measures: the equation is not dropped at all.
func TestIfNextCharNoLongerDropsTheEquation(t *testing.T) {
	d := mathDropRun(t, `\newcommand\wb[1]{\@ifnextchar^{\overline{#1}}{\bar{#1}}}$\wb{x}^2 + \wb{y}$`)
	if len(d) != 0 {
		t.Errorf("equation dropped: %v", d)
	}
}

// ⛔ The comparison is \ifx, not string equality on the name. \let makes two DIFFERENT
// names one meaning, and TeX's look-ahead sees the meaning — which is why the kernel can
// write \let\@sptoken= and then ask \ifx whether what it peeked was a space. A resolver
// comparing names would answer "no" here and silently typeset the other half.
//
// Both names are DEFINED on purpose: two undefined control sequences are \ifx-equal to
// each other, so a test built on undefined names would pass for a resolver that only
// ever compared nil against nil.
func TestIfNextCharComparesMeaningsAndNotNames(t *testing.T) {
	d := mathDropRun(t, `\let\gotexPeek\relax`+
		`\def\wb{\@ifnextchar\gotexPeek{\gotexYes}{\gotexNo}}$\wb\relax$`)
	if n := d[`\gotexYes`]; n != 1 {
		t.Errorf("\\let alias did not compare equal to its target: %v", d)
	}
	// The control: a different meaning under the same shape must answer no, or the
	// test above would also pass for a resolver that always says yes.
	d = mathDropRun(t, `\let\gotexPeek\relax`+
		`\def\wb{\@ifnextchar\gotexPeek{\gotexYes}{\gotexNo}}$\wb\hbox$`)
	if n := d[`\gotexNo`]; n != 1 {
		t.Errorf("a different meaning compared equal: %v", d)
	}
}

// \kernel@ifnextchar is \let to \@ifnextchar precisely so a package that redefines the
// public name cannot break the kernel (latex.ltx:1607), and \@testopt — every
// \newcommand-with-a-default — goes through that private name. Dispatching on the
// primitive's identity rather than on the reported name is what serves it.
func TestIfNextCharIsServedUnderItsKernelAlias(t *testing.T) {
	d := mathDropRun(t, `\def\wb{\kernel@ifnextchar^{\gotexYes}{\gotexNo}}$\wb^2$`)
	if n := d[`\gotexYes`]; n != 1 {
		t.Errorf("\\kernel@ifnextchar was not served: %v", d)
	}
}

// \@xifnch loops while the peeked token is a space, so the spaces before the look-ahead
// are CONSUMED and the token behind them is what decides.
func TestIfNextCharSkipsSpacesBeforeTheLookahead(t *testing.T) {
	d := mathDropRun(t, `\newcommand\wb[1]{\@ifnextchar^{\gotexYes{#1}}{\gotexNo{#1}}}$\wb{x}   ^2$`)
	if n := d[`\gotexYes`]; n != 1 {
		t.Errorf("spaces before the look-ahead were not skipped: %v", d)
	}
}

// At the end of a formula there is no token left in the source, and what TeX would
// actually peek is the math shift that closed it — never the [ or ^ a look-ahead asks
// about. Answering no there is TeX's answer, not a fallback.
func TestIfNextCharAnswersNoAtTheEndOfTheFormula(t *testing.T) {
	d := mathDropRun(t, `\newcommand\wb[1]{\@ifnextchar^{\gotexYes{#1}}{\gotexNo{#1}}}$\wb{x}$`)
	if n := d[`\gotexNo`]; n != 1 {
		t.Errorf("the end of the formula did not answer no: %v", d)
	}
}

// ⛔ A look-ahead that cannot be READ is left standing, so the census reports it. The
// witness has to distinguish refusal from success: this input supplies only two of the
// three arguments, so a resolver that read them anyway would find \gotexYes and drop
// under THAT name instead — a different observation, not a quieter one.
func TestIfNextCharLeavesAnIncompleteLookaheadStanding(t *testing.T) {
	d := mathDropRun(t, `\def\wb{\@ifnextchar^{\gotexYes}}$\wb$`)
	if n := d[`\@ifnextchar`]; n != 1 {
		t.Errorf("an incomplete look-ahead was not left standing: %v", d)
	}
}

// findMathCS must stop at a control-WORD boundary, or a search for \@ifnextchar would
// also fire on the start of a longer name — and rewrite a macro the document defined.
func TestFindMathCSStopsAtAControlWordBoundary(t *testing.T) {
	if i := findMathCS(`\@ifnextcharX `, "@ifnextchar"); i != -1 {
		t.Errorf("matched inside a longer name at %d", i)
	}
	if i := findMathCS(`\@ifnextchar^`, "@ifnextchar"); i != 0 {
		t.Errorf("did not match before a control symbol: %d", i)
	}
	// @ continues a name here (the kernel makes it a letter while a class is read),
	// which isMathLetter — which answers about TeX's letters — does not say.
	if i := findMathCS(`\@ifnextchar@ `, "@ifnextchar"); i != -1 {
		t.Errorf("matched before an @, which continues the name: %d", i)
	}
}
