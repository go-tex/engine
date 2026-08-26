// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \documentclass{amsart} now loads the real embedded amsart.cls, not the built-in
// emulation.
//
// The two blockers that forced the emulation gate are both cleared: amsart's
// \newtheorem…[section] machinery no longer runs away (a delimited-argument
// brace-stripping bug — a single enclosing { } pair was not removed, so
// \@oparg{\@ynthm{thm}}[] delivered {\@ynthm{thm}} and derailed a downstream
// delimited match into an infinite \@ynthm→\@oparg→\@ifnextchar cycle; fixed in
// stripOuterBraces, covered by TestDelimitedArgStripsSingleEnclosingGroup and, for
// amsart, TestAmsartNewtheoremNoLongerLoops), and the heading artifacts are fixed:
// the theorem head renders through \@labels (a real \newbox now), the stray author
// bullet is gone (a bare \item in a \trivlist carries no label), and the duplicated
// section number is gone (\sbox\z@{…}, amsart's toc measure, no longer spilled its
// content because \sbox accepts the low-level register form). The gate is now a
// single reachability check (realAmsart, GOTEX_AMSART=0 forces the emulation back),
// mirroring realBeamer. The real amsart.cls is embedded; the class-kernel additions
// it drove — token registers (toks_test.go), the ## parameter-char fix, and the
// plain-TeX substrate (amssubstrate_test.go) — benefit every real class/package.
func TestAmsartRoutesToRealClass(t *testing.T) {
	if emulatedClasses["amsart"] {
		t.Fatal("amsart should no longer be gated to the emulation")
	}
	if _, ok := embeddedTeXFile("amsart.cls"); !ok {
		t.Fatal("amsart.cls is not embedded in texmf/")
	}
	// A fresh engine resolves the embedded class, so \documentclass{amsart} routes
	// to it (realAmsart is true) unless GOTEX_AMSART=0.
	e := New()
	e.LoadLaTeX()
	if !e.realAmsart() {
		t.Fatal("realAmsart() is false although amsart.cls is embedded")
	}
}

// GOTEX_AMSART=0 forces the built-in emulation back (the A/B escape hatch, like
// GOTEX_BEAMER=0): realAmsart is then false and \documentclass{amsart} takes the
// emulation return in doDocumentClass instead of loading amsart.cls. The document
// still compiles, and the amsart text block (applyAmsartGeometry, run in both
// paths) is still installed.
func TestAmsartEmulationForcedByEnv(t *testing.T) {
	t.Setenv("GOTEX_AMSART", "0")
	e := New()
	e.LoadLaTeX()
	if e.realAmsart() {
		t.Fatal("GOTEX_AMSART=0 should force the emulation (realAmsart must be false)")
	}
	got, err := compile([]byte(`\documentclass{amsart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("amsart emulation errored: %v", err)
	}
	if want := ptToSP(360); got.hsize != want {
		t.Errorf("emulation hsize = %d, want %d (360pt)", got.hsize, want)
	}
}

// The real amsart class no longer loops on \newtheorem: with the delimited-argument
// brace-stripping fix, routing \documentclass{amsart} to the embedded class compiles
// the numbered-theorem machinery to completion instead of running away.
func TestAmsartNewtheoremNoLongerLoops(t *testing.T) {
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

// The real amsart class numbers within-section theorems CORRECTLY, not merely
// without looping. amsart's \@xthm freezes a within-numbered theorem's printed
// number as \the<within>\@thmcountersep\@thmcounter{<thm>}; those two hooks come
// from amsmath.sty, which the engine stubs, so before they were provided in the
// AMS substrate \the<thm> kept them verbatim and \label captured the literal
// "1\@thmcountersep\@thmcounter{thm}" instead of "1.1". This asserts the numbers
// (via the label table, exactly like the emulation's TestTheoremWithin): first
// section 1.1/1.2, a shared lemma continuing that counter to 1.3, and a reset to
// 2.1 in section 2.
func TestAmsartRealClassNumbersWithinSection(t *testing.T) {
	src := []byte(`\documentclass{amsart}` +
		`\newtheorem{thm}{Theorem}[section]` +
		`\newtheorem{lem}[thm]{Lemma}` +
		`\begin{document}` +
		`\section{One}` +
		`\begin{thm}\label{t:1}A.\end{thm}` +
		`\begin{thm}\label{t:2}B.\end{thm}` +
		`\begin{lem}\label{l:1}L.\end{lem}` +
		`\section{Two}` +
		`\begin{thm}\label{t:3}C.\end{thm}` +
		`\end{document}`)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("amsart real class errored: %v", err)
	}
	if e.runaway {
		t.Fatalf("amsart real class runs away (steps=%d)", e.steps)
	}
	for _, tc := range []struct{ key, want string }{
		{"t:1", "1.1"},
		{"t:2", "1.2"},
		{"l:1", "1.3"}, // shared counter, still within section 1
		{"t:3", "2.1"}, // reset on \section
	} {
		if got := e.labels[tc.key]; got != tc.want {
			t.Errorf("label %s = %q, want %q", tc.key, got, tc.want)
		}
	}
}
