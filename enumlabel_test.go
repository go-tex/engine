// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⭐ AN ITEM IS NAMED BY ITS LABEL MACRO, NOT BY ITS COUNTER.
//
// \labelenumi is \theenumi. and \labelenumii is (\theenumii); \theenumN is the
// bare counter, and it is what \ref prints. Every call site handed \@listitem
// the counter, so the decoration — the period at level one, the parentheses at
// level two — was defined and never reached the page. Measured against
// tectonic, one document, whole page:
//
//	what is set          reference   before   after
//	label, level 1       1.          1        1.
//	label, level 2       (a)         a        (a)
//	\ref to level 1      1           1        1
//
// \itemize was already right (\@bulletitem takes \labelitemN), which is what
// made the asymmetry visible at all. go-tex/engine#845.
func TestAnEnumerateItemIsNamedByItsLabelMacro(t *testing.T) {
	e := runEI(t, `\documentclass{article}\begin{document}`+
		`\begin{enumerate}\item one\item two\end{enumerate}\end{document}`)
	got := mvlText(e.mvl)
	if !strings.Contains(got, "1.one") || !strings.Contains(got, "2.two") {
		t.Fatalf("typeset %q, want the items labelled \"1.\" and \"2.\" — "+
			"a bare counter means \\labelenumi was never expanded", got)
	}
}

// The second level's decoration is parentheses, not a period, which is why the
// label cannot be built by appending anything to the counter: article.cls:349
// is \newcommand\labelenumii{(\theenumii)}. A fix that hard-coded "." would
// pass the test above and still be wrong here.
func TestEachEnumerateLevelCarriesItsOwnDecoration(t *testing.T) {
	e := runEI(t, `\documentclass{article}\begin{document}`+
		`\begin{enumerate}\item one\begin{enumerate}\item deep\end{enumerate}`+
		`\end{enumerate}\end{document}`)
	got := mvlText(e.mvl)
	if !strings.Contains(got, "(a)deep") {
		t.Fatalf("typeset %q, want the nested item labelled \"(a)\" — "+
			"the period is level one's decoration, not every level's", got)
	}
}

// ⛔⛔ THE CONTROL THAT REFUSES THE OBVIOUS FIX. \@listitem's one argument used
// to serve BOTH \@currentlabel and the printed label, so the shortest way to
// put the period on the page is to hand \labelenumi to the old two-argument
// macro — and that puts the period inside every \ref as well.
//
// \refstepcounter composes \@currentlabel from \p@<ctr>\the<ctr>, never from
// the label (ltcounts.dtx), and the reference agrees: \ref to a level-one item
// is "1" in tectonic and was already "1" here. This assertion is what keeps a
// visible defect from being traded for a quieter one in every cross-reference.
func TestAReferenceToAnItemDoesNotCarryTheLabelsDecoration(t *testing.T) {
	e := runEI(t, `\documentclass{article}\begin{document}`+
		`\begin{enumerate}\item\label{one} one\end{enumerate}`+
		`seen[\ref{one}]\end{document}`)
	got := mvlText(e.mvl)
	if strings.Contains(got, "seen[1.]") {
		t.Fatalf("typeset %q: \\ref printed the LABEL. \\@currentlabel is "+
			"\\p@<ctr>\\the<ctr>, so a reference to item one is \"1\", not \"1.\"", got)
	}
	if !strings.Contains(got, "seen[1]") {
		t.Fatalf("typeset %q, want \"seen[1]\"", got)
	}
}

// enumitem's label= sets the LABEL, as its itemize branch always did for
// \labelitemN. It set \theenumN, which is the same confusion one layer down:
// the spec's closing parenthesis ended up inside every \ref, and once
// \labelenumN carried its own period the two stacked ("a)." for a spec of
// "a)"). Real enumitem keeps ref= for the counter.
func TestEnumitemLabelOverridesTheLabelNotTheCounter(t *testing.T) {
	e := runEI(t, `\begin{enumerate}[label=\alph*)]\item one\item two\end{enumerate}`)
	if got, want := mvlText(e.mvl), "a)oneb)two"; got != want {
		t.Fatalf("typeset %q, want %q — label= must REPLACE the label, not be "+
			"decorated by it", got, want)
	}
}

// A document with no \documentclass gets the kernel's own \labelenumN, or an
// item asking for its label by name finds nothing defined. Nineteen tests in
// this package write \begin{enumerate} with no class, and they are why the
// kernel carries the four rather than leaving them to a class.
func TestTheKernelCarriesTheEnumerateLabelsForAClasslessDocument(t *testing.T) {
	e := runEI(t, `\begin{enumerate}\item one\end{enumerate}`)
	if got := mvlText(e.mvl); !strings.Contains(got, "1.one") {
		t.Fatalf("typeset %q with no \\documentclass, want \"1.one\"", got)
	}
}
