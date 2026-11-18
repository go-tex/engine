// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strconv"
	"testing"
)

// headBoxOf declares \@oddhead and returns the box the page assembler would put
// at the top of the page.
func headBoxOf(t *testing.T, preamble, head string) *boxNode {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}` + preamble +
		`\makeatletter\def\@oddhead{` + head + `}\makeatother` +
		`\begin{document}x\end{document}`); err != nil {
		t.Fatal(err)
	}
	b := e.fancyHeader()
	if b == nil {
		t.Fatal("no head box was built, so nothing here measures anything")
	}
	return b
}

// ⛔⛔ THE RUNNING HEAD TRAVELS IN A \vbox to\headheight, AND ITS BASELINE IS
// THEREFORE FIXED.
//
// ltoutput.dtx:3350-3369, inside \@outputpage:
//
//	\setbox\@tempboxa \vbox to\headheight {%
//	  \vfil
//	  … \hb@xt@ \textwidth {\@thehead }%
//	}
//	\dp \@tempboxa \z@
//	\box \@tempboxa
//	\vskip \headsep
//
// A \vbox to h measures its height to the baseline of its LAST box, so the \vfil
// above puts the head's baseline exactly \headheight below the band's top
// WHATEVER the head contains. This engine emitted the line at its natural height,
// so the baseline sat at the height of the head's own glyphs and MOVED with them.
//
// Measured against tectonic before the fix — same band top (87.90pt) in both
// engines, article 10pt, \pagestyle{headings}, head baseline from the page top:
//
//	head text     reference   ours     deficit
//	1 ALFA 1      99.90       95.05    4.85
//	1 (QJPY) 1    99.90       95.39    4.51
//	1             99.90       94.55    5.35
//
// The reference is 99.90 all three times; each of our deficits is \headheight
// minus the height of that head's tallest glyph. go-tex/engine#822.
//
// ⚠ The mock face gives every glyph the same 7pt height, so content cannot vary
// it HERE; what this asserts instead is the thing that makes the baseline
// content-independent in the first place — that the box is \headheight tall and
// not the 7pt its contents are.
func TestTheHeadBoxIsHeadheightTallWhateverIsInIt(t *testing.T) {
	b := headBoxOf(t, "", `ALFA\hfil\thepage`)
	if want := 12 * unity; b.height != want {
		t.Errorf("head box height = %.2fpt, want %.2fpt (\\headheight): the glyphs "+
			"are 7pt tall, so a natural-height box is what the old code produced and "+
			"what moved the baseline with the text (ltoutput.dtx:3350)",
			spToPt(b.height), spToPt(want))
	}
}

// ⛔ THE HEIGHT COMES FROM THE REGISTER, not from a 12 written in the code. A
// constant would pass the test above and ignore every document that sets
// \headheight — which is most of them, since geometry.sty assigns it.
func TestTheHeadBoxFollowsHeadheight(t *testing.T) {
	for _, hh := range []int{30, 5} {
		b := headBoxOf(t, `\headheight=`+strconv.Itoa(hh)+`pt`, `ALFA\hfil\thepage`)
		if want := hh * unity; b.height != want {
			t.Errorf("with \\headheight=%dpt the head box is %.2fpt tall, want %.2fpt",
				hh, spToPt(b.height), spToPt(want))
		}
	}
}

// ⛔ \headheight=5pt IS THE INTERESTING ONE and it is in the table above: the
// head's glyphs are 7pt tall, so the box is OVERFULL. \vbox to\headheight makes
// it 5pt anyway — TeX reports an overfull box and sets it to the size asked for —
// and a reading that quietly took max(headheight, natural) would put the head
// 2pt lower on every page of a document with a small \headheight.
func TestASmallHeadheightIsObeyedNotWidened(t *testing.T) {
	b := headBoxOf(t, `\headheight=5pt`, `ALFA\hfil\thepage`)
	if want := 5 * unity; b.height != want {
		t.Errorf("head box height = %.2fpt with \\headheight=5pt and 7pt glyphs, "+
			"want %.2fpt: \\vbox to\\headheight is a demand, not a minimum",
			spToPt(b.height), spToPt(want))
	}
}

// ⛔ THE DEPTH IS ZEROED (\dp\@tempboxa\z@, ltoutput.dtx:3368), because \headsep
// is measured from the head's BASELINE and not from under its descenders. The
// mock gives every glyph 2pt of depth, so without this every page with a running
// head would push its text block 2pt down.
func TestTheHeadBoxHasNoDepth(t *testing.T) {
	if b := headBoxOf(t, "", `ALFA\hfil\thepage`); b.depth != 0 {
		t.Errorf("head box depth = %.2fpt, want 0: \\headsep runs from the baseline "+
			"(ltoutput.dtx:3368)", spToPt(b.depth))
	}
}

// ⛔⛔ AND THE HEAD IS SET TO THE FULL TEXT WIDTH, so the \hfil that \@oddhead
// itself contains can push the folio to the right margin.
//
// \@oddhead is `{\slshape\rightmark}\hfil\thepage` — ONE field with the \hfil
// INSIDE it. The old code typeset that field into its own natural-width box
// first, which set the \hfil to zero, and then packed that rigid box to \hsize,
// which just left it at the left edge. The running head came out as "1 ALFA1",
// the folio abutting the title, ending at x=180.35pt where the reference runs to
// 477.48pt.
//
// ⚠ fancyhdr's three-field path was never wrong, and that is the distinction
// worth keeping: there the \hfil's sit BETWEEN three separately-packed fields
// (fancyLine), so packing each one naturally is correct.
func TestTheHeadIsSetToTheFullTextWidth(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}` +
		`\makeatletter\def\@oddhead{ALFA\hfil\thepage}\makeatother` +
		`\begin{document}x\end{document}`); err != nil {
		t.Fatal(err)
	}
	b := e.fancyHeader()
	if b == nil {
		t.Fatal("no head box")
	}
	// [\vfil, line] — the \vfil is what holds the baseline down, so the line is last.
	if len(b.list) != 2 {
		t.Fatalf("head box holds %d items, want the \\vfil and the line", len(b.list))
	}
	line, ok := b.list[1].(*boxNode)
	if !ok {
		t.Fatalf("the head box's last item is a %T, want the line box", b.list[1])
	}
	// ⛔⛔ NOT `line.width == e.hsize` — THAT PASSES EITHER WAY AND PROVES NOTHING.
	// hpackSP(…, packTo, e.hsize) sets the OUTER box's width to \hsize whether or
	// not anything inside it stretched, so wrapping one natural-width box (the
	// defect) yields a line that is exactly \hsize wide with its content huddled at
	// the left. That assertion was written first, passed, and passed again against
	// a deliberately reintroduced defect; it measured the wrapper and not the
	// effect.
	//
	// What distinguishes the two states is whether \@oddhead's own \hfil is a
	// direct child of the line, able to receive the stretch. Pre-packing the field
	// buries it inside a natural-width box where it is already set to zero.
	var stretchy bool
	for _, n := range line.list {
		if g, ok := n.(glueNode); ok && g.spec.stretchOrder > 0 {
			stretchy = true
		}
	}
	if !stretchy {
		t.Errorf("the head line holds no infinitely stretchable glue of its own "+
			"(%d items): \\@oddhead's \\hfil was packed away inside a natural-width "+
			"box, so the folio sits beside the title instead of at the right margin "+
			"(ltoutput.dtx:3363, \\hb@xt@\\textwidth)", len(line.list))
	}
	if line.width != e.hsize {
		t.Errorf("the head line is %.2fpt wide, want \\hsize = %.2fpt",
			spToPt(line.width), spToPt(e.hsize))
	}
}
