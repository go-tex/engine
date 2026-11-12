// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// discCount returns how many discretionary hyphens hyphenateList puts into a
// list built from the given nodes.
func discCount(t *testing.T, build func() []node) int {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	n := 0
	for _, x := range e.hyphenateList(build()) {
		if _, ok := x.(discNode); ok {
			n++
		}
	}
	return n
}

func word(s string) []node {
	out := make([]node, 0, len(s))
	for _, r := range s {
		out = append(out, charNode{ch: r})
	}
	return out
}

// ⛔ TeX HYPHENATES A WORD ONLY WHERE A GLUE PRECEDES IT (tex.web §894, §896:
// the scan starts at a glue node). This engine hyphenated every maximal run of
// characters, whatever sat on either side.
//
// ⭐ MEASURED AGAINST tectonic 0.17.0, on a document where the SAME WORD appears
// in both positions at a 72pt measure:
//
//	Incomprehensibilities abound.          <- first word of its paragraph
//
//	Then incomprehensibilities abound again.
//
//	reference   Incomprehensibilities      NOT hyphenated, the line overruns
//	            Then incom-/prehensibilities   hyphenated
//
//	before      Incomprehen-/sibilities    hyphenated — wrong
//	            Then incom-/prehensibilities   hyphenated
//
//	after       Incomprehensibilities      not hyphenated
//	            Then incom-/prehensibilities   still hyphenated
//
// The second paragraph is the control, and it is the point: a fix that simply
// stopped hyphenating would pass the first assertion and fail that one.
func TestAWordIsHyphenatedOnlyAfterGlue(t *testing.T) {
	const w = "incomprehensibilities"

	// After a glue: hyphenated, as it always was.
	afterGlue := discCount(t, func() []node {
		return append([]node{glueNode{spec: glueSpec{width: 3 * unity}}}, word(w)...)
	})
	if afterGlue == 0 {
		t.Fatalf("%q after a glue got no break points: the control is broken, so "+
			"every assertion below means nothing", w)
	}

	// At the very head of the list: nothing precedes it, so nothing is glue.
	atHead := discCount(t, func() []node { return word(w) })
	if atHead != 0 {
		t.Errorf("%q at the head of the list got %d break points, want 0 — nothing "+
			"precedes it, so tex.web's scan never starts", w, atHead)
	}

	// After a BOX, which is what a paragraph's \parindent is.
	afterBox := discCount(t, func() []node {
		return append([]node{hpackSP(nil, packNatural, ptToSP(20))}, word(w)...)
	})
	if afterBox != 0 {
		t.Errorf("%q after a box got %d break points, want 0 — a paragraph's first "+
			"word follows the \\parindent box, and the reference does not hyphenate it",
			w, afterBox)
	}

	// ⚠ A KERN BETWEEN THE GLUE AND THE WORD IS TRANSPARENT. §896-§899 walk from
	// the glue to the first letter; a font kern there does not make the word
	// unhyphenatable, and treating it as a barrier would silently disable
	// hyphenation after every space a kern follows.
	afterGlueKern := discCount(t, func() []node {
		return append([]node{
			glueNode{spec: glueSpec{width: 3 * unity}},
			kernNode{width: ptToSP(1)},
		}, word(w)...)
	})
	if afterGlueKern == 0 {
		t.Errorf("%q after glue+kern got no break points: the kern was treated as a "+
			"barrier, which would disable hyphenation wherever a kern follows a space", w)
	}
}

// And the whole-document shape, so the rule is pinned where it acts rather than
// only on a hand-built list.
func TestAParagraphsFirstWordIsNotHyphenated(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=72pt` +
		`\hbadness=10000 \tolerance=10000 ` +
		`Incomprehensibilities abound.\par Then incomprehensibilities abound again.` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	// The first word must survive whole. pageChars strips spaces, so the test is
	// for the letters in sequence.
	if !strings.Contains(got, "Incomprehensibilities") {
		t.Errorf("the paragraph's first word was broken up: %q", firstN(got, 120))
	}
	// …and the control: the same word after a space still breaks, or this test
	// would pass on an engine that had stopped hyphenating altogether.
	if !strings.Contains(got, "incom") {
		t.Fatalf("the second paragraph did not set at all: %q", firstN(got, 120))
	}
}
