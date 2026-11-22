// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// beamerBandClass replays, in order, what real beamer does to the head band:
//
//   - asks geometry for head=0.5cm and headsep=0pt (beamer.cls:355-362);
//   - \input's LaTeX's size11.clo (beamer.cls:375), which sets \headheight=12pt
//     and \headsep=25pt;
//   - calls \geometry{left=…} (beamerbaseframecomponents.sty:39), and geometry
//     re-asserts its head= and headsep= keys over size11.clo's values;
//   - sets \headheight to its headline's height at \begin{document}, 0pt for the
//     default theme (beamerbaseframecomponents.sty:169), inside the document group.
const beamerBandClass = `\RequirePackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm,head=0.5cm,headsep=0pt,foot=0.5cm]{geometry}
\setlength\headheight{12pt}\setlength\headsep{25pt}
\geometry{left=1cm}
\AtBeginDocument{\headheight=0pt }
\makeatletter\def\ps@zzhead{\def\@oddhead{\hfil}\def\@evenhead{\hfil}\def\@oddfoot{}\def\@evenfoot{}}\makeatother
\pagestyle{zzhead}
`

func runBeamerBands(t *testing.T, body string) (*Engine, string) {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
		if name == "beamer.cls" {
			return []byte(beamerBandClass), true
		}
		return nil, false
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{beamer}` + body)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return e, out
}

// geometry's head= and headsep= keys hold through a later \geometry call, as
// \Gm@process re-applies them (geometry.sty:750-752). They were lost to the
// 12pt/25pt a size file had set in between.
func TestALaterGeometryCallReassertsTheHeadBand(t *testing.T) {
	_, out := runBeamerBands(t, `\message{[\the\headheight/\the\headsep]}\begin{document}x\end{document}`)
	if want := "[14.22636pt/0.0pt]"; out != want {
		t.Errorf("preamble band = %q, want %q", out, want)
	}
}

// Only geometry makes a zero band register a real layout. Without it, a class the
// engine emulates may never have set the register, so \end{document} records
// nothing and a zero still reads as unset: article's 12pt and 25pt stand in.
func TestWithoutGeometryAZeroBandIsStillUnset(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\pagestyle{headings}\begin{document}\headheight=0pt \headsep=0pt x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.geom != nil {
		t.Fatal("geometry ran; this test is about a document without it")
	}
	if e.shipBands != nil {
		t.Errorf("shipBands = %v, want nothing recorded without geometry", e.shipBands)
	}
	if got, want := e.headBand(), 37*unity; got != want {
		t.Errorf("headBand = %.2fpt, want %.2fpt (12pt + 25pt defaults)", spToPt(got), spToPt(want))
	}
}

// A register \end{document} did not record still falls back to the default.
func TestBandDimenFallsBackForAnUnrecordedRegister(t *testing.T) {
	e, _ := runBeamerBands(t, `\begin{document}x\end{document}`)
	if e.geom == nil {
		t.Fatal("geometry did not run")
	}
	if got := e.bandDimen("zz@notallocated", 7*unity); got != 7*unity {
		t.Errorf("bandDimen(unallocated) = %.2fpt, want the 7pt default", spToPt(got))
	}
}

// The page is assembled with the band as the BODY left it — 0pt, beamer's own
// assignment inside the document group — and not with geometry's global 14.23pt
// that the group's end restores. The head box was 14.23pt tall and every frame's
// body sat 14pt low (go-tex/engine#924).
func TestABeamerPageUsesTheBandTheBodyLeft(t *testing.T) {
	e, _ := runBeamerBands(t, `\begin{document}x\end{document}`)
	if got := e.headBand(); got != 0 {
		t.Errorf("headBand = %.2fpt, want 0", spToPt(got))
	}
	if got := e.renderVMargin(72); got != 0 {
		t.Errorf("renderVMargin = %.2fpt, want 0 (vmargin=0cm, no band)", got)
	}
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("no page")
	}
	if b, ok := pages[0].list[0].(*boxNode); ok && b.height != 0 {
		t.Errorf("the page opens with a %.2fpt box, want the 0pt head", spToPt(b.height))
	}
}
