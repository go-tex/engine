// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The substrate's figure*/table* must be defined GLOBALLY. A local \def is rolled back by
// the first group that closes after \begin{document}, and a document does not have to be
// well-behaved for that to happen: a macro call abandoned on a \par (tex.web §392) leaves
// the brace depth off, and the \def'd figure*/table* go with the group.
//
// The witness reproduces the cause rather than any one paper: an \author whose argument
// contains a blank line, then a figure*. It is written on elsarticle DELIBERATELY. The
// same witness on IEEEtran no longer abandons anything — IEEEtran declares its own \author
// \long (IEEEtran.cls:4882) and the engine now follows it — so an IEEEtran witness would
// assert nothing at all. The RunawayArgs check below is there to say so out loud: if a
// later change makes elsarticle's \author \long too, this test fails rather than quietly
// becoming a test of nothing.
func TestDblFloatSurvivesAnAbandonedCall(t *testing.T) {
	const src = `\documentclass{elsarticle}\begin{document}` +
		"\\author{A. Author\n\n\\thanks{with a \\par in the argument}\n}\n" +
		`\begin{figure*}FIGBODY\caption{THECAPTION}\end{figure*}` +
		`\begin{table*}TABBODY\caption{TABCAPTION}\end{table*}` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := e.Diagnostics().RunawayArgs; got == 0 {
		t.Fatalf("the witness abandons no call any more, so it guards nothing — " +
			"give it a class whose \\author is not \\long, or delete it")
	}
	// The abandoned call is expected and reported; what must NOT follow is a lost
	// environment.
	for _, env := range []string{"figure*", "table*"} {
		if got := e.Diagnostics().UndefinedEnvs[env]; got != 0 {
			t.Errorf("%s went with the group (%d) — the definition was local", env, got)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"THECAPTION", "TABCAPTION", "Figure 1", "Table 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 90))
		}
	}
}

// IEEEtran's \author is \long, and the class says why on the line above its own definition:
// "V1.7 allow \author to contain \par's. This is needed to allow \thanks to contain \par."
// (IEEEtran.cls:4881-4882). LaTeX's \author is deliberately not \long — latex.ltx declares it
// with the STARRED \DeclareRobustCommand — and the engine keeps that for article and amsart,
// which is right. Keeping it for IEEEtran was not: corpus paper 2405.05734 writes three
// \thanks separated by blank lines inside \author{…}, so the call was abandoned on the first
// \par, the closing brace read as an Extra }, and the title, the author line and the
// \begin{IEEEkeywords} block after it were all lost. Σ could not see any of it: the paper
// came out at 16 pages against the reference's 16.
func TestIEEEtranAuthorMayContainAPar(t *testing.T) {
	const src = `\documentclass[journal]{IEEEtran}\begin{document}` +
		`\title{THETITLE}` +
		"\\author{A. Author\n\n\\thanks{first note}\n\n\\thanks{second note}\n}\n" +
		`\maketitle` +
		"\\begin{abstract}\nTHEABSTRACT\n\\end{abstract}\n" +
		"\\begin{IEEEkeywords}\nTHEKEYWORD\n\\end{IEEEkeywords}\n" +
		`\section{Intro}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := e.Diagnostics().RunawayArgs; got != 0 {
		t.Errorf("\\author was abandoned %d time(s): it is not \\long", got)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// The title block, the abstract and the keyword block with IEEEtran's own name for it.
	for _, want := range []string{"THETITLE", "A. Author", "THEABSTRACT", "Index", "Terms", "THEKEYWORD"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 160))
		}
	}
}
