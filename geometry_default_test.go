// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strings"
	"testing"
)

// ⛔ A GEOMETRY THAT FIXES NOTHING STILL HAS A BODY. geometry.sty decides each
// direction from which of margin, body, margin the user fixed (\Gm@detall,
// geometry.dtx:2237-2271); with none fixed the body is 0.7 of the paper
// (\Gm@Dhscale, \Gm@Dvscale = 0.7, geometry.dtx:1928-1929) and the margins split
// the rest 1:1 horizontally and 2:3 vertically (1918-1920). The engine kept the
// state's initial 1in on every side instead, so a document that names only
// letterpaper, head= and heightrounded got a 469.755pt x 646pt body.
//
// Witness g3 (unblock-outliers-2.md), tectonic 0.17.0:
//
//	\documentclass{article}
//	\usepackage[letterpaper,head=13pt,marginparwidth=6pc,heightrounded]{geometry}
//	TW=430.00462pt TH=562.0pt OS=19.8752pt ES=19.8752pt
//
// 11in = 794.97pt; 0.7 of it is 556.48pt, minus \topskip 10pt = 546.48, which is
// 45 lines of 12pt and a remainder of 6.48pt, doubled 12.96 > 12, so the
// heightrounded step rounds up to 46: 46*12 + 10 = 562.
func TestGeometryDefaultBodyIsSevenTenths(t *testing.T) {
	src := `\documentclass{article}` +
		`\usepackage[letterpaper,head=13pt,marginparwidth=6pc,heightrounded]{geometry}` +
		`\begin{document}TW=\the\textwidth\ TH=\the\textheight\ ` +
		`OS=\the\oddsidemargin\ ES=\the\evensidemargin\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	got := b.String()
	for _, w := range []string{"TW=430.00462pt", "TH=562.0pt", "OS=19.8752pt", "ES=19.8752pt"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s (tectonic's value) in %q", w, got)
		}
	}
}

// Two-sided, the default horizontal ratio is 2:3 inner:outer (geometry.dtx:1919)
// and \evensidemargin takes the OUTER margin. tectonic, acmart manuscript (which
// is twoside) at the same options: OS=1.44614pt ES=38.30426pt.
func TestGeometryDefaultBodyTwoSide(t *testing.T) {
	src := `\documentclass[twoside]{article}` +
		`\usepackage[letterpaper,head=13pt,marginparwidth=6pc,heightrounded]{geometry}` +
		`\begin{document}OS=\the\oddsidemargin\ ES=\the\evensidemargin\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	got := b.String()
	for _, w := range []string{"OS=1.44614pt", "ES=38.30426pt"} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s (tectonic's value) in %q", w, got)
		}
	}
}

// \topmargin loses the head band unless the head is part of the body
// (geometry.dtx:3286-3292): the two differ by \headheight + \headsep.
func TestGeometryTopmarginLosesTheHeadBand(t *testing.T) {
	read := func(opts string) int {
		e := runGeom(t, `\usepackage[letterpaper,`+opts+`]{geometry}`)
		v, _ := e.namedDimen("topmargin")
		return v
	}
	// top and bottom are fixed so the body cannot move between the two reads.
	plain := read(`head=13pt,top=2in,bottom=1in`)
	incl := read(`head=13pt,top=2in,bottom=1in,includehead`)
	band := texSP(t, "13pt") + e0HeadSep(t)
	if d := incl - plain; d != band {
		t.Errorf("topmargin differs by %.3fpt between includehead and not, want the head band %.3fpt",
			spToPt(d), spToPt(band))
	}
}

// e0HeadSep is the article class's \headsep, the one a geometry that names only
// head= leaves alone.
func e0HeadSep(t *testing.T) int {
	t.Helper()
	e := runGeom(t, ``)
	v, _ := e.namedDimen("headsep")
	return v
}

// A named margin fixes its part, so the default body must NOT replace the
// arithmetic the user asked for: only the unfixed direction takes 0.7.
func TestGeometryDefaultBodyYieldsToANamedMargin(t *testing.T) {
	e := runGeom(t, `\usepackage[letterpaper,hmargin=1in]{geometry}`)
	if want := texSP(t, "614.295pt") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %.3fpt, want 1in margins (%.3fpt)", spToPt(e.hsize), spToPt(want))
	}
	if h := geomDefaultBody(texSP(t, "794.97pt")); e.vsize != h {
		t.Errorf("vsize = %.3fpt, want the default 0.7 body %.3fpt", spToPt(e.vsize), spToPt(h))
	}
}

// Witness g2: acmart's manuscript format hands geometry the same three options
// as g3 and nothing else, and tectonic prints TW=430.00462pt TH=560.0pt for it.
// The emulation's row said 465 x 585, measured off a PDF.
func TestAcmartManuscriptTextBlock(t *testing.T) {
	f := acmartFormats["manuscript"]
	if math.Abs(f.inkedW-430.00462) > 0.0001 || f.textH != 560 {
		t.Errorf("manuscript block = %.5f x %.1f, tectonic reports 430.00462 x 560", f.inkedW, f.textH)
	}
}

// ⛔ ONE MARGIN BESIDE A BODY DOES NOT CANCEL THE BODY. neurips_2024.sty and
// arxiv.sty run, at \begin{document},
//
//	\newgeometry{textheight=9in,textwidth=5.5in,top=1in,headheight=12pt,
//	             headsep=25pt,footskip=30pt}
//
// after \usepackage[verbose=true,letterpaper]{geometry}. `top` used to clear the
// text height, and the bottom margin it then subtracted was the 0.7-body default
// the first call had left in the state: an 8in text block, 5 extra pages on
// 2405.16883 (29 -> 34) and 2405.18549 (40 -> 45).
//
// Witnesses, tectonic 0.17.0 (--print):
//
//	the line above:        TW=397.48499pt TH=650.43pt OS=36.13501pt TM=-37.0pt
//	+ bottom=2in,left=1in: TW=397.48499pt TH=578.16pt OS=0.0pt     TM=-37.0pt
//
// The second is over-specified in the vertical direction (top, height, bottom),
// where geometry drops the height; horizontally textwidth and left leave the
// right margin free, so the width stays.
func TestGeometryOneMarginKeepsTheBody(t *testing.T) {
	// The text layer joins the control words with no space. The last two cases
	// are tectonic's values for both key orders: margin + body + margin, in
	// either order, drops the body (it is the order that decides which margin
	// key sees the other already named).
	for _, c := range []struct{ name, newgeo, want string }{
		{"top beside textheight",
			`\newgeometry{textheight=9in,textwidth=5.5in,top=1in,headheight=12pt,headsep=25pt,footskip=30pt}`,
			"TW=397.48499ptTH=650.43ptOS=36.13501ptTM=-37.0pt"},
		{"top and bottom drop the height",
			`\newgeometry{textheight=9in,top=1in,bottom=2in,textwidth=5.5in,left=1in}`,
			"TW=397.48499ptTH=578.16ptOS=0.0ptTM=-37.0pt"},
		{"right then left, bottom then top",
			`\newgeometry{textheight=9in,textwidth=5.5in,right=1.5in,left=1in,bottom=2in,top=1in}`,
			"TW=433.62001ptTH=578.16ptOS=0.0ptTM=-37.0pt"},
		{"left then right, top then bottom",
			`\newgeometry{textheight=9in,textwidth=5.5in,left=1in,right=1.5in,top=1in,bottom=2in}`,
			"TW=433.62001ptTH=578.16ptOS=0.0ptTM=-37.0pt"},
	} {
		want := c.want
		src := `\documentclass{article}\usepackage[verbose=true,letterpaper]{geometry}` +
			`\AtBeginDocument{` + c.newgeo + `}` +
			`\begin{document}TW=\the\textwidth\ TH=\the\textheight\ OS=\the\oddsidemargin\ ` +
			`TM=\the\topmargin\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for _, p := range e.Pages() {
			b.WriteString(mvlText(p.list))
		}
		if got := b.String(); !strings.Contains(got, want) {
			t.Errorf("%s: want %q (tectonic), got %q", c.name, want, got)
		}
	}
}
