// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The substrate's figure*/table* must be defined GLOBALLY. A local \def is rolled back by
// the first group that closes after \begin{document}, and a document does not have to be
// well-behaved for that to happen.
//
// Corpus paper 2405.05734 writes its IEEEtran \author{…} across BLANK LINES, so the call is
// abandoned on a \par (tex.web §392), the brace depth is left off, and the \def'd
// figure*/table* went with the group. Its five appendix floats then lost their captions and
// their numbers: the reference has Fig. 13 … Fig. 17 and we repeated "Figure 12". The same
// document lost table* the same way.
//
// A real class defines these with \newenvironment, which is global; so must this.
//
// The witness reproduces the cause rather than the paper: an \author whose argument contains
// a blank line, then a figure*.
func TestDblFloatSurvivesAnAbandonedCall(t *testing.T) {
	const src = `\documentclass{IEEEtran}\begin{document}` +
		"\\author{A. Author\n\n\\thanks{with a \\par in the argument}\n}\n" +
		`\begin{figure*}FIGBODY\caption{THECAPTION}\end{figure*}` +
		`\begin{table*}TABBODY\caption{TABCAPTION}\end{table*}` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
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
