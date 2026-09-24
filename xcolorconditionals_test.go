// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// An UNDEFINED conditional is the one thing a TeX engine cannot skip past: it
// cannot know the token is an \if, so the matching \else and \fi are swallowed
// with it and every group opened since stays open.
//
// xcolor declares two switches saying WHEN it converts a colour to the target
// model (xcolor.sty:73-74), and a package loaded FOR REAL reads them —
// pgfplots.sty:110-121 tests them nested:
//
//	\ifconvertcolorsD … \else \ifconvertcolorsU … \fi \fi
//
// This engine emulates xcolor natively (packages.go neverLoadReal) and so never
// declared them. Both are false: it keeps every colour in RGB and converts
// nothing, which is also what \newif gives.
func TestXcolorConversionSwitchesExistAndAreFalse(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// Exactly pgfplots' nested test, which must reach the \else of the outer \if
	// and leave nothing open.
	out, err := e.Run(`\message{[\ifconvertcolorsD D\else\ifconvertcolorsU U\else N\fi\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[N]") {
		t.Errorf("got %q, want [N] — both switches false, both \\fi consumed", out)
	}
	if n := len(e.SkippedCommands()); n != 0 {
		t.Errorf("skipped %v, want none", e.SkippedCommands())
	}
	if len(e.groups) != 0 {
		t.Errorf("%d group(s) left open by a conditional that should have skipped cleanly",
			len(e.groups))
	}
}

// \newif also builds the setters, so a package that flips one is not left with an
// undefined command either.
func TestXcolorConversionSwitchesCanBeSet(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\convertcolorsDtrue\message{[\ifconvertcolorsD D\else N\fi]}` +
		`\convertcolorsDfalse\message{[\ifconvertcolorsD D\else N\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[D]") || !strings.Contains(out, "[N]") {
		t.Errorf("got %q, want both [D] and [N]", out)
	}
}
