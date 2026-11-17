// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"testing"
)

// glyphFaces reports the face each letter was SET with, first occurrence wins —
// the weight/shape counterpart of glyphSizes (fontsize_test.go:310).
//
// charNode.face exists because metrics being right is not enough: the drivers
// used to carry one face for the whole document and redraw every glyph in it, so
// \textbf came back roman (#590). It is therefore the observable for "was this
// glyph bold", and it is exact where a width comparison would be approximate.
func glyphFaces(e *Engine) map[rune]fontFace {
	faces := map[rune]fontFace{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				if _, seen := faces[c.ch]; !seen {
					faces[c.ch] = c.face
				}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return faces
}

// captionEngine builds an engine with a real roman and a real bold face, because
// \bf is only bound to anything when a bold font is supplied (api.go:21) — with
// none, every face is the same object and this question cannot be asked.
func captionEngine(t *testing.T, src string) map[rune]fontFace {
	t.Helper()
	const reg = "/System/Library/Fonts/Supplemental/Georgia.ttf"
	const bold = "/System/Library/Fonts/Supplemental/Georgia Bold.ttf"
	rb, err := os.ReadFile(reg)
	if err != nil {
		t.Skip("no system roman font")
	}
	bb, err := os.ReadFile(bold)
	if err != nil {
		t.Skip("no system bold font")
	}
	e, err := buildEngine(Options{Font: rb, BoldFont: bb, Size: 11}, true)
	if err != nil {
		t.Fatal(err)
	}
	if m := e.eq["bf"]; m == nil || m.kind != mFont {
		t.Fatalf("\\bf is not bound to a font (%+v), so this test cannot tell a bold "+
			"label from a roman one and would pass for the wrong reason", e.eq["bf"])
	}
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return glyphFaces(e)
}

// ⛔⛔ A CAPTION LABEL IS NOT BOLD, AND NOT ONE OF THE CLASSES THIS ENGINE EMBEDS
// MAKES IT SO.
//
// article.cls's \@makecaption is the whole font decision and it makes none
// (classes.dtx:3554-3562): \vskip\abovecaptionskip then \sbox\@tempboxa{#1: #2}.
//
// Censused in texmf/, because "maybe we are accidentally right for the
// class-styled case" has to be counted rather than assumed:
//
//	article.cls:482   no font
//	book.cls:568      no font
//	report.cls        no font
//	amsart.cls:1376   \@captionfont\@captionheadfont, and :1341 is \scshape
//
// Three set nothing and the fourth sets SMALL CAPS. Bold was not even
// accidentally right for one of them.
//
// Measured against tectonic, article at 11pt, a figure[h!] holding a rule:
//
//	reference   x 226.36 – 383.89   LMRoman10-Regular
//	before      x 223.09 – 387.19   LMRoman10-Bold      (+6.57pt, and centred)
//	after       x 226.36 – 383.91   LMRoman10-Regular   (0.02pt from the reference)
//
// ⚠ AND NOTHING IN THIS SUITE CAUGHT IT. The whole suite was green with the label
// bold and is green with it roman. The weight had no witness at all, which is how
// it stayed wrong — so this file is the gap as much as the fix.
func TestACaptionLabelIsSetInTheBodyFace(t *testing.T) {
	// B is body text; F comes from the label ("Figure 1:") and Z from the caption
	// text, so the two halves of the caption are read separately.
	faces := captionEngine(t, `\noindent B\begin{figure}\caption{Z}\end{figure}`)
	if faces['B'] == nil {
		t.Fatal("the body letter was never set; this test cannot say anything")
	}
	for _, r := range []rune{'F', 'Z'} {
		if faces[r] == nil {
			t.Fatalf("the caption's %q was never set: the fixture is wrong, not the rule", r)
		}
		if faces[r] != faces['B'] {
			t.Errorf("caption glyph %q was set in a different face from the body — a "+
				"caption takes the ambient font (classes.dtx:3554-3562), and none of "+
				"the four classes in texmf/ bolds its label", r)
		}
	}
}

// ⛔ THE CONTROL, and it is what makes the rule above a rule rather than "the bold
// face is unreachable here". \captionlabelfont is caption.sty's own name, and a
// document that asks for a bold label must still get one — which is how a class
// that wants one says so, since this engine does not run \@makecaption (#281).
func TestACaptionLabelFollowsCaptionLabelFontWhenAskedFor(t *testing.T) {
	faces := captionEngine(t, `\renewcommand\captionlabelfont{\bf}`+
		`\noindent B\begin{figure}\caption{Z}\end{figure}`)
	if faces['F'] == faces['B'] {
		t.Error("\\captionlabelfont{\\bf} left the label in the body face: the knob does " +
			"nothing, so removing the hard-coded \\bf took the capability with it")
	}
	// ⛔ And ONLY the label. A hook that reached the caption's TEXT as well would be
	// a different defect wearing this one's fix.
	if faces['Z'] != faces['B'] {
		t.Error("\\captionlabelfont reached the caption TEXT as well as its label")
	}
}
