// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"

	"github.com/go-tex/texmf"
)

// With pgf emulated (GOTEX_PGF unset), beamer's \RequirePackage of the pgf family
// is skipped, and pgf's declarations were left undefined. The rounded inner theme
// declares its block shadows lazily, at the first block, IN THE DOCUMENT, where an
// undefined command's arguments are typeset: every Warsaw talk printed
// "bmb@shadowball color(0bp)=(pgftransparent!50); ..." under its first block.
// The declarations now read pgf's own arguments and discard them.
func TestPGFDeclarationsPrintNothingWhenPGFIsEmulated(t *testing.T) {
	withTeXMF(t, texmf.Beamer)
	t.Setenv("GOTEX_PGF", "")
	e, err := compile([]byte(`\documentclass{beamer}
\usetheme{Warsaw}
\begin{document}
\begin{frame}{FIRSTTITLE}
\begin{block}{BLOCKTITLE}
BLOCKBODY
\end{block}
\end{frame}
\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	for _, junk := range []string{"pgftransparent", "bmb@shadow", "color(0bp)"} {
		if strings.Contains(got, junk) {
			t.Errorf("a shading declaration was typeset: %q on the page", junk)
		}
	}
	for _, want := range []string{"BLOCKTITLE", "BLOCKBODY"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from the page", want)
		}
	}
}

// Each declaration reads exactly pgf's arguments: what follows them is text and
// must be typeset. A stub one argument too greedy would eat AFTER; one too short
// would print an argument.
func TestPGFDeclarationStubsReadPGFsArguments(t *testing.T) {
	t.Setenv("GOTEX_PGF", "")
	for _, c := range []struct{ name, call string }{
		{"radial", `\pgfdeclareradialshading{n}{\pgfpoint{1pt}{1pt}}{rgb(0pt)=(1,0,0)}`},
		{"radial with deps", `\pgfdeclareradialshading[c]{n}{p}{spec}`},
		{"horizontal", `\pgfdeclarehorizontalshading{n}{1cm}{spec}`},
		{"vertical", `\pgfdeclareverticalshading{n}{1cm}{spec}`},
		{"functional", `\pgfdeclarefunctionalshading{n}{ll}{ur}{init}{fn}`},
		{"image", `\pgfdeclareimage[width=1cm]{n}{file}`},
		{"image without options", `\pgfdeclareimage{n}{file}`},
		{"mask", `\pgfdeclaremask{n}{file}`},
		{"alias", `\pgfaliasimage{n}{m}`},
		{"object", `\pgfdefobject{n}{a}{b}{c}`},
		{"fading", `\pgfdeclarefading{n}{content}`},
		{"use shading", `\pgfuseshading{n}`},
		{"use image", `\pgfuseimage{n}`},
		{"set colour", `\pgfsetcolor{n}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}
\usepackage{pgf}
\begin{document}
BEFORE `+c.call+` AFTER
\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got := strings.Join(strings.Fields(pageChars(e)), " ")
			if !strings.Contains(got, "BEFORE AFTER") && !strings.Contains(got, "BEFOREAFTER") {
				t.Errorf("the page reads %q, want BEFORE then AFTER and nothing between", got)
			}
		})
	}
}

// The stubs stand in only for a pgf that was ASKED for and not loaded. A document
// that never requests pgf keeps TeX's rule for an undefined command in the body:
// its braced argument is an ordinary group, and typeset.
func TestPGFDeclarationsStayUndefinedWithoutPGF(t *testing.T) {
	t.Setenv("GOTEX_PGF", "")
	e, err := compile([]byte(`\documentclass{article}
\begin{document}
BEFORE \pgfuseshading{SHOWN} AFTER
\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); !strings.Contains(got, "SHOWN") {
		t.Errorf("the page reads %q: without pgf the argument is text", got)
	}
}
