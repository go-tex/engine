// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A PAGE BROKEN AGAINST A HEIGHT THE DOCUMENT NEVER ASKED FOR MUST SAY SO.
//
// `effectiveVsize` falls back to plain TeX's 8.9in when \vsize is unset, which is
// 643.20pt against article's 550pt — 93pt, more than seven lines, of page per page.
// A Σ|deviation| that contains such a document says nothing about the engine, and
// the fallback fired in silence, so nobody could tell which documents those were.
//
// That silence is how its own comment came to assert, unchecked, that revtex and
// aastex land here. Measured with this counter on 2307.10134, a revtex paper: it
// stays at zero. \vsize is set there.
func TestTheVsizeFallbackCountsItself(t *testing.T) {
	e := New()
	e.vsize = 0
	if got, want := e.effectiveVsize(), ptToSP(8.9*7227.0/100.0); got != want {
		t.Errorf("effectiveVsize = %s, want %s (plain TeX)", formatPt(got), formatPt(want))
	}
	if e.skippedCS["gotex@vsize-fallback"] != 1 {
		t.Errorf("le repli ne s'est pas recensé: %v — un repli invisible est une "+
			"mesure que personne ne sait qu'il lit", e.skippedCS)
	}
}

// ⛔ THE CONTROL: with \vsize set, nothing is counted and nothing is replaced. A
// counter that ticks on the ordinary path is worse than no counter, because it makes
// every run look like the broken case.
func TestNothingIsCountedWhenVsizeIsSet(t *testing.T) {
	e := New()
	e.vsize = ptToSP(550)
	if got, want := e.effectiveVsize(), ptToSP(550); got != want {
		t.Errorf("effectiveVsize = %s, want %s (\\vsize)", formatPt(got), formatPt(want))
	}
	if len(e.skippedCS) != 0 {
		t.Errorf("rien ne devait être recensé: %v", e.skippedCS)
	}
}

// ⛔⛔ AND \textheight IS THE SAME REGISTER, which is why there is no second fallback
// here however much ltoutput.dtx:183 invites one ("Page layout must initialize
// \@colht and \@colroom to \textheight"). This engine \lets \textheight to \vsize,
// so `if \textheight > 0` after `if \vsize > 0` is unreachable by construction.
//
// This test exists so that the next reader who opens ltoutput.dtx and has the same
// idea finds out in one `go test` instead of in a corpus sweep — and so that the day
// the two are separated, it fails and says why.
func TestTextheightIsTheSameRegisterAsVsize(t *testing.T) {
	e, err := buildEngine(Options{}, true) // the LaTeX lead is where the \let lives
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\vsize=273pt\message{\the\textheight}`); err != nil {
		t.Fatal(err)
	}
	if got := trimNL(e.out.String()); got != "273.0pt" {
		t.Errorf("\\the\\textheight = %q après \\vsize=273pt, want \"273.0pt\": "+
			"si ces deux noms ne sont plus un seul registre, le repli de "+
			"effectiveVsize peut enfin consulter \\textheight — et doit le faire "+
			"(ltoutput.dtx:183, 288-296)", got)
	}
}
