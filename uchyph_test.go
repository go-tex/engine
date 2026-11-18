// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// breaksIn runs the real hyphenation pass over one word preceded by a glue —
// the shape hyphenateList is given inside a paragraph — and counts the
// discretionaries it came back with.
//
// The glue is not decoration: a word is hyphenated only where one precedes it
// (precededByGlue, tex.web §894/§896), so a fixture without it measures nothing.
func breaksIn(e *Engine, word string) int {
	list := []node{glueNode{}}
	for _, r := range word {
		list = append(list, charNode{ch: r})
	}
	n := 0
	for _, nd := range e.hyphenateList(list) {
		if _, ok := nd.(discNode); ok {
			n++
		}
	}
	return n
}

// withUchyph returns an engine with \uchyph set to v.
func withUchyph(t *testing.T, v string) *Engine {
	t.Helper()
	e := New()
	if _, err := e.Run("\\uchyph=" + v); err != nil {
		t.Fatal(err)
	}
	return e
}

// ⛔⛔ \uchyph WAS SETTABLE AND READ BY NOBODY.
//
// tex.web:17584-17586, in the pre-hyphenation scan:
//
//	if lc_code(c)<>0 then
//	  if (lc_code(c)=c)or(uc_hyph>0) then goto done2
//	  else goto done1;
//
// A word whose first letter is uppercase is hyphenated only when \uchyph is
// positive; otherwise the attempt is abandoned for the whole word. This engine
// carried `"uchyph": 1` in texparams.go and consulted it nowhere, so a document
// that set \uchyph=0 — to keep proper nouns and acronyms whole — got its words
// broken anyway, silently. go-tex/engine#765 item (5).
//
// ⚠ The two cells that matter are the uppercase ones: lowercase must be
// unaffected by \uchyph, which is what makes this a case rule and not a switch
// that turns hyphenation off.
func TestUchyphDecidesOnlyWordsBeginningWithACapital(t *testing.T) {
	for _, c := range []struct {
		name, word, uchyph string
		wantBreaks         bool
	}{
		{"capital, uchyph=0", "Extraordinarily", "0", false},
		{"capital, uchyph=1", "Extraordinarily", "1", true},
		{"lowercase, uchyph=0", "extraordinarily", "0", true},
		{"lowercase, uchyph=1", "extraordinarily", "1", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := breaksIn(withUchyph(t, c.uchyph), c.word)
			if (got > 0) != c.wantBreaks {
				t.Errorf("%q with \\uchyph=%s got %d discretionaries, want %s: "+
					"tex.web:17585 refuses a word whose first letter is uppercase "+
					"only when \\uchyph<=0", c.word, c.uchyph, got,
					map[bool]string{true: "at least one", false: "none"}[c.wantBreaks])
			}
		})
	}
}

// ⛔ THE DEFAULT IS UNCHANGED, and that is the point of stating it. plain.tex and
// INITEX both leave \uchyph=1, so no ordinary document moves: a change here that
// also altered the default would be a rendering change wearing a parity fix's
// name, and nothing else in the suite is positioned to notice.
func TestUchyphDefaultsToOneSoCapitalsStillHyphenate(t *testing.T) {
	e := New()
	if got := e.namedInt("uchyph"); got != 1 {
		t.Fatalf("\\uchyph defaults to %d, want 1 (plain.tex, INITEX)", got)
	}
	if breaksIn(e, "Extraordinarily") == 0 {
		t.Error("a capitalised word is not hyphenated under the default \\uchyph=1")
	}
}

// ⛔ IT IS THE FIRST LETTER, AND IT REFUSES THE WHOLE WORD. `done1` abandons the
// attempt rather than restarting past the capital, so an interior capital is
// never consulted — the scan is already past done2 by then — and a leading one
// withholds every break in the word, not just the ones near it.
//
// ⚠ "McDonaldization" is here because it is the case an implementation that
// tested *any* uppercase letter would get wrong, and nothing else in the table
// above would catch that: both its readings agree on every row there.
func TestOnlyTheFirstLetterIsConsulted(t *testing.T) {
	if breaksIn(withUchyph(t, "0"), "McDonaldization") != 0 {
		t.Error("\"McDonaldization\" broke under \\uchyph=0: its FIRST letter is " +
			"uppercase, so tex.web:17585 refuses the word entire")
	}
	if breaksIn(withUchyph(t, "0"), "deMcDonaldization") == 0 {
		t.Error("\"deMcDonaldization\" did not break under \\uchyph=0: its first " +
			"letter is lowercase, and the capitals inside it are never consulted")
	}
}

// ⛔ AND \lccode DECIDES WHAT "UPPERCASE" MEANS, not the unicode package.
// §891:17480-17483 classifies by \lccode: nonletter (zero), lowercase (equal to
// itself), uppercase (anything else). A document that makes a capital its own
// \lccode has made it lowercase for this test, and \uchyph=0 must then let the
// word through.
//
// Without this row the implementation could call unicode.IsUpper and nothing
// would say so.
func TestTheDocumentsLccodeDecidesWhatIsUppercase(t *testing.T) {
	e := New()
	if _, err := e.Run("\\uchyph=0 \\lccode`\\E=`\\E"); err != nil {
		t.Fatal(err)
	}
	if breaksIn(e, "Extraordinarily") == 0 {
		t.Error("with \\lccode`\\E=`\\E the word's first letter IS its own lowercase, " +
			"so §891:17482 calls it lowercase and \\uchyph=0 does not apply")
	}
}
