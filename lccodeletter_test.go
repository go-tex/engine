// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// wordAt gathers the word that starts just after a leading glue, and reports the
// character that ended it (0 when the list ran out).
func wordAt(e *Engine, text string) (string, rune) {
	// A glue in front, because a word is only gathered where one precedes it
	// (precededByGlue, tex.web §894/§896). gatherWord itself does not test that;
	// the glue is here so the fixture is the shape hyphenateList passes in.
	list := []node{glueNode{}}
	for _, r := range text {
		list = append(list, charNode{ch: r})
	}
	_, letters, j := e.gatherWord(list, 1)
	var end rune
	if j < len(list) {
		if c, ok := list[j].(charNode); ok {
			end = c.ch
		}
	}
	return string(letters), end
}

// ⛔⛔ \lccode DECIDES WHAT A LETTER IS, AND THE PUNCTUATION WAS INSIDE THE WORD.
//
// tex.web:17601, inside the hyphenation scan:
//
//	if lc_code(c)=0 then goto done3
//
// and §891:17480-17483 defines a letter the same way. A character whose \lccode
// is zero ENDS the word being gathered. This engine gathered every maximal run of
// charNodes instead, so "word," reached the hyphenator as a five-letter word and
// matched Liang patterns against a string no pattern file contains, and "don't"
// as one six-letter word rather than two short ones. go-tex/engine#765 item (3).
//
// ⚠ WHY THIS IS NOT A PDF COMPARISON. The obvious witness — a narrow measure,
// long words with and without commas, against tectonic — does NOT discriminate:
// it comes back identical before and after the change, because on that fixture a
// larger divergence dominates (our lines carry 114.8pt of ink on a 110pt measure
// where the reference stays inside it, in the paragraph WITHOUT any punctuation
// too). A witness that cannot tell the two states apart is evidence for neither,
// so what is asserted here is the thing the change decides: where the word ends.
func TestAWordEndsAtACharacterWhoseLccodeIsZero(t *testing.T) {
	for _, c := range []struct {
		name, text, want string
		after            rune
	}{
		{"comma", "word,", "word", ','},
		{"apostrophe", "don't", "don", '\''},
		{"digit", "x2y", "x", '2'},
		{"full stop", "end.", "end", '.'},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, end := wordAt(New(), c.text)
			if got != c.want {
				t.Errorf("the word gathered from %q is %q, want %q: \\lccode of %q is "+
					"zero and ends it (tex.web:17601)", c.text, got, c.want, string(c.after))
			}
			if end != c.after {
				t.Errorf("the word %q ended at %q, want %q", c.text, string(end), string(c.after))
			}
		})
	}
}

// ⛔ THE CONTROL, both halves of it.
//
// A plain word must still be gathered WHOLE — a letter test that ended every word
// at its first character would pass the table above and hyphenate nothing — and an
// accented word must survive, which is the deliberate divergence from 8-bit
// tex.web that lcCodeOf states: INITEX gives \lccode only to a-z and A-Z, so
// "éclair" would be cut at its first character and every French word with it.
func TestAPlainOrAccentedWordIsGatheredWhole(t *testing.T) {
	for _, text := range []string{"hyphenation", "éclair", "Extraordinarily"} {
		t.Run(text, func(t *testing.T) {
			if got, _ := wordAt(New(), text); got != text {
				t.Errorf("gathered %q from %q: every character here is a letter", got, text)
			}
		})
	}
}

// ⛔ AND THE DOCUMENT'S OWN TABLE DECIDES, in both directions. This is what makes
// the rule \lccode's rather than Unicode's: a document that zeroes a letter's
// \lccode takes it out of words, and one that gives a non-letter an \lccode puts
// it in. Without these two rows the implementation could consult Unicode alone
// and nothing would say so.
func TestTheDocumentsOwnLccodeDecides(t *testing.T) {
	e := New()
	if _, err := e.Run("\\lccode`\\'=`\\'"); err != nil {
		t.Fatal(err)
	}
	if got, _ := wordAt(e, "don't"); got != "don't" {
		t.Errorf("with \\lccode of ' set, the word is %q, want \"don't\": the "+
			"document's table wins over the Unicode default", got)
	}

	e2 := New()
	if _, err := e2.Run("\\lccode`\\e=0"); err != nil {
		t.Fatal(err)
	}
	if got, _ := wordAt(e2, "hyphenation"); got != "hyph" {
		t.Errorf("with \\lccode`\\e=0 the word is %q, want \"hyph\": a letter the "+
			"document un-letters ends the word", got)
	}
}
