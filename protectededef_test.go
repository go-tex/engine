// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \protected@edef / \protected@xdef must keep a \protect'd token from expanding
// while the body is expanded, as real LaTeX does. If \protect stays a no-op the
// (x)edef expands right through it: a robust command whose replacement text names
// itself (or another robust command) then expands forever and swallows the rest of
// the document. This is what bm.sty (\protected@edef\bm#1{\bm{#1}}) and imsart
// (\protected@xdef\@thanks{…\protect\thanks@thefnmark…}) do. Here \risky loops if
// expanded; behind \protect in a \protected@edef it must not be, and the body that
// follows must render.
func TestProtectedEdefKeepsProtectUnexpanded(t *testing.T) {
	src := `\documentclass{article}` +
		`\makeatletter\def\risky{\risky}\protected@edef\out{X\protect\risky Y}\makeatother` +
		`\begin{document}BODYMARKER text after the protected definition.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.runaway {
		t.Error("expansion ran away: \\protect did not stop \\risky from expanding in \\protected@edef")
	}
	if got := pageChars(e); !strings.Contains(got, "BODYMARKER") {
		t.Errorf("body swallowed by the runaway; want BODYMARKER, got %q", got)
	}
}

// The protection still lets an ordinary (unprotected) token expand normally — the
// mechanism only guards \protect'd tokens.
func TestProtectedEdefExpandsUnprotected(t *testing.T) {
	src := `\makeatletter\def\word{HELLO}\protected@edef\out{\word-\protect\word}\makeatother\message{[\meaning\out]}`
	out, _ := runLaTeX(t, src)
	// The first \word expanded to HELLO; the \protect'd one stayed as \word.
	if !strings.Contains(out, "HELLO-") {
		t.Errorf("unprotected \\word did not expand; \\out = %q", out)
	}
	if !strings.Contains(out, `\word`) {
		t.Errorf("protected \\word was expanded (should stay literal); \\out = %q", out)
	}
}
