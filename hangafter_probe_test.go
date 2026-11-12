// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// TestHangafterIsAnIntegerParameterNotADimen pins what \hangafter IS.
//
// ⛔ TWO DECLARATIONS DISAGREE ABOUT ITS TYPE. texparams.go lists "hangafter"
// among texIntParams with TeX's initial value 1 — which is right: tex.web §236
// makes \hangafter an INTEGER parameter. And amssubstrate.go:276 declares
// `\newdimen\hangafter`, which rebinds the same name to a dimension register.
//
// The two are not interchangeable, because they SCAN DIFFERENTLY. `\hangafter=1`
// against an integer takes the 1 and stops. Against a dimension it scans a
// <dimen>, which REQUIRES a unit — so the scanner reads past the 1, looking for
// pt/em/in in the text that follows, and whatever it swallows on the way never
// reaches the page.
//
// ⚠ How far that reaches is measured, not assumed, and it is narrow: a census
// over the 2935-paper corpus (comments stripped) finds 109 papers that write
// \hangafter and ZERO where the scan eats a word. The consequence below is
// exact and reproducible; its incidence here is nil. Both belong in the record.
//
// This test does not assume which one wins — the answer depends on load order,
// which is exactly the trap texparams.go records for \prevdepth, and the
// existing TestNoParameterIsBothAPrimitiveAndAListedParam does not cover a name
// re-declared by a SUBSTRATE rather than by a primitive.
func TestHangafterIsAnIntegerParameterNotADimen(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`BEFORE \hangafter=1 AFTERWARD` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	m := e.eq["hangafter"]
	if m == nil {
		t.Fatal(`\hangafter is not defined at all`)
	}
	if m.kind != mCountRef {
		t.Errorf(`\hangafter is kind %v, want mCountRef (tex.web §236: an INTEGER parameter)`, m.kind)
	}
	// The scanning consequence, which is what costs a reader text. BEFORE is the
	// control: it proves the document reached the page at all, so a missing
	// AFTERWARD is about the scanner and not about a compile that produced nothing.
	got := pageChars(e)
	if !strings.Contains(got, "BEFORE") {
		t.Fatalf("the control word is not on the page either — this test is measuring nothing: %q", got)
	}
	if !strings.Contains(got, "AFTERWARD") {
		t.Errorf("the word after \\hangafter=1 was swallowed by the scanner; page = %q", got)
	}
}

// TestHangafterScansAUnitOutOfTheFollowingWord is the consequence the type
// confusion actually has, and it is narrower than I first assumed.
//
// A word that cannot begin a unit survives: AFTERWARD above reaches the page.
// But TeX's unit names are ordinary letter pairs — in, pt, pc, bp, cm, mm, dd,
// cc, sp, ex, em — and a <dimen> scan that has read "1" and wants a unit will
// take those two letters out of whatever follows. "integers" begins with "in".
//
// An integer parameter cannot do this: it reads the 1 and stops.
func TestHangafterScansAUnitOutOfTheFollowingWord(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`BEFORE \hangafter=1 integers follow` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "BEFORE") {
		t.Fatalf("the control word is not on the page either — this test is measuring nothing: %q", got)
	}
	if !strings.Contains(got, "integers") {
		t.Errorf("\\hangafter=1 ate the \"in\" of \"integers\": page = %q", got)
	}
}
