// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"testing"
)

// lineGlyphSizes compiles src and returns the size of the first glyph of every
// text line, in page order (the folio comes last).
func lineGlyphSizes(t *testing.T, src string) []float64 {
	t.Helper()
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var sizes []float64
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && directChar(b) {
				if ch, ok := firstGlyph(b); ok {
					sizes = append(sizes, ch.face.sizePt())
				}
				continue
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	return sizes
}

// \fontsize{17bp}{22.5bp} is a <dimen> in big points: \set@fontsize reads
// \@defaultunits\@tempdimb#2pt\relax\nnil, which scans a whole dimension and drops
// the trailing "pt" (latex.ltx:8533-8548), and tex.web §458 gives 1bp = 72.27/72pt.
// springer's sn-jnl (arxiv.cls:465) states its title as exactly this. The parser
// refused every unit but pt, so the size was thrown away and the title came out in
// body size; tectonic sets the witness at 17.00bp.
func TestFontsizeReadsBigPoints(t *testing.T) {
	sizes := lineGlyphSizes(t, `\documentclass{article}
\begin{document}
{\fontsize{17bp}{22.5bp}\selectfont Title}
\end{document}
`)
	if len(sizes) < 1 {
		t.Fatal("no text line")
	}
	if want := 17 * 72.27 / 72; math.Abs(sizes[0]-want) > 0.01 {
		t.Errorf("\\fontsize{17bp} set at %.3fpt, want %.3fpt", sizes[0], want)
	}
}

// A class that restates \normalsize states the body size: sn-jnl does
// \@setfontsize\normalsize{10bp}{12bp} over \LoadClass[11pt]{article}
// (arxiv.cls:57,160), and tectonic sets the whole document at 10bp, with the
// \large/\Large... table taken from that body. The engine took the base only from
// the class option, so the body stayed at 10.95pt. The size here is plain points so
// the witness does not depend on the unit parser.
func TestClassNormalsizeRestatedBecomesTheBody(t *testing.T) {
	sizes := lineGlyphSizes(t, `\documentclass[11pt]{article}
\makeatletter
\renewcommand\normalsize{\@setfontsize\normalsize{10}{12}}
\makeatother
\begin{document}
\normalsize Body text.\par
\end{document}
`)
	if len(sizes) < 1 {
		t.Fatal("no text line")
	}
	if math.Abs(sizes[0]-10) > 0.01 {
		t.Errorf("body set at %.3fpt after \\normalsize{10}, want 10pt", sizes[0])
	}
}
