// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// a2eText sets body inside an algorithm float under [opts]algorithm2e and returns the
// page text with spaces removed (glue is not a character, so the spacing is not stable).
func a2eText(t *testing.T, opts, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}
\usepackage[`+opts+`]{algorithm2e}
\begin{document}
\begin{algorithm}
\DontPrintSemicolon
`+body+`
\end{algorithm}
\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(pageText(e), " ", "")
}

// ⭐ algorithm2e's \If, \Else, \For, \While and \Repeat were the algorithmicx fallback's,
// defined LAST in the prelude, so the argument-taking block forms of algorithm2e lost to
// one-argument versions and the body leaked out of its block. Measured against tectonic on
// [ruled,linesnumbered]algorithm2e: fifteen numbered lines
//
//	1 if x > 0 then / 2 y←x / 3 else / 4 y ← −x / 5 end / 6 for i ← 1 to n do / …
//
// The case below pins the ORDER of the lines and their numbers: a body that ran into its
// header would drop a number, and an unnumbered keyword line would shift every later one.
func TestAlgorithm2eBlocksAreNumberedLineByLine(t *testing.T) {
	got := a2eText(t, "linesnumbered", `\eIf{P}{a\;}{b\;}
\For{Q}{\uIf{R}{c\;}\Else{d\;}}`)
	// 1 if P then / 2 a / 3 else / 4 b / 5 end / 6 for Q do / 7 if R then / 8 c / 9 else /
	// 10 d / 11 end / 12 end
	want := "1ifPthen2a3else4b5end6forQdo7ifRthen8c9else10d11end12end"
	if !strings.Contains(got, want) {
		t.Errorf("block lines or numbers differ:\n got %q\nwant %q", got, want)
	}
}

// The numbers come from the OPTION, which an emulated package records as
// opt@algorithm2e.sty: without linesnumbered the same body has no numbers at all.
func TestAlgorithm2eNumbersOnlyUnderLinesnumbered(t *testing.T) {
	got := a2eText(t, "ruled", `\eIf{P}{a\;}{b\;}`)
	want := "ifPthenaelsebend"
	if !strings.Contains(got, want) {
		t.Errorf("unnumbered body differs:\n got %q\nwant %q", got, want)
	}
}

// \SetKwProg declares a block with a Title line, a body and an End line, and \KwIn prints
// "Input: text" WITHOUT a number (the reference numbers neither, and counts the lines after).
func TestAlgorithm2eProgAndInputLine(t *testing.T) {
	got := a2eText(t, "linesnumbered", `\SetKwProg{Fn}{Function}{:}{end}
\KwIn{X}
\Fn{F}{r\;}`)
	want := "Input:X1FunctionF:2r3end"
	if !strings.Contains(got, want) {
		t.Errorf("\\KwIn / \\SetKwProg lines differ:\n got %q\nwant %q", got, want)
	}
}
