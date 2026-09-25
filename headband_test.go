// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A running head sits ABOVE the text block, and the band it takes is exactly
// \headheight + \headsep. classes.dtx, "Page Layout": all margins are measured
// from a point one inch from the top, and the text starts at
//
//	1in + \voffset + \topmargin + \headheight + \headsep
//
// so the two lengths between \topmargin and the text ARE the head's band. The
// margin accessors answer where the TEXT starts, and a page assembled with a head
// begins with the head — so the page is lifted by the band it carries, and the box
// is packed to \vsize plus the band rather than taking it out of the text.
//
// Measured against the reference on corpus paper 2405.18549 page 3, first ink
// block: ours sat at 72pt from the top with 10pt under it, the reference at 37pt
// with 31pt. After: 33pt and 29pt.
func TestHeadBandIsHeadheightPlusHeadsep(t *testing.T) {
	// No head: no band, whatever the page style.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	if got := e.headBand(); got != 0 {
		t.Errorf("no page style: band = %d sp, want 0", got)
	}

	// \pagestyle{fancy} with a head field: the band is the two lengths.
	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\documentclass{article}\usepackage{fancyhdr}\pagestyle{fancy}` +
		`\chead{TETE}\begin{document}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	if got, want := e2.headBand(), 37*unity; got != want {
		t.Errorf("band = %d sp, want %d (\\headheight 12pt + \\headsep 25pt)", got, want)
	}

	// fancy with NO head field draws no head, so it takes no band — the footer
	// alone must not lift the page.
	e3 := New()
	if err := e3.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e3.SetFont(spMock{})
	if _, err := e3.Run(`\documentclass{article}\usepackage{fancyhdr}\pagestyle{fancy}` +
		`\cfoot{\thepage}\begin{document}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	if got := e3.headBand(); got != 0 {
		t.Errorf("footer only: band = %d sp, want 0", got)
	}
}

// headBand must not TYPESET anything. It used to call fancyHeader(), which builds
// the header line, and pdfdriver.go reads the vertical margin BEFORE paginating —
// so a geometry question was answered by running the typesetter.
func TestHeadBandDoesNotTypeset(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\usepackage{fancyhdr}\pagestyle{fancy}` +
		`\chead{TETE}\begin{document}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	before := len(e.mvl)
	for i := 0; i < 5; i++ {
		e.headBand()
	}
	if after := len(e.mvl); after != before {
		t.Errorf("the vertical list went from %d to %d nodes: headBand typeset something",
			before, after)
	}
}
