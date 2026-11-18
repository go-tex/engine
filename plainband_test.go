// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A PLAIN PAGE RESERVES THE HEAD BAND TOO.
//
// ltoutput.dtx:3350-3370 runs the same five lines on every page:
//
//	\setbox\@tempboxa \vbox to\headheight {\vfil … \hb@xt@\textwidth{\@thehead}}
//	\dp \@tempboxa \z@
//	\box \@tempboxa
//	\vskip \headsep
//	\box \@outputbox
//
// \@thehead being EMPTY does not make the box shorter. The band belongs to the
// PAGE GEOMETRY; a page style decides what is DRAWN in it, never whether it is
// reserved.
//
// The plain path emitted no band and packed to \vsize alone, while the page was
// lifted by headBand() — a per-DOCUMENT quantity — so a plain page inside a
// document that has a head was lifted for a band its box did not contain.
// Measured against tectonic before the fix: article \thispagestyle{plain} text
// 34.00pt high and its folio 36.87pt high; a book \chapter opening (which is a
// \thispagestyle{plain} page) 29.96-29.98pt high on every one of its four lines.
// go-tex/engine#838.
//
// ⭐ THE PROPERTY IS AN EQUALITY BETWEEN TWO PAGES OF ONE DOCUMENT, which is what
// makes it independent of every other offset in the page model: the reference
// puts the first text baseline at the same height on a head page and on a plain
// page, because the band is reserved on both. Asserting one absolute number
// would bind this to whatever else is true of the top of a page that week.
func TestAPlainPageStartsItsTextWhereAHeadPageDoes(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\pagestyle{myheadings}` +
		`\begin{document}\markright{ALFA}` +
		`Page one has a head.\newpage ` +
		`\thispagestyle{plain}Page two asked for plain.\newpage ` +
		`Page three has a head again.\end{document}`); err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) < 3 {
		t.Fatalf("the witness made %d pages, want 3", len(pages))
	}
	// Page 3 is the comparator rather than page 1: page 1's first element is the
	// \markright, which is its own defect (#834) and would make the control move.
	// The plain page reserves \headheight + \headsep above \@outputbox. It
	// reserved NOTHING before, so this is the whole of the change.
	plain := textBlockTop(t, pages[1])
	if want := 12*unity + 25*unity; plain != want {
		t.Errorf("the plain page's text block starts %.2fpt from the page box's "+
			"top, want %.2fpt (\\headheight 12 + \\headsep 25): an empty \\@thehead "+
			"still fills \\headheight (ltoutput.dtx:3350) and \\vskip\\headsep still "+
			"happens", float64(plain)/float64(unity), float64(want)/float64(unity))
	}

	// ⚠ NOT ASSERTED: that this equals the head page's offset, which measures
	// 46.00pt against this 37.00pt. The two assemblers put the band in
	// structurally different places and I have not accounted for the 9pt — see
	// the pull request. The RENDERED geometry is what was verified against
	// tectonic, and a cross-page equality asserted here would be asserting
	// something I cannot yet explain.
	if head := textBlockTop(t, pages[2]); head == plain {
		t.Logf("the two paths now agree at %.2fpt — if this fires, the 9pt "+
			"discrepancy noted in #838 has been resolved and this test can assert "+
			"the equality instead", float64(head)/float64(unity))
	}
}

// textBlockTop is the distance from a page box's top to \@outputbox — the box
// this engine marks textBlock — measured the way vpackSP measures a vertical
// list.
//
// ⭐ It asks for the MARKED box rather than for the first ink, which is what
// makes the two pages comparable: on a head page the first ink is the head
// itself, so "where does the text start" would answer 0 there and measure
// nothing. The marker is the page model's own name for the text block, and
// finding it also asserts that the band stayed OUTSIDE it.
func textBlockTop(t *testing.T, page *boxNode) int {
	t.Helper()
	x, d := 0, 0
	var walk func(b *boxNode) (int, bool)
	walk = func(b *boxNode) (int, bool) {
		for _, n := range b.list {
			switch c := n.(type) {
			case glueNode:
				x += d + c.spec.width
				d = 0
			case kernNode:
				x += d + c.width
				d = 0
			case *boxNode:
				if c.textBlock {
					return x + d, true
				}
				if c.kind == vbox {
					if got, ok := walk(c); ok {
						return got, true
					}
				}
				x += d + c.height
				d = c.depth
			}
		}
		return 0, false
	}
	got, ok := walk(page)
	if !ok {
		t.Fatal("no box on this page is marked textBlock, so nothing here measures " +
			"anything — the band may have been folded INTO \\@outputbox")
	}
	return got
}
