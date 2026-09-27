// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \footnotemark and \footnotetext are the two halves of a footnote placed separately
// (latex.ltx:13202-13206 and :13219-13222). \footnotemark was undefined — 10 uses over 7
// corpus papers, and a hard stop for three of them — while \footnotetext was DEFINED as
//
//	\def\footnotetext{\@ifnextbracket\@gobbleoptarg\@gobble}
//
// a stub that swallowed the note whole. So the pair lost every note and reported nothing:
// the census counts what the engine does not HAVE, and a stub is something it has.
//
// Checked against tectonic on this witness. Both produce marks 1, 2, 3 inline at the same
// places and the three notes at the foot in that order.
func TestFootnoteMarkAndTextArePlacedSeparately(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		"Corps un\\footnotemark{} puis corps deux\\footnotemark{}.\n" +
		"\\footnotetext[1]{NOTEUNE}\n\\footnotetext[2]{NOTEDEUX}\n" +
		"Et une note ordinaire\\footnote{NOTETROIS}.\n" +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, cs := range []string{"footnotemark", "footnotetext"} {
		if n := e.Diagnostics().Skipped[cs]; n != 0 {
			t.Errorf("\\%s is undefined (%d skipped)", cs, n)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// ⛔ The note TEXT is the assertion. The stub left the marks' numbers behind and only
	// the bodies went missing, so a test that checked the marks would have passed.
	for _, want := range []string{"NOTEUNE", "NOTEDEUX", "NOTETROIS"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q never reached the page; it reads %q", want, firstN(text, 140))
		}
	}
	// ⛔ And the ORDER of the counter. \footnotemark steps it, \footnotetext does not — it
	// pairs with a mark that already stepped. Backwards, every later note is numbered one
	// too high, so the plain \footnote after two marks must be 3 and not 5.
	i1, i2, i3 := strings.Index(text, "NOTEUNE"), strings.Index(text, "NOTEDEUX"), strings.Index(text, "NOTETROIS")
	if !(i1 < i2 && i2 < i3) {
		t.Errorf("the notes are out of order: NOTEUNE@%d NOTEDEUX@%d NOTETROIS@%d", i1, i2, i3)
	}
	if !strings.Contains(text, "3. NOTETROIS") {
		t.Errorf("the plain \\footnote after two \\footnotemark is not numbered 3 — "+
			"\\footnotetext stepped the counter when it must not; it reads %q", firstN(text, 140))
	}
}

// The bracketless form takes the CURRENT counter, which is what a \footnotemark just set.
// Checked against tectonic: both give mark 1 with SANSCROCHET as note 1, then 2/APRES.
func TestFootnoteTextWithoutABracketUsesTheCurrentNumber(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		"Texte\\footnotemark{}.\n\\footnotetext{SANSCROCHET}\n" +
		"Suite\\footnote{APRES}.\n" + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"1. SANSCROCHET", "2. APRES"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 120))
		}
	}
}
