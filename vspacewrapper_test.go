// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ \vspace must be a MACRO, because the standard wrapper idiom expands it one step.
//
// lineno.sty installs its hooks like this (lineno.sty:3029-3035):
//
//	\def\@tempa#1#2{\expandafter\def\expandafter#2\expandafter{\expandafter
//	  \ifLineNumbers\expandafter#1\expandafter\fi#2}}
//	\@tempa\@LN@changevadjust\vspace
//
// The \expandafter chain expands #2 ONE step while the new body is being formed, so
// the body holds the command's old MEANING instead of its name. A primitive cannot be
// expanded, so the body kept the token \vspace and the wrapper called itself: 400
// turns into the runaway guard. Three corpus papers of class aa with lineno —
// 2607.18707, 2607.28723, 2607.29488 — each stopped at ONE page and came out at 21,
// 15 and 15 once \vspace was a macro in front of the primitive, as LaTeX has it
// (\def\vspace{\@ifstar\@vspacer\@vspace}).
//
// \pagebreak and \nopagebreak, which lineno wraps identically, were already macros
// here — which is why only \vspace looped, and why the witness below is about the
// SHAPE of the definition and not about vertical space.

const vspaceWrapperSrc = `\documentclass{article}\makeatletter` +
	`\newif\ifLineNumbers \LineNumberstrue` +
	`\def\zzhook{}` +
	`\def\@tempa#1#2{\expandafter\def\expandafter#2\expandafter{\expandafter` +
	`\ifLineNumbers\expandafter#1\expandafter\fi#2}}` +
	`\@tempa\zzhook\vspace` +
	`\begin{document}A\par\vspace{10pt}\par B\end{document}`

func TestWrappingVspaceTheLinenoWayDoesNotRecurse(t *testing.T) {
	e, err := compile([]byte(vspaceWrapperSrc), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.Diagnostics().Runaway {
		t.Error("the runaway guard tripped: the wrapper's body holds \\vspace itself")
	}
	if n := len(e.RenderPages(e.renderMargin(0))); n == 0 {
		t.Fatal("no page produced")
	}
}

// And the wrapped \vspace must still SPACE: a macro in front of the primitive that
// swallowed its argument would pass the test above while silently losing the glue. The
// witness is binary and needs no geometry — a skip taller than the text block has to
// push the second paragraph onto a second page.
func TestAWrappedVspaceStillContributesItsGlue(t *testing.T) {
	pages := func(mid string) int {
		t.Helper()
		const pre = `\documentclass{article}\makeatletter` +
			`\newif\ifLineNumbers \LineNumberstrue\def\zzhook{}` +
			`\def\@tempa#1#2{\expandafter\def\expandafter#2\expandafter{\expandafter` +
			`\ifLineNumbers\expandafter#1\expandafter\fi#2}}\@tempa\zzhook\vspace` +
			`\begin{document}A\par`
		e, err := compile([]byte(pre+mid+`\par B\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		return len(e.RenderPages(e.renderMargin(0)))
	}
	plain := pages(``)
	spaced := pages(`\vspace{600pt}`)
	if spaced <= plain {
		t.Errorf("the wrapped \\vspace contributed no glue: %d page(s) with a 600pt skip, %d without",
			spaced, plain)
	}
}
