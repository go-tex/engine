// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// elsarticle states its sheet per JOURNAL TYPE, not once for the class.
// elsarticle.cls:1222-1270 wraps each of 1p/3p/5p in
//
//	\RequirePackage{geometry}
//	\geometry{twoside, paperwidth=210mm, paperheight=297mm,
//	          textheight=622pt, textwidth=468pt, …}          % jtype=3
//
// so those three are A4 and geometry writes it to the media box. Without one of
// them the class requires geometry at all: nothing writes a media box and the
// sheet stays US letter, even though \paperwidth is A4 from
// \ExecuteOptions{a4paper,…} at :109. Asked directly, tectonic answers
//
//	elsarticle[final,12pt]              612 x 792   (letter)
//	elsarticle[preprint,11pt,3p,review] 595 x 842   (A4)
//
// which is the pair this table has to tell apart.
func TestElsarticleJournalTypesAreA4(t *testing.T) {
	for _, c := range []struct {
		opts  string
		wantW float64 // inches; 0 = the engine's default sheet
	}{
		{"1p", 210 / 25.4},
		{"3p", 210 / 25.4},
		{"5p", 210 / 25.4},
		{"preprint,11pt,3p,review", 210 / 25.4},
		{"final,12pt", 0},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass[` + c.opts + `]{elsarticle}\begin{document}X\end{document}`); err != nil {
			t.Fatalf("%s: %v", c.opts, err)
		}
		w, _, ok := e.paperSizePt()
		if !ok {
			t.Errorf("%s: no paper size", c.opts)
			continue
		}
		if c.wantW == 0 {
			continue // the no-journal-type case is the media-box question, see #419
		}
		if got := w / 72.27; got < c.wantW-0.01 || got > c.wantW+0.01 {
			t.Errorf("[%s]: paper width %.3fin, want %.3fin (210mm)", c.opts, got, c.wantW)
		}
	}
}

// A journal type states no leading — those types keep \baselinestretch at 1, so
// the size option's own leading is right. applyClassGeometry must therefore leave
// \baselineskip alone when the format carries none, or every page collapses.
func TestElsarticleKeepsItsLeading(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass[3p]{elsarticle}\begin{document}X\end{document}`); err != nil {
		t.Fatal(err)
	}
	if e.baselineskip <= 0 {
		t.Errorf("baselineskip = %d sp after a format that states none", e.baselineskip)
	}
}
