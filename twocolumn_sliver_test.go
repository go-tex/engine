// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The columns under a tall \twocolumn[...] span are filled whenever LaTeX fills them.
// latex.ltx \@topnewpage ends with
//
//	\ifdim \@colht<2.5\baselineskip  % warning "Optional argument of \twocolumn too tall", empty columns
//	\else \global\vsize\@colht \global\@colroom\@colht \@floatplacement \fi
//
// so the only refusal is under 2.5\baselineskip, not under a tenth of the page. Witness
// d1 (unblock-outliers-2.md): span of ~480pt plus two short paragraphs and a \newpage;
// tectonic sets the paragraphs under the span on page 1, we threw the sliver away and
// shipped 2 pages.
func TestTwocolumnSliverUnderSpanIsFilled(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	src := `\documentclass[twocolumn]{article}
\begin{document}
\twocolumn[{\centering SPANTITLE\par\vskip 470pt SPANEND\par\vskip 10pt}]
SLIVERA

SLIVERB
\newpage
\section{Introduction}
BODYSTART text.
\end{document}
`
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if got := len(e.Pages()); got != 1 {
		t.Errorf("pages = %d, tectonic gives 1 (\\newpage ends the left column)", got)
	}
}

// The edge of the rule, \@colht = 2.5\baselineskip exactly: a span that is the whole
// \textheight (550pt) less 30pt leaves 30pt = 2.5 x 12pt, and the columns are filled (the
// test is \@colht < 2.5\baselineskip); one point more and they are not, so the text
// moves to a second page.
func TestTwocolumnFillFloorEdge(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	for _, c := range []struct {
		spanPt string
		pages  int
	}{{"520", 1}, {"521", 2}} {
		src := `\documentclass[twocolumn]{article}
\begin{document}
\twocolumn[{\vbox to ` + c.spanPt + `pt{}}]
SLIVERA
\end{document}
`
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		if got := len(e.Pages()); got != c.pages {
			t.Errorf("span of %spt: %d pages, want %d", c.spanPt, got, c.pages)
		}
	}
}

func TestColumnsFillFloor(t *testing.T) {
	if got, want := columnsFillFloor(12*unity), 30*unity; got != want {
		t.Errorf("columnsFillFloor(12pt) = %d, want %d (2.5 baselineskip)", got, want)
	}
}
