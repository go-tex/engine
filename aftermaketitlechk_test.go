// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ Skipping an undefined command is not neutral when its ARGUMENT is code.
//
// \aftermaketitle@chk{<name>} is revtex's guard against a front-matter construct used
// after \maketitle: the argument is only the construct's name, for the error message, and
// the real macro never typesets it. aastex inherits it and calls it from the BEGIN code of
// an environment it renews (aastex631.cls:896):
//
//	\renewenvironment{frontmatter@abstract}{%
//	  \aftermaketitle@chk{\begin{abstract}}%
//
// Undefined, it was skipped — and skipping RELEASED its argument, so \begin{abstract} ran,
// entered frontmatter@abstract, and reached \aftermaketitle@chk again. Unbounded recursion
// out of a name that was never meant to be typeset.
//
// Measured on 2607.24141 (aastex631, 123KB of source): 100000 skips — the runaway guard's
// own limit — 200005 groups left open, and the document came out as ONE page. Across the
// 999-paper corpus the same signature truncated four papers; restoring them is +226291
// glyph paths and +94 pages, and no other paper moves.

// The claim in its simplest form: the argument is discarded, not released.
func TestAfterMaketitleChkDiscardsItsArgument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter\begin{document}`+
		`A\aftermaketitle@chk{ZZSWALLOWED}B\makeatother\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// Two glyphs for A and B, one for the page number. ZZSWALLOWED would add eleven.
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "<path"); n != 3 {
		t.Errorf("%d glyph path(s), want 3 (A, B and the page number): the argument was "+
			"typeset instead of discarded", n)
	}
}

// ⛔ The corpus shape, and the reason this is a truncation rather than a cosmetic loss: an
// environment whose begin-code passes \begin{<itself>} as the argument. Released, that is
// unbounded recursion; discarded, the environment runs once.
//
// The witness asserts the runaway guard did NOT trip and the body IS typeset, because
// either alone can hold while the document is destroyed: a truncated render still "writes"
// a page, and a guard that trips still returns a PDF.
func TestAnEnvironmentThatNamesItselfInTheGuardDoesNotRecurse(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+
		`\newenvironment{zzfront}{\aftermaketitle@chk{\begin{zzfront}}}{}`+
		`\makeatother\begin{document}\begin{zzfront}XY\end{zzfront}\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.Runaway {
		t.Error("the runaway guard tripped: the argument was released and the environment " +
			"re-entered itself")
	}
	if d.OpenGroups > 0 {
		t.Errorf("%d group(s) left open, want 0", d.OpenGroups)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "<path"); n != 3 {
		t.Errorf("%d glyph path(s), want 3 (X, Y and the page number): the body was lost", n)
	}
}
