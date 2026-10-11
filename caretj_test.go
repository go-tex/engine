// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ^^J is char 10, and INITEX leaves char 10 at catcode 12: tex.web §232 gives
// catcode 5 to the CARRIAGE RETURN alone. This engine had char 10 at catcode 5
// too, because it reads a line ending as \n — but mouthChar never hands char 10 to
// the catcode table, it hands \endlinechar (13), so the entry cost nothing for
// line endings and broke the only way a document can write char 10.
//
// The expectation is the REFERENCE's, measured on
//
//	\the\catcode`\^^J    tectonic 12    this engine 5
//
// and the same probe says 5 for ^^M and 10 for ^^I on both sides, so this is the
// one entry that differed.
func TestCaretJIsAnOrdinaryCharacter(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("\\message{[\\the\\catcode`\\^^J|\\the\\catcode`\\^^M|\\the\\catcode`\\^^I]}"); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "[12|5|10]" {
		t.Errorf("catcodes of ^^J|^^M|^^I = %s, want [12|5|10] — tectonic's values", got)
	}
}

// And what the wrong catcode cost: a ^^J followed by the source's own newline was
// TWO ends of line, which is \par, and a \par inside a macro argument abandons the
// call (tex.web §392). A warning whose text spans lines — which is how warnings
// are written — therefore lost its call and left its PROSE in the input, to be
// typeset.
//
// fancyhdr's \@fancywarning is the shape that found it, and two corpus papers
// carried its text onto page 1 (go-tex/engine#607).
func TestAMultiLineArgumentWithCaretJSurvivesItsCall(t *testing.T) {
	const src = "\\def\\warn#1{}\n" +
		"\\warn\n" +
		"  {first line ^^J\n" +
		"   second line ^^J\n" +
		"   third line}%\n" +
		"\\noindent VISIBLE"
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	if strings.Contains(got, "second") {
		t.Errorf("the argument's prose reached the page: %q", got)
	}
	if !strings.Contains(got, "VISIBLE") {
		t.Errorf("the text after the call was lost: %q", got)
	}
	// And the engine must not be reporting an abandoned call either: that counter
	// is the alarm for exactly this, and it fired once on the witness above.
	if n := e.Diagnostics().RunawayArgs; n != 0 {
		t.Errorf("%d macro call(s) abandoned on a \\par; the argument spans lines, it has no \\par in it", n)
	}
}
