// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \DeclareOption{X}{code} stores code in \ds@X, and a class may APPLY one of its own
// options by calling that macro. base/ltclass.dtx:
//
//	\long\def\@declareoption#1#2{%
//	   \xdef\@declaredoptions{\@declaredoptions,#1}%
//	   \toks@{#2}%
//	   \expandafter\edef\csname ds@#1\endcsname{\the\toks@}}
//
// and \ProcessOptions reaches the same macro through \@use@ption, which ends in
// \csname ds@\detokenize\expandafter{\CurrentOption}\endcsname. aas.cls sets its
// paper size that way (\ds@letterpaper, aas.cls:5547) — with the macro undefined,
// the call was skipped and the size never set. See #625.
func TestDeclareOptionDefinesDsMacro(t *testing.T) {
	withTempDir(t, map[string]string{
		"dscls.cls": `\DeclareOption{letterpaper}{\message{DS-RAN}}` +
			`\DeclareOption{a4paper}{\message{A4-RAN}}` +
			`\ds@letterpaper` +
			`\ProcessOptions\relax`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{dscls}\begin{document}x\end{document}`)
		if !strings.Contains(out, "DS-RAN") {
			t.Errorf("\\ds@letterpaper did not run the option's code; \\message=%q", out)
		}
		if strings.Contains(out, "A4-RAN") {
			t.Errorf("an option nobody asked for ran; \\message=%q", out)
		}
	})
}

// \@options is the LaTeX 2.09 spelling, and base/ltclass.dtx keeps it as exactly
// that: "\@options is a synonym for \ProcessOptions* for upward compatibility with
// LaTeX 2.09 style files", \def\@options{\ProcessOptions*}. A class written to it —
// aas.cls:6961 is its ONLY option processing — ran no option at all here.
func TestAtOptionsProcessesOptions(t *testing.T) {
	withTempDir(t, map[string]string{
		"optcls.cls": `\DeclareOption{big}{\message{BIG-RAN}}\@options`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass[big]{optcls}\begin{document}x\end{document}`)
		if !strings.Contains(out, "BIG-RAN") {
			t.Errorf("\\@options processed no option; \\message=%q", out)
		}
	})
}
