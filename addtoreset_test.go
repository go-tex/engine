// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Counter resets in report/book. Both defects below were measured against the
// real embedded report.cls, not against a reconstruction of it.

// report.cls ties the equation counter to chapter with \@addtoreset, NOT with
// \newcounter{equation}[chapter] — the counter already exists, so there is
// nothing to declare:
//
//	\@addtoreset {equation}{chapter}
//	\renewcommand\theequation
//	  {\ifnum \c@chapter>\z@ \thechapter.\fi \@arabic\c@equation}
//
// With \@addtoreset undefined that line did nothing, so the chapter part of an
// equation number advanced while the equation part kept counting: the second
// chapter's first equation printed (2.2).
func TestAddtoresetTiesAnExistingCounterToAParent(t *testing.T) {
	got := runMsg(t, `\newcounter{parent}\newcounter{child}`+
		`\@addtoreset{child}{parent}`+
		`\stepcounter{child}\stepcounter{child}\message{\arabic{child}}`+
		`\stepcounter{parent}\message{\arabic{child}}`)
	if got != "2 0" {
		t.Errorf("child around a parent step = %q, want \"2 0\" — the parent's step must zero it", got)
	}
}

// A reset CASCADES. LaTeX zeroes a counter by stepping it out of −1
// (\@stpelt), so zeroing one counter also runs ITS reset list. A flat
// assignment stops at the first level.
//
// With child within parent and grandchild within child, stepping the parent
// must clear both. Before this, the grandchild survived: \thesubsection after a
// second \chapter read 2.0.2 where LaTeX gives 2.0.0.
func TestAResetCascadesDownTheWholeChain(t *testing.T) {
	got := runMsg(t, `\newcounter{parent}\newcounter{child}[parent]\newcounter{grandchild}[child]`+
		`\stepcounter{grandchild}\stepcounter{grandchild}\message{\arabic{grandchild}}`+
		`\stepcounter{parent}\message{\arabic{grandchild}}`)
	if got != "2 0" {
		t.Errorf("grandchild around a parent step = %q, want \"2 0\" — a reset must cascade, not stop one level down", got)
	}
}

// \@stpelt is the kernel's own reset step and is usable on its own: it zeroes
// the counter AND runs its list.
func TestStpeltZeroesAndCascades(t *testing.T) {
	got := runMsg(t, `\newcounter{a}\newcounter{b}[a]`+
		`\setcounter{a}{7}\setcounter{b}{9}`+
		`\@stpelt{a}\message{\arabic{a}}\message{\arabic{b}}`)
	if got != "0 0" {
		t.Errorf("\\@stpelt{a} left (a,b) = %q, want \"0 0\"", got)
	}
}

// Neither command may explode on input a class file could plausibly contain.
func TestResetCommandsIgnoreWhatTheyCannotAct(t *testing.T) {
	// An unknown counter: nothing to reset, and nothing to complain about.
	got := runMsg(t, `\newcounter{p}\@addtoreset{nosuch}{p}\stepcounter{p}\message{ok}`)
	if got != "ok" {
		t.Errorf("\\@addtoreset on an unknown counter = %q, want \"ok\"", got)
	}
	// Empty names, and \@stpelt on a counter that was never declared.
	got = runMsg(t, `\@addtoreset{}{}\@stpelt{}\@stpelt{nosuch}\message{ok}`)
	if got != "ok" {
		t.Errorf("empty/unknown reset arguments = %q, want \"ok\"", got)
	}
}

// The end-to-end shape, through the real class: a second chapter restarts the
// section, the equation and every within-chapter counter.
func TestReportRestartsItsCountersEachChapter(t *testing.T) {
	got := runMsg(t, `\documentclass{report}\begin{document}`+
		`\chapter{One}\section{A}\subsection{a}`+
		`\message{[\thesection][\thesubsection]}`+
		`\chapter{Two}`+
		`\message{[\thechapter][\thesubsection]}`+
		`\section{B}\message{[\thesection]}`+
		`\end{document}`)
	// report.cls announces each chapter on the terminal (\typeout{\@chapapp\space
	// \thechapter.}), so those lines share the message stream with ours.
	const want = "Chapter 1. [1.1][1.1.1] Chapter 2. [2][2.0.0] [2.1]"
	if got != want {
		t.Errorf("report numbering = %q, want %q", got, want)
	}
}
