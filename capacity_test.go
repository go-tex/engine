// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
	"time"
)

// ⛔ A .tex is a program and this package is its interpreter, run over untrusted
// input. Two guards here bound what one document can cost, and both were found by
// probing rather than by reading:
//
//   - a macro that expands EXPONENTIALLY (ten macros, each ten copies of the next
//     — the "billion laughs" shape) asks for 10^10 characters while taking few
//     expansion steps and keeping a shallow input stack, so neither the step
//     ceiling nor the depth ceiling could fire. Measured before the fix: 15.96 GB
//     of resident memory in 30 seconds, still growing. The reference engine stops
//     — tectonic fails the same file in 14.5 s at 4.3 GB with TeX's capacity error.
//   - and 98.6% of that time was inside the HYPHENATOR, because a document with no
//     space in it is one enormous word and Liang's algorithm is quadratic in a
//     word's length. TeX does not hyphenate a word longer than 63 letters
//     (tex.web §891); neither does this, now.
//
// Together the same file fails in under half a second at ~90 MB.
func TestADocumentCannotExhaustMemoryByExpanding(t *testing.T) {
	// Ten levels, ten copies each: \j is 10^10 characters if fully expanded.
	var b strings.Builder
	b.WriteString(`\documentclass{article}\begin{document}`)
	b.WriteString(`\def\a{xxxxxxxxxx}`)
	prev := 'a'
	for _, c := range "bcdefghij" {
		b.WriteString(`\def\` + string(c) + `{`)
		for i := 0; i < 10; i++ {
			b.WriteString(`\` + string(prev))
		}
		b.WriteString(`}`)
		prev = c
	}
	b.WriteString(`\j\end{document}`)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := compile([]byte(b.String()), Options{Lenient: true}); err == nil {
			t.Error("the engine reported no error for a document that asks for 10^10 characters")
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("still running after 30s: the capacity guard did not bound the work")
	}
}

// ⛔ The hyphenation bound is TeX's rule, so it is asserted as a RULE and not as a
// timing: a word of 63 letters may be hyphenated, a longer one may not. Without
// it the guard above would be a stopwatch, which is not a test.
func TestALongWordIsNotHyphenated(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	count := func(word string) int {
		list := make([]node, 0, len(word))
		for _, r := range word {
			list = append(list, charNode{ch: r})
		}
		n := 0
		for _, x := range e.hyphenateList(list) {
			if _, ok := x.(discNode); ok {
				n++
			}
		}
		return n
	}
	short := strings.Repeat("hyphenation", 5) // 55 letters, under the ceiling
	if count(short) == 0 {
		t.Fatalf("a %d-letter word got no hyphenation points: the control is broken, so the check below means nothing", len(short))
	}
	long := strings.Repeat("hyphenation", 10) // 110 letters, over it
	if got := count(long); got != 0 {
		t.Errorf("a %d-letter word got %d hyphenation points, want 0 (tex.web §891)", len(long), got)
	}
}

// ⛔ And the same expansion routed into a MACRO rather than onto a page. The ceiling
// above counts what expansion puts in a paragraph; \edef\boom{…} never builds one, so
// the identical bomb passed it untouched. A paired witness, the same source bytes
// differing only in the last line, measured on v0.234.1:
//
//	the expansion TYPESET      94 MB   0.06 s   maxParNodes fires
//	the same in an \edef     5072 MB   1.66 s   nothing fires
//
// The reference stops both: tectonic fails the \edef form in 0.10 s at 236 MB with
// "TeX capacity exceeded, sorry [main memory size=5000000]". With maxTokenList the
// same file returns in half a second at ~99 MB.
//
// \boom is never USED here, deliberately: the cost is in holding the tokens, and a
// test that typeset them would be testing the paragraph ceiling again.
func TestADocumentCannotExhaustMemoryByBuildingAReplacementText(t *testing.T) {
	var b strings.Builder
	b.WriteString(`\documentclass{article}\makeatletter`)
	b.WriteString(`\def\a{xxxxxxxxxx}`)
	prev := 'a'
	for _, c := range "bcdefghij" {
		b.WriteString(`\def\` + string(c) + `{`)
		for i := 0; i < 10; i++ {
			b.WriteString(`\` + string(prev))
		}
		b.WriteString(`}`)
		prev = c
	}
	b.WriteString(`\begin{document}\edef\boom{\j}X\end{document}`)

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := compile([]byte(b.String()), Options{Lenient: true}); err == nil {
			t.Error("no error for an \\edef that asks for 10^10 characters")
		}
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("still running after 30s: the token ceiling did not bound the work")
	}
}

// ⛔ The negative, which matters more: a ceiling that fires on a real document is worse
// than none. The largest replacement text any of the 154 corpus papers builds is 2999
// tokens (median 414), so a list two orders of magnitude above that must still pass —
// this builds ~100 000 and has to come through untouched.
func TestAReplacementTextTheSizeARealDocumentBuildsIsUntouched(t *testing.T) {
	var b strings.Builder
	b.WriteString(`\documentclass{article}\makeatletter`)
	b.WriteString(`\def\a{xxxxxxxxxx}`) // 10
	prev := 'a'
	for _, c := range "bcde" { // 10 -> 100 -> 1000 -> 10_000 -> 100_000
		b.WriteString(`\def\` + string(c) + `{`)
		for i := 0; i < 10; i++ {
			b.WriteString(`\` + string(prev))
		}
		b.WriteString(`}`)
		prev = c
	}
	b.WriteString(`\begin{document}\edef\big{\e}\message{[LEN:\the\numexpr0\relax]}X\end{document}`)

	e, err := compile([]byte(b.String()), Options{Lenient: true})
	if err != nil {
		t.Fatalf("a replacement text of ~100 000 tokens was refused: %v", err)
	}
	if d := e.Diagnostics(); d.Runaway {
		t.Error("the guard fired on a list a real document could plausibly build")
	}
}
