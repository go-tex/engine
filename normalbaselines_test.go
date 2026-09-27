// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \normalbaselines restores the three interline parameters from the saved copies the kernel
// keeps for them. latex.ltx:558-559, verbatim:
//
//	\def\normalbaselines{\lineskip\normallineskip
//	  \baselineskip\normalbaselineskip \lineskiplimit\normallineskiplimit}
//
// All three registers already existed here; only the macro was missing, and it was skipped
// 17 times across 6 corpus papers. acmart is the one where it shows: \@typeset@author@bx
// (acmart.cls:2656-2657) opens with \def\and{\par}\normalbaselines before
// \global\setbox\author@bx=\vtop{…}, so the author box of every bundled-acmart paper was
// built with whatever interline spacing the enclosing group happened to leave.
//
// ⛔ The measured page effect is almost nothing — 2304.11274 moves by 2 words and no paper
// changes page count. This is not shipped as a layout fix; it is shipped because the macro
// is part of the kernel, the registers it reads are already here, and a class that calls it
// should not have it skipped. The test asserts the three assignments, not a page.
func TestNormalbaselinesRestoresTheThree(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		// Move all three away from their saved values, then restore.
		`\normalbaselineskip=12pt \normallineskip=1pt \normallineskiplimit=2pt` +
		`\baselineskip=33pt \lineskip=7pt \lineskiplimit=9pt` +
		`\normalbaselines` +
		`[\the\baselineskip|\the\lineskip|\the\lineskiplimit]` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := e.Diagnostics().Skipped["normalbaselines"]; n != 0 {
		t.Fatalf("\\normalbaselines is undefined (%d skipped)", n)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// Each must come back from its saved copy, not keep the value set just above.
	for _, want := range []string{"12.0pt", "1.0pt", "2.0pt"} {
		if !contains(text, want) {
			t.Errorf("%q is not in %q — \\normalbaselines did not restore all three", want, firstN(text, 90))
		}
	}
	for _, bad := range []string{"33.0pt", "7.0pt", "9.0pt"} {
		if contains(text, bad) {
			t.Errorf("%q survived \\normalbaselines; it reads %q", bad, firstN(text, 90))
		}
	}
}
