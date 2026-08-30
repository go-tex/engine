// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A class calls kernel macros it never declares, and beamer runs a whole batch of
// them at \begin{document}. Undefined, each one stops a strict run and, in a lenient
// one, spills its arguments onto the page: \DeclareMathSymbol{0}\mathalpha{numbers}
// {"30} alone left the text 0, numbers and "30 behind, 62 times a talk.
func TestKernelMacrosAreDefined(t *testing.T) {
	src := `\documentclass{article}
\begin{document}\makeatletter
\def\chk#1{\expandafter\ifx\csname#1\endcsname\relax MANQUE:#1 \fi}
\chk{DeclareMathSymbol}\chk{@input}\chk{@input@}\chk{offinterlineskip}
\chk{reset@font}\chk{endgraf}\chk{@@par}\chk{color@begingroup}\chk{color@endgroup}
\chk{@arrayparboxrestore}\chk{if@filesw}\chk{@checkend}\chk{pdfstringdefDisableCommands}
\end{document}`
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); strings.Contains(got, "MANQUE") {
		t.Errorf("macro(s) manquante(s): %q", got)
	}
}

// The declarations must consume their arguments, and must do so in a STRICT run:
// what these documents need is not to be skipped but to be read and dropped.
func TestKernelDeclarationsConsumeTheirArguments(t *testing.T) {
	for _, c := range []struct{ nom, src string }{
		{"DeclareMathSymbol", `\DeclareMathSymbol{0}\mathalpha{numbers}{"30}`},
		{"pdfstringdefDisableCommands", `\pdfstringdefDisableCommands{\def\x{ne doit pas paraitre}}`},
		{"@checkend", `\@checkend{document}`},
		{"@input absent", `\@input{ce-fichier-n-existe-pas.tex}`},
	} {
		e, err := compile([]byte(`\documentclass{article}\begin{document}\makeatletter `+
			c.src+`x\end{document}`), Options{})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if got := pageChars(e); got != "x" {
			t.Errorf("%s: la page porte %q, want %q", c.nom, got, "x")
		}
	}
}
