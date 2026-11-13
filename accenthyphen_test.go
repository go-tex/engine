// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ AN ACCENTED WORD BROKE IN THE WRONG PLACE, AND THE SUITE WAS GREEN
// THROUGHOUT.
//
// go-typeset/hyphenation built its value array over the augmented word by BYTE
// index and read it back by RUNE index (its #8, fixed in v1.0.1). The two agree
// for exactly as long as every letter is one byte — every English word and no
// French one — so every test this engine had went on passing while French,
// German, Czech, Polish and Vietnamese words broke at an offset.
//
// ⭐ MEASURED against tectonic 0.17.0 on this paragraph at \hsize=72pt, the
// same English patterns on both sides (no babel, no language option):
//
//	reference   Une présenta-/tion … représentations élé-/mentaires …
//	v1.0.0      Une présent-/ation … des représentat-/ions éléme-/ntaires …
//	v1.0.1      Une présenta-/tion … représentations élémentaires …
//
// `éléme-ntaires` is the shape of the defect: a break inside a syllable, three
// letters from where any pattern could put one, and exactly as far out as the
// word has multi-byte letters before it.
//
// ⚠ THE MUTATION FOR THIS TEST IS THE DEPENDENCY. Pinning go.mod back to
// hyphenation v1.0.0 reddens it; that is how both outputs above were produced,
// from two builds of this engine differing only in that line.
func TestAnAccentedWordBreaksWhereTheReferenceBreaksIt(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=72pt ` +
		`Une présentation générale des représentations élémentaires, avec des ` +
		`considérations supplémentaires sur la compréhension des phénomènes ` +
		`thermodynamiques particulièrement irrégulières.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)

	// The one word both engines break in the same place: tectonic sets
	// `présenta-` / `tion`, and so must we.
	if !strings.Contains(got, "présenta-") {
		t.Errorf("`présentation` is not broken where the reference breaks it "+
			"(`présenta-` / `tion`): %q", firstN(got, 120))
	}
	// ⛔ AND THE OFFSET BREAK MUST BE GONE. `éléme-ntaires` cuts between the m
	// and the n of a syllable; no pattern in hyphen.tex can put a break there,
	// which is what made it a defect in the INDEXING rather than in the
	// patterns.
	for _, bad := range []string{"éléme-", "présent-ation", "représentat-", "considérat-"} {
		if strings.Contains(got, bad) {
			t.Errorf("the byte/rune offset is back: %q is in the page", bad)
		}
	}
}
