// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The space factor's rules, as tex.web §1034's adjust_space_factor states them.
// Each row is one clause of that macro, and the fourth is the one worth having a
// row of its own: an sfcode of 0 leaves the factor ALONE, so a closing bracket
// after a period does not cancel the sentence.
func TestTheSpaceFactorFollowsEachCharactersSfcode(t *testing.T) {
	for _, c := range []struct {
		name, src string
		want      int
	}{
		{"a lowercase letter is 1000", `x`, 1000},
		{"an UPPERCASE letter is 999", `X`, 999},
		{"a period is 3000", `x.`, 3000},
		{"a comma is 1250", `x,`, 1250},
		{"a colon is 2000", `x:`, 2000},
		{"a semicolon is 1500", `x;`, 1500},
		// §1034: from below 1000 the factor goes to 1000, never straight past it —
		// which is the whole reason \@ exists.
		{"a period after a capital gives 1000, not 3000", `X.`, 1000},
		// sfcode 0 (latex.ltx:641 gives one to ) ' ]) leaves it untouched.
		{"a close paren after a period keeps 3000", `x.)`, 3000},
		{"\\@ sets it to 1000 by hand", `X\@`, 1000},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\noindent ` + c.src + `\message{[\the\spacefactor]}`); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := trimNL(e.out.String()); got != "["+itoa(c.want)+"]" {
			t.Errorf("%s: \\the\\spacefactor = %s, want [%d]", c.name, got, c.want)
		}
	}
}

// And what the factor is FOR: §1043 adds the font's extra_space to the interword
// glue at a sentence end, and scales stretch and shrink at every factor other
// than 1000.
//
// The expectation is the REFERENCE's shape, measured with Latin Modern on both
// sides (comparing absolute widths across different fonts would compare the
// fonts): three sentences of the same words, with commas and with periods —
//
//	           commas   periods   difference
//	tectonic    88.02    91.34     +3.32bp
//	gotex       86.38    89.37     +2.99bp
//
// and 0.00 before this existed. The residual is our space ADVANCE being 3.0pt
// where cmr10's \fontdimen2 is 3.33pt, which shows on the comma line too (86.38
// against 88.02) and is a different question.
func TestASentenceEndGetsTheFontsExtraSpace(t *testing.T) {
	width := func(src string) int {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\setbox0=\hbox{` + src + `}`); err != nil {
			t.Fatal(err)
		}
		return e.box[0].width
	}
	// spMock's space is 3pt, so extra_space is 1pt and a sentence end is 4pt.
	comma := width(`x, x`)
	period := width(`x. x`)
	if period <= comma {
		t.Fatalf("a sentence end is not wider: comma %dsp, period %dsp", comma, period)
	}
	if got, want := period-comma, unity; got != want {
		t.Errorf("a sentence end adds %dsp, want %dsp (spMock's space is 3pt, so extra_space is 1pt)", got, want)
	}
	// A capital before the period takes it back: sfcode 999 → factor 1000.
	if got := width(`X. x`); got != comma {
		t.Errorf("a period after a capital widened the space: %dsp, want the ordinary %dsp", got, comma)
	}
	// …and \@ puts it back, which is what \@ is for.
	if got := width(`X\@. x`); got != period {
		t.Errorf("\\@ did not restore the sentence end: %dsp, want %dsp", got, period)
	}
	// \frenchspacing means "no sentence spacing": every sfcode back to 1000.
	if got := width(`\frenchspacing x. x`); got != comma {
		t.Errorf("\\frenchspacing left the sentence space in: %dsp, want %dsp", got, comma)
	}
}

// \sfcode is readable and group-scoped, like \catcode. LaTeX asks
// \ifnum\sfcode`\.=\@m to find out whether \frenchspacing is in force, so a table
// that could not be read back would answer that question wrongly.
func TestSfcodeIsReadableAndScoped(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\message{[\the\sfcode46]}` + // 46 is "."
		`{\frenchspacing\message{[\the\sfcode46]}}` +
		`\message{[\the\sfcode46]}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got, want := trimNL(e.out.String()), "[3000] [1000] [3000]"; got != want {
		t.Errorf("\\the\\sfcode`\\. inside and outside a group = %q, want %q", got, want)
	}
}

// A control space is DELIBERATELY not adjusted: tex.web §20054 sends ex_space to
// append_normal_space, which takes the font's glue as it stands. "e.g.\ x" is how
// a document asks for an ordinary space after a period, and it has to work.
func TestAControlSpaceIgnoresTheSpaceFactor(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\setbox0=\hbox{x. x}\setbox1=\hbox{x.\ x}`); err != nil {
		t.Fatal(err)
	}
	sentence, control := e.box[0].width, e.box[1].width
	if control >= sentence {
		t.Errorf("a control space took the sentence widening: %dsp against %dsp", control, sentence)
	}
	if got, want := sentence-control, unity; got != want {
		t.Errorf("the control space is %dsp narrower, want %dsp — exactly the extra_space it skips", got, want)
	}
}
