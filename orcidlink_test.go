// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ orcidlink's \orcidlink{<id>} sets the ORCID logo as a tikz picture sized to the current
// X height (orcidlink.sty:51-59). The IDENTIFIER is never typeset — it goes into the \href
// target and into the empty second branch of \texorpdfstring, and nowhere else.
//
// Undefined, it was skipped — and skipping RELEASES the argument, so the bare identifier was
// set next to the author's name: "Author0009-0003-9684-6966 wrote this" here against
// "Author wrote this" from tectonic.
//
// Measured on 999 papers: 3147 uses; 50 papers write it in a .tex and 39 of them change,
// -3993 glyph paths and -4 pages. 29 of the 39 deltas are exact multiples of 19 — the length
// of an ORCID identifier — and the rest are that plus reflow: 2607.27411's author list holds
// 84 of them, 84x19 = 1596 against a measured 1608, and the paper loses a page because ~1600
// characters of text it never asked for are gone.

func orcidText(t *testing.T, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(e.RenderPages(e.renderMargin(0)), "")
}

func orcidGlyphs(t *testing.T, body string) int {
	t.Helper()
	return strings.Count(orcidText(t, body), "<path")
}

// ⭐ NEGATIVE is the win here: the identifier must leave the page. The assertion is against
// the same sentence without the command at all, which is what the reference sets.
func TestOrcidlinkDoesNotTypesetTheIdentifier(t *testing.T) {
	with := orcidGlyphs(t, `Author\orcidlink{0009-0003-9684-6966} wrote this.`)
	without := orcidGlyphs(t, `Author wrote this.`)
	if with != without {
		t.Errorf("%d glyph path(s) against %d for the same sentence without the command: "+
			"the identifier was typeset (an ORCID id is 19 characters)", with, without)
	}
}

// The argument must be CONSUMED, not merely unprinted: an unconsumed one would also swallow
// or displace what follows. The witness is the text after the command.
func TestOrcidlinkConsumesItsArgumentAndNothingElse(t *testing.T) {
	got := orcidGlyphs(t, `A\orcidlink{0000-0001-2345-6789}BC`)
	want := orcidGlyphs(t, `ABC`)
	if got != want {
		t.Errorf("%d glyph path(s) against %d for ABC: the command took too much or too "+
			"little", got, want)
	}
}

// An identifier ending in the X check digit is still 19 characters, and still must not print.
func TestOrcidlinkHandlesTheXCheckDigit(t *testing.T) {
	got := orcidGlyphs(t, `A\orcidlink{0000-0002-1825-009X}B`)
	want := orcidGlyphs(t, `AB`)
	if got != want {
		t.Errorf("%d glyph path(s) against %d for AB", got, want)
	}
}
