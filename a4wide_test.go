// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// a4wide.sty (J.-F. Lamy, loading NTG's a4.sty) sets the page geometry through
// \textwidth, \oddsidemargin and \textheight, and nothing emulated it: about 140
// corpus files loaded it and kept the class margins, so paper 2111.13456 ran 36
// pages against 29. The two real files are bundled now (texmf/a4wide.sty,
// texmf/a4.sty) and the engine runs them.
//
// The expected values are what tectonic 0.15 (XeTeX) prints for
// \documentclass[10pt]{amsart}\usepackage{a4wide}: textwidth=424.58624pt,
// textheight=646.0pt, oddsidemargin=18.06749pt, topmargin=0.0pt
// (a4wide.sty:24, a4.sty:49-57: 5.875in, 53\baselineskip+\topskip, 0.25in).
func TestA4wideSetsTheTextBlock(t *testing.T) {
	e := runGeom(t, `\documentclass[10pt]{article}\usepackage{a4wide}\begin{document}`)
	if want := texSP(t, "424.58624pt"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (tectonic: 424.58624pt)", e.hsize, want)
	}
	if want := texSP(t, "646pt"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (tectonic: 646.0pt)", e.vsize, want)
	}
	// The left edge of the text is 1in (TeX's origin) + \oddsidemargin; tectonic's
	// page puts it at 90.00bp, and so does this engine's PDF (checked with
	// pdftotext -bbox: first word at x 101.96 in both, after the 12pt parindent).
	if got, want := e.renderMargin(72), spToPt(texSP(t, "1in")+texSP(t, "18.06749pt")); got < want-0.01 || got > want+0.01 {
		t.Errorf("left edge = %v, want %v (tectonic: 90.00bp)", got, want)
	}
}

// The 11pt and 12pt branches of \ifcase\@ptsize (a4wide.sty:25-40).
func TestA4wideFollowsTheClassSize(t *testing.T) {
	for _, c := range []struct{ size, width string }{
		{"11pt", "6.125in"}, {"12pt", "6.375in"},
	} {
		e := runGeom(t, `\documentclass[`+c.size+`]{article}\usepackage{a4wide}\begin{document}`)
		if want := texSP(t, c.width); e.hsize != want {
			t.Errorf("%s: hsize = %d, want %d (%s)", c.size, e.hsize, want, c.width)
		}
	}
}
