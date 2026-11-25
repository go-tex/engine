// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

// spXY is the x and baseline of the first <tspan> whose text is word; ok false when
// the page does not draw it.
func spXY(svg, word string) (x, y float64, ok bool) {
	y, ok = wordY(svg, word)
	if !ok {
		return 0, 0, false
	}
	// The x sits in the same tag as the y; find the tag that holds this word.
	i := strings.Index(svg, ">"+word+"</tspan>")
	if i < 0 {
		return 0, y, true
	}
	open := strings.LastIndex(svg[:i], "<tspan ")
	tag := svg[open:i]
	if j := strings.Index(tag, ` x="`); j >= 0 {
		rest := tag[j+4:]
		if k := strings.Index(rest, `"`); k >= 0 {
			if v, err := strconv.ParseFloat(rest[:k], 64); err == nil {
				x = v
			}
		}
	}
	return x, y, true
}

// ⭐ eso-pic draws its argument on the page as it ships. #903 dropped the argument so it
// would stop costing a page, and the drawing was lost with it. This one draws it, at the
// paper's lower-left corner.
//
// Measured against tectonic on the document below (pdftotext -bbox, big points; word
// boxes, so the interesting numbers are DIFFERENCES from the body's own baseline, which
// shares the font and the reading):
//
//	page 1  body  yMax 136.697   ALLBG 594.680 (+457.98bp)  ONLYFG 694.306 (+557.61bp)
//	        STARLL 783.970 (+647.27bp)    x: ALLBG 99.626bp  ONLYFG 99.626bp  STARLL 9.963bp
//	page 2  body  yMax 136.697   ALLBG 594.680 (+457.98bp)   no ONLYFG, no STARLL
//
// ALLBG is a plain BG picture written in the PREAMBLE: every page has it. ONLYFG and
// STARLL are starred and written in the body of page 1: page 1 only.
const esoWitness = `\documentclass{article}
\usepackage{eso-pic}
\AddToShipoutPictureBG{\put(100,200){ALLBG}}
\begin{document}
\AddToShipoutPictureFG*{\put(100,100){ONLYFG}}
\AddToShipoutPicture*{\AtPageLowerLeft{\put(10,10){STARLL}}}
Body\ text\ of\ page\ one.
\newpage
Body\ text\ of\ page\ two.
\end{document}`

func TestEsoPicDrawsAtThePaperOriginPageOneOnlyForTheStarredForm(t *testing.T) {
	e, err := compile([]byte(esoWitness), Options{})
	if err != nil {
		t.Fatal(err)
	}
	svgs := e.RenderPages(72)
	if len(svgs) != 2 {
		t.Fatalf("%d pages, want 2", len(svgs))
	}
	const bpPerPt = 72.0 / 72.27
	_, bodyY, ok := spXY(svgs[0], "Body")
	if !ok {
		t.Fatalf("no body word on page 1:\n%s", svgs[0])
	}
	// Offsets from the body baseline, in points: the reference's big-point offsets
	// divided by 72/72.27.
	want := map[string]struct{ dy, x float64 }{
		"ALLBG":  {457.98 / bpPerPt, 99.626 / bpPerPt},
		"ONLYFG": {557.61 / bpPerPt, 99.626 / bpPerPt},
		"STARLL": {647.27 / bpPerPt, 9.963 / bpPerPt},
	}
	for w, ex := range want {
		x, y, ok := spXY(svgs[0], w)
		if !ok {
			t.Errorf("page 1 does not draw %s", w)
			continue
		}
		if math.Abs((y-bodyY)-ex.dy) > 1 || math.Abs(x-ex.x) > 0.6 {
			t.Errorf("%s at x=%.2f, %.2f below the body baseline; reference x=%.2f, %.2f", w, x, y-bodyY, ex.x, ex.dy)
		}
	}
	// Page 2: the plain preamble picture again, and neither starred one.
	_, body2, ok := spXY(svgs[1], "Body")
	if !ok {
		t.Fatal("no body word on page 2")
	}
	if _, y, ok := spXY(svgs[1], "ALLBG"); !ok || math.Abs((y-body2)-want["ALLBG"].dy) > 1 {
		t.Errorf("page 2 ALLBG: drawn %v, %.2f below the body; want %.2f", ok, y-body2, want["ALLBG"].dy)
	}
	for _, w := range []string{"ONLYFG", "STARLL"} {
		if _, _, ok := spXY(svgs[1], w); ok {
			t.Errorf("starred picture %s is drawn on page 2", w)
		}
	}
}

// The picture is a DRAWING, not text: it must not enter the body flow, add a page or
// shift the body (the failure #903 stopped, which this must not bring back).
func TestEsoPicDoesNotMoveTheBody(t *testing.T) {
	plain, err := compile([]byte(`\documentclass{article}
\begin{document}
Body\ text\ of\ page\ one.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	with, err := compile([]byte(`\documentclass{article}
\usepackage{eso-pic}
\AddToShipoutPictureBG{\put(100,200){ALLBG}}
\begin{document}
\AddToShipoutPicture*{\put(100,100){ONLYFG}}
Body\ text\ of\ page\ one.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(with.Pages()) != 1 {
		t.Fatalf("%d pages, want 1", len(with.Pages()))
	}
	y0, _ := wordY(plain.RenderPages(72)[0], "Body")
	y1, _ := wordY(with.RenderPages(72)[0], "Body")
	if y0 == 0 || y0 != y1 {
		t.Errorf("body baseline %.2f without eso-pic, %.2f with", y0, y1)
	}
}

// \ClearShipoutPicture empties the persistent list from that page on.
func TestEsoPicClearStopsThePlainPicture(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\usepackage{eso-pic}
\AddToShipoutPictureBG{\put(100,200){ALLBG}}
\begin{document}
Body\ one.
\newpage
\ClearShipoutPicture
Body\ two.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	svgs := e.RenderPages(72)
	if len(svgs) != 2 {
		t.Fatalf("%d pages, want 2", len(svgs))
	}
	if _, _, ok := spXY(svgs[0], "ALLBG"); !ok {
		t.Error("page 1 lost its picture")
	}
	if _, _, ok := spXY(svgs[1], "ALLBG"); ok {
		t.Error("page 2 still draws the picture after \\ClearShipoutPicture")
	}
}

// A void box register is not a picture: the hook ignores it instead of drawing a nil
// box (which would panic the renderer).
func TestEsoPicVoidBoxDrawsNothing(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\makeatletter
\begin{document}
\gotex@shipoutpic{0}{bg}200
Body\ text.
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) != 1 || len(pages[0].overlays) != 0 {
		t.Fatalf("%d pages, %d overlays; want 1 page and none", len(pages), len(pages[0].overlays))
	}
	if svgs := e.RenderPages(72); len(svgs) != 1 {
		t.Fatalf("%d pages, want 1", len(svgs))
	}
}

// A page may break after a picture node as it may after a mark: the node is not a
// box, and a break after it is a break after the line before it.
func TestEsoPicNodeAllowsABreakAfterIt(t *testing.T) {
	if !precedesBreak(shipPicNode{}) {
		t.Error("a shipout-picture node must not forbid a break after it")
	}
}

// eso-pic's \AtText* corners are the text block's, and on an even page of a twoside
// document the block starts at 1in+\evensidemargin (eso-pic.sty:98-118,
// \if@twoside\ifodd\c@page). The picture was built once with \oddsidemargin, so an
// even page drew it where the odd page does.
//
// Measured with tectonic 0.17.0 (pdftotext -bbox, big points) on the document below:
//
//	page 1  XTXT xMin 81.962640bp = 82.27pt   = 1in + 10pt  (\oddsidemargin)
//	page 2  XTXT xMin 111.850560bp = 112.27pt = 1in + 40pt  (\evensidemargin)
func TestEsoPicAtTextUsesEvenSideMarginOnEvenPages(t *testing.T) {
	const bpPerPt = 72.0 / 72.27
	doc := func(class string) []string {
		src := `\documentclass` + class + `{article}
\usepackage{eso-pic}
\setlength\oddsidemargin{10pt}
\setlength\evensidemargin{40pt}
\AddToShipoutPictureBG{\AtTextUpperLeft{\put(0,0){XTXT}}}
\begin{document}
Body\ one.
\newpage
Body\ two.
\end{document}`
		e, err := compile([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		return e.RenderPages(72)
	}
	x := func(svg string) float64 {
		got, _, ok := spXY(svg, "XTXT")
		if !ok {
			t.Fatalf("XTXT is not drawn:\n%s", svg)
		}
		return got
	}
	two := doc("[twoside]")
	if len(two) != 2 {
		t.Fatalf("%d pages, want 2", len(two))
	}
	for i, ref := range []float64{81.962640, 111.850560} {
		if got := x(two[i]); math.Abs(got*bpPerPt-ref) > 0.6 {
			t.Errorf("twoside page %d: XTXT at %.3fbp, tectonic %.3fbp", i+1, got*bpPerPt, ref)
		}
	}
	// A one-sided document has no even margin: both pages draw at the odd one.
	one := doc("")
	if got := x(one[1]); math.Abs(got*bpPerPt-81.962640) > 0.6 {
		t.Errorf("oneside page 2: XTXT at %.3fbp, want the odd margin 81.963bp", got*bpPerPt)
	}
}
