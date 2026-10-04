// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ \MakeTextUppercase has to EXPAND its argument, because amsart's \altucnm reads the
// result back through \the — and \the inside an \edef contributes its tokens UNEXPANDED.
//
// amsart.cls:429 switches \uppercasenonmath to \altucnm as soon as \MakeTextUppercase is
// defined (which the ams substrate does, for the documents that call it), and :424 is
//
//	\def\altucnm#1{\MakeTextUppercase{\toks@{#1}}\edef#1{\the\toks@}}
//
// so \MakeTextUppercase receives an ASSIGNMENT to perform, not text to typeset. It must
// expand \@title BEFORE \uppercase hands the list back to the mouth, or \toks@ ends up
// holding the control sequence \@title itself — and then \edef\@title{\the\toks@}, whose
// \the does not expand, writes \@title := \@title. The engine spun it 400 times into the
// runaway guard: 2511.23047 rendered ONE page of 22KB, 2604.01571 one page of its 48.
//
// Two forms were refuted before this one, and each refutation is the reason the test below
// checks \meaning rather than a page count:
//
//   - \let\MakeTextUppercase\MakeUppercase — \MakeUppercase does not exist when the ams
//     substrate runs; only amsart.cls:431 \lets it to \uppercase, later. A \let here binds
//     the undefined.
//   - \def\MakeTextUppercase#1{\uppercase{#1}} — executes the \toks@ assignment, so the
//     loop looks addressed, but \uppercase does not expand a control sequence: \toks@ still
//     receives the token \@title and the self-reference survives. Measured: no change.

// The idiom in isolation, with no class involved: \@title must come out as its TEXT.
func TestMakeTextUppercaseExpandsSoTheToksIdiomDoesNotSelfReference(t *testing.T) {
	const src = `\documentclass{article}\makeatletter` +
		// The document's own disarming is the condition: it is what leaves an indirection
		// through \MakeUppercase with nothing behind it.
		`\let\MakeUppercase\relax` +
		`\def\@title{Hearing the Sides}` +
		`\def\altucnm#1{\MakeTextUppercase{\toks@{#1}}\edef#1{\the\toks@}}` +
		`\altucnm\@title` +
		`\message{[\meaning\@title]}` +
		`\begin{document}x\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().Messages
	// The self-reference reads back as a macro whose body is \@title again.
	if strings.Contains(got, `->\@title`) {
		t.Fatalf("\\@title is self-referential after \\altucnm: %s", got)
	}
	if !strings.Contains(strings.ToUpper(got), "HEARING THE SIDES") {
		t.Fatalf("\\@title lost its text through \\altucnm: %s", got)
	}
}

// And the shape a real paper has: a document that disarms \MakeUppercase around \maketitle
// must still reach its body. Without the fix the guard trips and five groups stay open.
func TestAmsartMaketitleSurvivesADocumentDisarmingMakeUppercase(t *testing.T) {
	const src = `\documentclass{amsart}` +
		`\title[Hearing the Sides]{Hearing the Sides}\author{A Name}` +
		`\begin{document}` +
		`\begingroup\let\MakeUppercase\relax\maketitle\endgroup` +
		`\section{Body}Visible prose.` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.Runaway {
		t.Error("the runaway guard tripped: \\maketitle looped on \\@title")
	}
	if d.OpenGroups != 0 {
		t.Errorf("OpenGroups = %d, want 0 — the abort left the document's groups open", d.OpenGroups)
	}
	if n := strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path"); n < 20 {
		t.Errorf("only %d glyph paths drawn: the body did not survive \\maketitle", n)
	}
}
