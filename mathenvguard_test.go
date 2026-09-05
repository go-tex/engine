// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A publisher class routinely copies latex.ltx's own \def\eqnarray verbatim to
// tweak its spacing. That definition drives \halign inside $$…$$, which the math
// renderer cannot execute: taking the copy does not change the spacing, it loses
// the environment — and, in arxiv.cls, left the rest of the document setting its
// formulas in TEXT mode, where every math command is undefined and the lenient
// path eats its argument. The engine keeps its own definition instead.
func TestHalignRedefinitionOfNativeMathEnvIsRefused(t *testing.T) {
	// The shape latex.ltx uses, reduced to what the guard looks at.
	classCopy := `\makeatletter
\def\eqnarray{\tabskip\@centering $$\everycr{}\halign to\displaywidth\bgroup\cr}
\def\endeqnarray{\egroup $$}
\makeatother`
	src := []byte(`\documentclass{article}
` + classCopy + `
\begin{document}
BEFORE
\begin{eqnarray} x = 1 \end{eqnarray}
AFTER $y = 2$ END
\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	// The prose on both sides must survive: losing the trailing text is exactly
	// how the failure showed up in the wild.
	for _, want := range []string{"BEFORE", "AFTER", "END"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q missing from output — the environment took the surrounding text with it: %q", want, text)
		}
	}
	if !e.keptNativeEnv["eqnarray"] {
		t.Error("the \\halign redefinition of eqnarray should have been refused")
	}
}

// The guard is narrow: a redefinition that does NOT use \halign is a real one
// and must be taken, and an unrelated macro is never touched.
func TestNonHalignRedefinitionIsTaken(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\def\eqnarray{PLAIN}\def\myalign{\halign{#\cr}}\noindent\eqnarray`); err != nil {
		t.Fatal(err)
	}
	if e.keptNativeEnv["eqnarray"] {
		t.Error("a redefinition without \\halign must be taken, not refused")
	}
	if got := mvlText(e.mvl); got != "PLAIN" {
		t.Errorf("redefined eqnarray typeset %q, want %q", got, "PLAIN")
	}
	if e.keptNativeEnv["myalign"] {
		t.Error("a macro that is not a native display-math environment must never be refused")
	}
}
