// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// amsart is still routed to the built-in emulation for \documentclass{amsart}.
//
// The loop that originally forced the gate is GONE: amsart's \newtheorem…[section]
// machinery no longer runs away. Its root cause was a delimited-argument
// brace-stripping bug (a single enclosing { } pair was not removed, so
// \@oparg{\@ynthm{thm}}[] delivered {\@ynthm{thm}} and derailed a downstream
// delimited match into an infinite \@ynthm→\@oparg→\@ifnextchar cycle); the fix is
// in stripOuterBraces (engine.go), covered by TestDelimitedArgStripsSingleEnclosingGroup
// and asserted here for amsart itself in TestAmsartNewtheoremNoLongerLoops.
//
// The gate remains ONLY as a page-count/timing precaution: routing amsart to the
// real class must first be proven, on the full cached-paper corpus in lenient mode,
// to (a) never hang, (b) never render FEWER pages than the emulation, and (c) keep a
// reasonable per-paper compile time. That corpus-level comparison is the remaining
// blocker, plus known cosmetic heading artifacts (a duplicated section number, a
// stray author bullet). The real amsart.cls stays embedded and loadable via
// \LoadClass, and the class-kernel additions it drove — token registers
// (toks_test.go), the ## parameter-char fix, and the plain-TeX substrate
// (amssubstrate_test.go) — remain under test and benefit every real class/package.
func TestAmsartGatedButEmbedded(t *testing.T) {
	if !emulatedClasses["amsart"] {
		t.Fatal("amsart should still be gated to the emulation pending the corpus page-count check")
	}
	if _, ok := embeddedTeXFile("amsart.cls"); !ok {
		t.Fatal("amsart.cls is not embedded in texmf/")
	}
}

// The real amsart class no longer loops on \newtheorem: with the delimited-argument
// brace-stripping fix, routing \documentclass{amsart} to the embedded class compiles
// the numbered-theorem machinery to completion instead of running away. This locks
// in the root-cause fix at the amsart level (independent of the gate decision, which
// is about page counts, not the hang).
func TestAmsartNewtheoremNoLongerLoops(t *testing.T) {
	delete(emulatedClasses, "amsart")
	defer func() { emulatedClasses["amsart"] = true }()
	// The exact construct that used to hang for ~90s: a numbered theorem counted
	// within section, plus a shared-counter and a starred variant.
	src := []byte(`\documentclass{amsart}` +
		`\newtheorem{thm}{Theorem}[section]` +
		`\newtheorem{lem}[thm]{Lemma}` +
		`\newtheorem*{rem}{Remark}` +
		`\begin{document}\section{S}\begin{thm}x\end{thm}\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("amsart real class errored: %v", err)
	}
	if e.runaway {
		t.Fatalf("amsart \\newtheorem still runs away (steps=%d): the brace-stripping fix did not take", e.steps)
	}
	// It finished in a handful of expansions, nowhere near the runaway ceilings.
	if e.steps > tightLoopSteps {
		t.Errorf("amsart compiled but took %d expansion steps; the loop is not fully resolved", e.steps)
	}
}
