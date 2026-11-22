// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"

	"github.com/go-tex/texmf"
)

// beamer's \section calls hyperref internals — \GetTitleString,
// \Hy@writebookmark, \hyper@anchorstart, \hyper@anchorend
// (beamerbasesection.sty:105, 135-136 and twins). With hyperref emulated they were
// undefined, and their arguments were typeset outside any frame: a page reading
// "One1OneOutline0.12tocOutline0.1" before the first frame of every talk with a
// \section.
func TestABeamerSectionLeavesNoPageOfItsOwn(t *testing.T) {
	withTeXMF(t, texmf.Beamer)
	e, err := compile([]byte(`\documentclass{beamer}
\begin{document}
\section{SECNAME}
\begin{frame}{FRAMETITLE}
FRAMEBODY
\end{frame}
\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := len(e.Pages()); n != 1 {
		t.Errorf("%d pages, want 1: the section must not open a page", n)
	}
	got := pageChars(e)
	for _, junk := range []string{"SECNAME", "Outline", "toc"} {
		if strings.Contains(got, junk) {
			t.Errorf("a hyperref internal's argument was typeset: %q in %q", junk, got)
		}
	}
	for _, want := range []string{"FRAMETITLE", "FRAMEBODY"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from the page %q", want, got)
		}
	}
}

// Each stub reads exactly hyperref's arguments: AFTER is text and must be typeset,
// and \GetTitleStringResult holds the title it was given.
func TestHyperrefInternalStubsReadTheirArguments(t *testing.T) {
	for _, c := range []struct{ name, call, want string }{
		{"title", `\GetTitleString{T1}`, "BEFORE AFTER"},
		{"title result", `\GetTitleString{T1}\GetTitleStringResult`, "BEFORE T1 AFTER"},
		{"bookmark", `\Hy@writebookmark{1}{B2}{B3}{B4}{B5}`, "BEFORE AFTER"},
		{"anchor", `\hyper@anchorstart{A1}\hyper@anchorend`, "BEFORE AFTER"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}
\usepackage{hyperref}
\begin{document}
\makeatletter
BEFORE `+c.call+` AFTER
\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			// pageChars carries no inter-word space: compare the letters alone.
			got := strings.Join(strings.Fields(pageChars(e)), "")
			if want := strings.ReplaceAll(c.want, " ", ""); got != want {
				t.Errorf("the page reads %q, want %q", got, c.want)
			}
		})
	}
}

// The stubs stand in only for a hyperref that was ASKED for. Without it, an
// undefined command's braced argument is an ordinary group, and typeset; and a
// definition the document made before requesting hyperref stands.
func TestHyperrefInternalsOnlyWhenHyperrefIsRequested(t *testing.T) {
	for _, c := range []struct{ name, preamble, want string }{
		{"no hyperref", ``, "SHOWN"},
		{"own definition", `\makeatletter\def\hyper@anchorstart#1{MINE}\makeatother
\usepackage{hyperref}`, "MINE"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}
`+c.preamble+`
\begin{document}
\makeatletter
BEFORE \hyper@anchorstart{SHOWN} AFTER
\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := pageChars(e); !strings.Contains(got, c.want) {
				t.Errorf("the page reads %q, want %q on it", got, c.want)
			}
		})
	}
}
