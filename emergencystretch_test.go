// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ \emergencystretch WAS ACCEPT-AND-IGNORE, and it is not a rare parameter.
// latex.go:1376 is LaTeX's own \sloppy:
//
//	\def\sloppy{\tolerance 9999\emergencystretch 3em\hfuzz .5pt\vfuzz\hfuzz}
//	\def\fussy{\emergencystretch 0pt\tolerance 200\hfuzz .1pt\vfuzz\hfuzz}
//
// so every \sloppy paragraph asks for 3em of emergency stretch, and until now
// only the \tolerance half of that request was honoured — sloppy_test.go says so
// in its own words ("\tolerance is the one this engine acts on … the others are
// accept-and-ignore parameters here"). Measured population: 8 of the 154-paper
// list use \sloppy and 4 more set \emergencystretch directly, disjointly, so 12
// of 154; and sloppy_test.go records \sloppy in 775 of the 2935 papers, 26.4%.
//
// tex.web §863 is the structure:
//
//	second_pass:=true; final_pass:=(emergency_stretch<=0);
//
// — so the second pass is the final one ONLY when the parameter is zero, and a
// positive one buys a THIRD pass that carries the stretch in the background.
// The engine hard-coded final_pass true, which made that third pass
// unreachable; go-typeset/linebreak v1.9.0 grew the Params field it needs.
//
// ⭐ THE WITNESS IS A LINE THAT STOPS OVERFLOWING. \emergencystretch adds finite
// stretch to every line of the emergency pass, so a measure that cannot be met
// under \fussy can be met under \sloppy — and the engine already counts exactly
// that, as Diagnostics().OverfullLines.
func TestEmergencyStretchReachesTheBreaker(t *testing.T) {
	// A narrow measure with long unbreakable-ish words: the shape #768's witness
	// uses, where the reference itself reports overfull boxes under \fussy.
	const body = `\hsize=72pt \parindent=0pt
Typographical incomprehensibilities overwhelm every paragraph.\par`

	run := func(t *testing.T, decl string) (overfull int, worst float64) {
		t.Helper()
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		src := `\documentclass{article}\begin{document}` + decl + body + `\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("Run: %v", err)
		}
		d := e.Diagnostics()
		return d.OverfullLines, d.WorstOverfull
	}

	fussyOver, fussyWorst := run(t, `\fussy `)
	sloppyOver, sloppyWorst := run(t, `\sloppy `)

	t.Logf("\\fussy  (emergencystretch 0pt): %d overfull line(s), worst %.2fpt", fussyOver, fussyWorst)
	t.Logf("\\sloppy (emergencystretch 3em): %d overfull line(s), worst %.2fpt", sloppyOver, sloppyWorst)

	// The control: without it the witness proves nothing, because a text that
	// overflows nowhere cannot show a parameter that stops an overflow.
	if fussyOver == 0 {
		t.Fatalf("\\fussy left no overfull line: the witness is broken, so the check below means nothing")
	}
	// ⭐ 3em of extra finite stretch per line must buy something: either fewer
	// lines overflow, or the worst one overflows less.
	if sloppyOver >= fussyOver && sloppyWorst >= fussyWorst {
		t.Errorf("\\sloppy bought nothing: %d overfull (worst %.2fpt) against \\fussy's %d (worst %.2fpt) — "+
			"\\emergencystretch is not reaching the breaker (tex.web §863)",
			sloppyOver, sloppyWorst, fussyOver, fussyWorst)
	}
}

// §863 gives the stretch to the THIRD pass, entered only when the second left
// something unset. A document that does not ask for it must reach identical
// code — which is what makes this change bit-identical for the other 142 of the
// 154 measured papers, by construction rather than by corpus run.
func TestWithoutEmergencyStretchNothingChanges(t *testing.T) {
	const body = `\hsize=200pt \parindent=0pt
The quick brown fox jumps over the lazy dog, and then does it again.\par`

	pages := func(t *testing.T, decl string) (int, int) {
		t.Helper()
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		src := `\documentclass{article}\begin{document}` + decl + body + `\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("Run: %v", err)
		}
		d := e.Diagnostics()
		return d.OverfullLines, d.UnderfullLines
	}

	// \fussy sets \emergencystretch to 0pt explicitly; the default is also 0.
	// Both must give the same answer, since neither can enter the third pass.
	declOver, declUnder := pages(t, `\fussy `)
	bareOver, bareUnder := pages(t, ``)
	if declOver != bareOver || declUnder != bareUnder {
		t.Errorf("\\fussy and the default disagree (%d/%d vs %d/%d): with \\emergencystretch at zero "+
			"the third pass is unreachable and both must reach identical code",
			declOver, declUnder, bareOver, bareUnder)
	}
}
