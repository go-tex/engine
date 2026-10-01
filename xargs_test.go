// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ Undefined, \newcommandx was skipped — and skipping RELEASES the arguments, so the
// replacement text was handed to the surrounding paragraph and typeset as prose. That is
// the harm class #513 measured at its worst; here it is caught in a unit test, because the
// corpus cannot show it: the three papers that use \newcommandx are truncated for other
// reasons or never call the macros they define, so pages and glyph paths are identical
// either way.
//
// 14 uses over 3 papers of the 999-paper corpus, every one with its optional positions as a
// LEADING run — [1=…] ten times, [1=x,2=z] twice, [1=x,2=z,3=\lr,4=\xz] twice — so the
// xparse specification this builds (O{default} then m) is exact.

func xargsRender(t *testing.T, src string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+src+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(e.RenderPages(e.renderMargin(0)), "")
}

func xargsGlyphs(t *testing.T, src string) int {
	t.Helper()
	return strings.Count(xargsRender(t, src), "<path")
}

// The body must not reach the page. Skipped, `ZZBODY` was set as prose; defined, only the
// call sites put anything there.
func TestNewcommandxDoesNotTypesetItsReplacementText(t *testing.T) {
	got := xargsGlyphs(t, `\newcommandx{\zzf}[2][1=a]{ZZBODY#1#2}X`)
	want := xargsGlyphs(t, `X`)
	if got != want {
		t.Errorf("%d glyph path(s) against %d for the same page without the definition: "+
			"the replacement text was released into the paragraph", got, want)
	}
}

// And it must define a usable command: the optional argument taken from the call when it is
// there, from the default when it is not.
func TestNewcommandxTakesItsOptionalArgumentAtPositionOne(t *testing.T) {
	// \zzg[2]{b} and \zzg{b} differ by one glyph — the 2 against the default's a, both one
	// glyph — so the counts are equal, and what separates them is WHICH glyph. Compare
	// against the literal each must produce instead.
	for _, c := range []struct{ call, same string }{
		{`\newcommandx{\zzg}[2][1=Q]{#1#2}\zzg{R}`, `QR`},
		{`\newcommandx{\zzg}[2][1=Q]{#1#2}\zzg[S]{R}`, `SR`},
	} {
		got, want := xargsGlyphs(t, c.call), xargsGlyphs(t, c.same)
		if got != want {
			t.Errorf("%s: %d glyph path(s), %s draws %d", c.call, got, c.same, want)
		}
	}
}

// ⛔ The optional positions may run past the first, which is the whole point of the package
// and the one thing \newcommand cannot express. 2608.05022 writes [1=x,2=z] on three
// arguments and [1=x,2=z,3=\lr,4=\xz] on four — the defaults are CONTROL SEQUENCES there,
// which is why the specification is built from tokens and not from a string.
func TestNewcommandxHandlesSeveralOptionalPositions(t *testing.T) {
	got := xargsGlyphs(t, `\newcommandx{\zzh}[3][1=P,2=Q]{#1#2#3}\zzh{R}`)
	want := xargsGlyphs(t, `PQR`)
	if got != want {
		t.Errorf("%d glyph path(s), PQR draws %d: two leading optional arguments were not "+
			"both defaulted", got, want)
	}
}

// A malformed entry is discarded rather than guessed at — a position that is not a positive
// integer leaves that argument mandatory, which at least consumes what the call site wrote.
// Guessing arity is what cost #497 its 62 wrong equations.
func TestParseNewcommandxDefaultsDiscardsAMalformedEntry(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		spec string
		want []int
	}{
		{`1=a`, []int{1}},
		{`1=a,2=b`, []int{1, 2}},
		{`2=b,1=a`, []int{1, 2}},
		{`nope=a,1=b`, []int{1}}, // no position
		{`0=a,1=b`, []int{1}},    // not positive
		{`1`, nil},               // no "="
	} {
		got := parseNewcommandxDefaults(tokenizeTeX(c.spec))
		if len(got) != len(c.want) {
			t.Errorf("%q gave %d default(s), want %d", c.spec, len(got), len(c.want))
			continue
		}
		for _, p := range c.want {
			if _, ok := got[p]; !ok {
				t.Errorf("%q: no default at position %d", c.spec, p)
			}
		}
	}
}
