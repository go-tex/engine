// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ A FONT WHOSE \hyphenchar IS NEGATIVE IS NEVER HYPHENATED (tex.web §896:
// `if hyf_char<0 then goto done1`), and tulmtt.fd:51 sets exactly that for the
// typewriter family.
//
// ⭐ MEASURED AGAINST tectonic 0.17.0: the same word at a 72pt measure, in two
// faces, the roman one being the control.
//
//	Word incom-            roman: hyphenated
//	prehensibilities
//	here.
//	   Word                \texttt: NOT hyphenated, the line overruns
//	incomprehensibilities
//	here.
//
// The roman half is asserted here too. Without it a fix that simply stopped
// hyphenating everything would pass.
func TestTypewriterIsNotHyphenatedAndRomanStillIs(t *testing.T) {
	const body = `\hsize=72pt\hbadness=10000 \tolerance=10000 `

	roman, err := compile([]byte(`\documentclass{article}\begin{document}`+body+
		`Word incomprehensibilities here.\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// pageChars strips spaces, so a hyphen in the text is the discretionary that
	// was taken.
	if !strings.Contains(pageChars(roman), "-") {
		t.Fatalf("the roman control was not hyphenated at a 72pt measure, so the "+
			"comparison below measures nothing: %q", firstN(pageChars(roman), 160))
	}

	tt, err := compile([]byte(`\documentclass{article}\begin{document}`+body+
		`Word \texttt{incomprehensibilities} here.\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(tt)
	if strings.Contains(got, "-") {
		t.Errorf("\\texttt was hyphenated: %q\n"+
			"tulmtt.fd:51 sets \\hyphenchar\\font\\m@ne for the typewriter family, and "+
			"tex.web §896 refuses to hyphenate a font whose \\hyphenchar is negative",
			firstN(got, 160))
	}
	// …and the word is still there, whole: withholding break points must not
	// withhold the word.
	if !strings.Contains(got, "incomprehensibilities") {
		t.Errorf("the \\texttt word did not reach the page at all: %q", firstN(got, 160))
	}
}

// The primitive itself, so a document or a class can say it.
func TestHyphencharIsAssignablePerFont(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	base := e.curFont
	if base == nil {
		t.Fatal("no current font: this test would measure nothing")
	}
	if got := e.hyphenCharOf(base); got < 0 {
		t.Fatalf("the roman face starts unhyphenatable (%d), so the assignment "+
			"below could not be told from the starting state", got)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}\hyphenchar\font=-1 x\end{document}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := e.hyphenCharOf(base); got >= 0 {
		t.Errorf("after \\hyphenchar\\font=-1 the face reports %d, want negative", got)
	}
}

// ⛔ AN UNSET FACE MUST KEEP HYPHENATING. \defaulthyphenchar is an
// accept-and-ignore parameter here whose INITEX value is -1; falling back to it
// would switch hyphenation off for every face in every document — fidelity in
// appearance, a corpus-wide regression in fact.
func TestAnUnsetFaceStillHyphenates(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := e.hyphenCharOf(e.curFont); got < 0 {
		t.Errorf("an unset face reports %d: it must permit hyphenation, not consult "+
			"the half-implemented \\defaulthyphenchar", got)
	}
	if got := e.hyphenCharOf(nil); got < 0 {
		t.Errorf("a nil face reports %d, want permitted", got)
	}
}
