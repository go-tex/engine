// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \hyphenation{…} WAS DEFINED AS A STUB THAT THREW ITS ARGUMENT AWAY, and
// the comment six lines above that stub, in the same file, warns about exactly
// that shape:
//
//	"DISCARDS its argument is the one shape no channel can see: the document
//	 says what it wants, the engine says nothing, every test stays green, and
//	 the loss shows up only as ink that is not on the page."
//
// `\def\hyphenation#1{}` in amssubstrate.go was that, and its own comment said
// so in the words "nothing to record here" — true when written, silently false
// once anything could record it.
//
// ⚠ AND THE SILENCE FELL ON THE WORDS IT WAS ASKED ABOUT. A document writes
// \hyphenation for the words the patterns get WRONG; that is the whole reason
// to write it. 170 of the 2935 corpus papers do (5.8%).
//
// ⭐ MEASURED, tectonic 0.17.0, \hyphenation{photo-graphy manu-script}. The
// patterns' own answer for that word is `pho-tog-ra-phy`, so a break after
// `pho` is the shape that proves the entry was ignored.
//
// ⛔⛔ AND THE MEASURE IS 40pt, NOT 58pt, BECAUSE THIS TEST USED TO ASSERT
// SOMETHING THE REFERENCE DOES NOT DO. At \hsize=58pt tectonic sets
//
//	A photography / and a manu- / script and a bib- / liography fol- / low here.
//
// — `photography` WHOLE. The old fixture asserted `photo-` was present at that
// measure, which the reference's own output contradicts; the line simply fits,
// so no break inside the word is needed and TeX takes none. The assertion
// passed only because an emergency pass at an effectively infinite tolerance
// used to hyphenate far more than the reference (#768 finding (2)).
//
// At 40pt the word cannot fit and the reference takes the entry's point:
//
//	A photo- / graphy and / a manuscript / follow here.
//
// which is the fixture below. The lesson is the one in this package's notes
// already: a want-list is only a check if its entries are the reference's.
func TestAnExceptionBreaksWhereTheDocumentSaysAndNowhereElse(t *testing.T) {
	src := `\documentclass{article}\hyphenation{photo-graphy manu-script}` +
		`\begin{document}\hsize=40pt ` +
		`A photography and a manuscript follow here.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)

	// ⛔ The entry's own point, and NOT the patterns'.
	if !strings.Contains(got, "photo-") {
		t.Errorf("`photography` was not broken at the point the document gave "+
			"(`photo-`), which is where tectonic breaks it at this measure: %q", got)
	}
	if strings.Contains(got, "pho-tog") || strings.Contains(got, "pho-to") {
		t.Errorf("`photography` was broken where the PATTERNS say, not where the "+
			"document says: the exception was ignored: %q", got)
	}
}

// ⚠ A word with no entry still gets the patterns: an exception list replaces
// the patterns for ITS words only, and a test that did not check this would
// pass for an engine that had stopped hyphenating altogether.
//
// Its own measure, because the two questions want different ones: this needs a
// line narrow enough that `bibliography` must break, and the test above needs
// one narrow enough that `photography` must.
func TestAWordWithNoEntryStillGetsThePatterns(t *testing.T) {
	src := `\documentclass{article}\hyphenation{photo-graphy manu-script}` +
		`\begin{document}\hsize=40pt ` +
		`A bibliography follows here.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "bib") ||
		!(strings.Contains(got, "bibliog-") || strings.Contains(got, "bib-") ||
			strings.Contains(got, "bibli-") || strings.Contains(got, "bibliogra-")) {
		t.Errorf("`bibliography`, which has no entry, lost its pattern breaks: %q", got)
	}
}

// ⛔ AN ENTRY WITH NO HYPHEN FORBIDS THE WORD ENTIRELY. tex.web §937: the
// exception dictionary REPLACES the patterns for a word it holds, so
// \hyphenation{bibliography} says "never break this", and recording an empty
// list of points is what says it. Dropping such an entry as "nothing to record"
// would hand the word straight back to the patterns — the opposite of what was
// asked, and silently.
func TestAnEntryWithNoHyphenForbidsTheBreak(t *testing.T) {
	src := `\documentclass{article}\hyphenation{bibliography}` +
		`\begin{document}\hsize=58pt ` +
		`A photography and a manuscript and a bibliography follow here.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); strings.Contains(got, "bibliog-") || strings.Contains(got, "bib-") {
		t.Errorf("`bibliography` was broken although its entry gives no point: %q", got)
	}
}

// The parser, on its own: the key is the word without its hyphens, and the
// points count LETTERS before each one.
func TestAnEntryRecordsItsPointsByLetter(t *testing.T) {
	x := hyphExceptions{}
	x.add("as-so-ciate")
	x.add("photography")
	for _, c := range []struct {
		word string
		want []int
		ok   bool
	}{
		{"associate", []int{2, 4}, true},
		{"ASSOCIATE", []int{2, 4}, true}, // the lookup is case-folded
		{"photography", []int{}, true},   // an entry, with no point: never break
		{"elsewhere", nil, false},        // not an entry at all
	} {
		got, ok := x.lookup(c.word)
		if ok != c.ok {
			t.Errorf("lookup(%q) ok=%v want %v", c.word, ok, c.ok)
			continue
		}
		if len(got) != len(c.want) {
			t.Errorf("lookup(%q) = %v want %v", c.word, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("lookup(%q) = %v want %v", c.word, got, c.want)
				break
			}
		}
	}
}
