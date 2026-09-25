// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A paper-size class option has to reach \paperwidth/\paperheight, not only
// geometry's defaults.
//
// The standard classes set those registers from the option, and a document that
// loads anything pulling in the graphics driver — graphicx, color, xcolor, tikz,
// siunitx, mathtools — then gets the value written to the media box. That is every
// real paper. An EMULATED class loads no .cls, so nothing did it:
//
//	\documentclass[a4paper,twocolumn]{revtex4-2} + \usepackage{graphicx}
//	   tectonic 595x842 (A4)        this engine 612x792 (letter)
//
// while the same document WITHOUT a4paper agrees at letter on both sides — so it
// was the option being dropped, not the sheet being guessed.
func TestClassPaperOptionReachesPaperwidth(t *testing.T) {
	for _, c := range []struct {
		src          string
		wantW, wantH float64 // inches
	}{
		{`\documentclass[a4paper,twocolumn]{revtex4-2}`, 210 / 25.4, 297 / 25.4},
		{`\documentclass[twocolumn]{revtex4-2}`, 8.5, 11},
		{`\documentclass[a4paper]{article}`, 210 / 25.4, 297 / 25.4},
		{`\documentclass[a5paper]{article}`, 148 / 25.4, 210 / 25.4},
		{`\documentclass{article}`, 8.5, 11},
		// A class that states its OWN sheet still wins: acmart.cls forces
		// 6.75in x 10in for acmsmall whatever paper option the document passed.
		{`\documentclass[acmsmall,a4paper]{acmart}`, 6.75, 10},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(c.src + `\begin{document}X\end{document}`); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		w, h, ok := e.paperSizePt()
		if !ok {
			t.Errorf("%s: no paper size", c.src)
			continue
		}
		if got := w / 72.27; got < c.wantW-0.02 || got > c.wantW+0.02 {
			t.Errorf("%s: width %.3fin, want %.3fin", c.src, got, c.wantW)
		}
		if got := h / 72.27; got < c.wantH-0.02 || got > c.wantH+0.02 {
			t.Errorf("%s: height %.3fin, want %.3fin", c.src, got, c.wantH)
		}
	}
}
