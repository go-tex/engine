// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// runGeom builds a LaTeX engine, runs src in the preamble and returns the engine
// so a test can inspect the resulting \hsize / \vsize / geometry state.
func runGeom(t *testing.T, src string) *Engine {
	t.Helper()
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(src); err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return e
}

// texSP converts a dimension the way TeX does — the engine's own scanner, exact
// integer arithmetic — so an expected value in these tests is not a floating-point
// approximation of the number the engine computes. TestGeometryUnitsAreTeXExact
// pins that conversion against what real TeX prints.
func texSP(t *testing.T, s string) int {
	t.Helper()
	e, err := buildEngine(Options{}, false)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	d, ok := e.geomEval(s)
	if !ok {
		t.Fatalf("texSP(%q): not a dimension", s)
	}
	return d
}

func TestGeometryA4Margin1in(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,margin=1in]{geometry}`)
	// \hsize = 210mm − 2in, \vsize = 297mm − 2in.
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2in)", e.hsize, want)
	}
	if want := texSP(t, "297mm") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2in)", e.vsize, want)
	}
	// The render margin is the (uniform) left margin = 1in.
	if got, want := e.renderMargin(72), spToPt(texSP(t, "1in")); got != want {
		t.Errorf("renderMargin = %v, want %v", got, want)
	}
}

func TestGeometryLetterDefault(t *testing.T) {
	// geometry's default paper is letterpaper; loading it with just a margin uses
	// 8.5in × 11in.
	e := runGeom(t, `\usepackage[margin=1in]{geometry}`)
	if want := texSP(t, "8.5in") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (8.5in-2in)", e.hsize, want)
	}
	if want := texSP(t, "11in") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (11in-2in)", e.vsize, want)
	}
	// An explicit letterpaper keyword gives the same result.
	e2 := runGeom(t, `\usepackage[letterpaper,margin=1in]{geometry}`)
	if e2.hsize != e.hsize || e2.vsize != e.vsize {
		t.Errorf("letterpaper explicit (%d,%d) != default (%d,%d)", e2.hsize, e2.vsize, e.hsize, e.vsize)
	}
}

func TestGeometryLandscapeSwaps(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,landscape,margin=1in]{geometry}`)
	// landscape swaps width/height: \hsize now derives from 297mm, \vsize from 210mm.
	if want := texSP(t, "297mm") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("landscape hsize = %d, want %d (297mm-2in)", e.hsize, want)
	}
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("landscape vsize = %d, want %d (210mm-2in)", e.vsize, want)
	}
}

func TestGeometryTextwidth(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,textwidth=15cm]{geometry}`)
	if want := texSP(t, "15cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (15cm)", e.hsize, want)
	}
}

func TestGeometryTextheight(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,textheight=20cm]{geometry}`)
	if want := texSP(t, "20cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (20cm)", e.vsize, want)
	}
}

func TestGeometryLeftRight(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,left=2cm,right=3cm]{geometry}`)
	if want := texSP(t, "210mm") - texSP(t, "2cm") - texSP(t, "3cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2cm-3cm)", e.hsize, want)
	}
	// The render margin is the left margin (2cm), not the right.
	if got, want := e.renderMargin(72), spToPt(texSP(t, "2cm")); got != want {
		t.Errorf("renderMargin = %v, want %v (left=2cm)", got, want)
	}
}

func TestGeometryTopBottom(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,top=1.5cm,bottom=2.5cm]{geometry}`)
	if want := texSP(t, "297mm") - texSP(t, "1.5cm") - texSP(t, "2.5cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-1.5cm-2.5cm)", e.vsize, want)
	}
}

func TestGeometryHVMargin(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,hmargin=2cm,vmargin=3cm]{geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2*2cm)", e.hsize, want)
	}
	if want := texSP(t, "297mm") - 2*texSP(t, "3cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2*3cm)", e.vsize, want)
	}
}

func TestGeometryAliasKeys(t *testing.T) {
	// lmargin/rmargin/tmargin/bmargin are aliases for left/right/top/bottom.
	e := runGeom(t, `\usepackage[a4paper,lmargin=1cm,rmargin=2cm,tmargin=3cm,bmargin=4cm]{geometry}`)
	if want := texSP(t, "210mm") - texSP(t, "1cm") - texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
	if want := texSP(t, "297mm") - texSP(t, "3cm") - texSP(t, "4cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d", e.vsize, want)
	}
}

func TestGeometryPaperWidthHeight(t *testing.T) {
	e := runGeom(t, `\usepackage[paperwidth=20cm,paperheight=25cm,margin=1cm]{geometry}`)
	if want := texSP(t, "20cm") - 2*texSP(t, "1cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
	if want := texSP(t, "25cm") - 2*texSP(t, "1cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d", e.vsize, want)
	}
}

func TestGeometryReapply(t *testing.T) {
	// \geometry{...} re-applies onto the earlier \usepackage state: paper stays a4,
	// margins become 2cm (later wins).
	e := runGeom(t, `\usepackage[a4paper,margin=1in]{geometry}\geometry{margin=2cm}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2*2cm)", e.hsize, want)
	}
	if want := texSP(t, "297mm") - 2*texSP(t, "2cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2*2cm)", e.vsize, want)
	}
}

func TestGeometryOtherPaperSizes(t *testing.T) {
	cases := []struct {
		name    string
		w, h    int
		keyword string
	}{
		{"a5", texSP(t, "148mm"), texSP(t, "210mm"), "a5paper"},
		{"b5", texSP(t, "176mm"), texSP(t, "250mm"), "b5paper"},
		{"legal", texSP(t, "8.5in"), texSP(t, "14in"), "legalpaper"},
		{"executive", texSP(t, "7.25in"), texSP(t, "10.5in"), "executivepaper"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := runGeom(t, `\usepackage[`+c.keyword+`,margin=1cm]{geometry}`)
			if want := c.w - 2*texSP(t, "1cm"); e.hsize != want {
				t.Errorf("hsize = %d, want %d", e.hsize, want)
			}
			if want := c.h - 2*texSP(t, "1cm"); e.vsize != want {
				t.Errorf("vsize = %d, want %d", e.vsize, want)
			}
		})
	}
}

func TestGeometryUnknownKeysIgnored(t *testing.T) {
	// Unknown keys and bare flags leave the layout at defaults (letterpaper, 1in).
	e := runGeom(t, `\usepackage[foo=3cm,bar,twoside,includehead]{geometry}`)
	if want := texSP(t, "8.5in") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (defaults preserved)", e.hsize, want)
	}
	if want := texSP(t, "11in") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (defaults preserved)", e.vsize, want)
	}
}

func TestGeometryMalformedDimenNoPanic(t *testing.T) {
	// A malformed dimension must be ignored, not panic, and not corrupt the layout:
	// margin stays at the 1in default.
	e := runGeom(t, `\usepackage[a4paper,margin=abc]{geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (malformed margin ignored ⇒ 1in default)", e.hsize, want)
	}
	// An empty value is likewise ignored.
	e2 := runGeom(t, `\usepackage[a4paper,margin=]{geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e2.hsize != want {
		t.Errorf("empty margin: hsize = %d, want %d", e2.hsize, want)
	}
}

func TestGeometryCommandStandalone(t *testing.T) {
	// \geometry without a prior \usepackage initialises the state from defaults.
	e := runGeom(t, `\geometry{a4paper,margin=2cm}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
}

func TestGeometryNotLoadedNoEffect(t *testing.T) {
	// Loading a non-geometry package must not touch \hsize / \vsize or install a
	// geometry render margin.
	e := runGeom(t, `\usepackage[a4paper]{article}\usepackage{amsmath}`)
	if e.geom != nil {
		t.Errorf("geom state = %+v, want nil (geometry not loaded)", e.geom)
	}
	if got := e.renderMargin(72); got != 72 {
		t.Errorf("renderMargin fallback = %v, want 72", got)
	}
}

func TestGeometryAmongPackageList(t *testing.T) {
	// geometry among a comma-separated package list still applies its options.
	e := runGeom(t, `\usepackage[a4paper,margin=1cm]{amsmath,geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "1cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
}

func TestGeometryTextThenMarginLastWins(t *testing.T) {
	// textwidth then a margin: the later margin re-establishes paper-based hsize.
	e := runGeom(t, `\usepackage[a4paper,textwidth=15cm]{geometry}\geometry{margin=2cm}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (margin should override textwidth)", e.hsize, want)
	}
}

func TestGeometryPortraitFlag(t *testing.T) {
	// portrait after landscape restores portrait orientation.
	e := runGeom(t, `\usepackage[a4paper,landscape,portrait,margin=1in]{geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("portrait hsize = %d, want %d", e.hsize, want)
	}
}

func TestGeometryEmptyAndBlankItems(t *testing.T) {
	// An empty option group and empty comma-separated items are skipped; the
	// layout stays at defaults (letterpaper, 1in).
	e := runGeom(t, `\geometry{}`)
	if want := texSP(t, "8.5in") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("empty group: hsize = %d, want %d", e.hsize, want)
	}
	// Double comma yields an empty middle item that must be skipped.
	e2 := runGeom(t, `\usepackage[a4paper,,margin=2cm]{geometry}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e2.hsize != want {
		t.Errorf("double comma: hsize = %d, want %d", e2.hsize, want)
	}
}

func TestGeometryUsepackageNoNameGroup(t *testing.T) {
	// \usepackage[opts] with no following {name} group: the peeked token is put
	// back and nothing is applied (no geometry among an empty name list).
	e := runGeom(t, `\usepackage[a4paper]x`)
	if e.geom != nil {
		t.Errorf("geom = %+v, want nil (no package name group)", e.geom)
	}
}

func TestGeometryDimenUnitDefaultsToPoints(t *testing.T) {
	e := runGeom(t, ``)
	// A bare number is points, as everywhere a <dimen> is scanned.
	if got, ok := e.geomEval("30"); !ok || got != 30*unity {
		t.Errorf("geomEval(30) = %d,%v, want %d,true", got, ok, 30*unity)
	}
	// A value that cannot begin a dimension is refused, so the key is ignored
	// rather than stored as a bogus zero.
	for _, bad := range []string{"  ", "in", "1.2.3in"} {
		if _, ok := e.geomEval(bad); ok {
			t.Errorf("geomEval(%q) ok = true, want false", bad)
		}
	}
}

// TestGeometryUnitsAreTeXExact locks geometry onto the engine's own dimension
// scanner. TeX converts a unit by exact integer arithmetic (cm is 7227/254 pt), and
// a floating-point conversion of the same value lands a few scaled points away:
// 12.8cm is 23867907sp to TeX and 23867902sp in float64. Geometry used to hold the
// second number, so a beamer page came out a rounding step narrower than the class
// asked for.
func TestGeometryUnitsAreTeXExact(t *testing.T) {
	e := runGeom(t, ``)
	for _, c := range []struct {
		val  string
		want int
	}{
		{"12.8cm", 23867907},
		{"9.6cm", 17900937},
		{"1in", 4736286},
	} {
		got, ok := e.geomEval(c.val)
		if !ok || got != c.want {
			t.Errorf("geomEval(%s) = %d,%v, want %d,true", c.val, got, ok, c.want)
		}
	}
}

// TestAmsartEmulatedGeometry locks amsart's real text block onto the emulated
// class. amsart.cls sets \textwidth=30pc (360pt) and \textheight=50.5pc (606pt),
// then \calclayout removes \headheight (8pt)+\headsep (14pt), leaving a 584pt text
// height. The emulation does not run those assignments, so before the fix \hsize/
// \vsize kept the plain-TeX defaults (6.5in × 8.9in) and amsart under-paginated;
// these assertions match what tectonic reports for \documentclass{amsart}.
func TestAmsartEmulatedGeometry(t *testing.T) {
	for _, opt := range []string{"", "11pt", "12pt", "reqno,12pt"} {
		e, err := compile([]byte(`\documentclass[`+opt+`]{amsart}\begin{document}x\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("amsart[%s]: %v", opt, err)
		}
		// The size options change only the leading, not the text block.
		if want := ptToSP(360); e.hsize != want {
			t.Errorf("amsart[%s] hsize = %d (%.2fpt), want %d (360pt)", opt, e.hsize, float64(e.hsize)/unity, want)
		}
		if want := ptToSP(584); e.vsize != want {
			t.Errorf("amsart[%s] vsize = %d (%.2fpt), want %d (584pt)", opt, e.vsize, float64(e.vsize)/unity, want)
		}
	}
}

// TestAmsartA4paperGeometry: on a4paper amsart uses \textheight=54.5pc (654pt),
// so the text height after \calclayout is 654-22 = 632pt; the width is unchanged.
func TestAmsartA4paperGeometry(t *testing.T) {
	e, err := compile([]byte(`\documentclass[a4paper]{amsart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("amsart a4paper: %v", err)
	}
	if want := ptToSP(360); e.hsize != want {
		t.Errorf("hsize = %d, want %d (360pt)", e.hsize, want)
	}
	if want := ptToSP(632); e.vsize != want {
		t.Errorf("vsize = %d (%.2fpt), want %d (632pt)", e.vsize, float64(e.vsize)/unity, want)
	}
}

// TestAmsartGeometryPackageOverrides: a paper's own \usepackage{geometry} must win
// over the amsart class defaults, since \documentclass runs before \usepackage.
func TestAmsartGeometryPackageOverrides(t *testing.T) {
	e, err := compile([]byte(`\documentclass[12pt]{amsart}\usepackage[margin=1in]{geometry}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("amsart+geometry: %v", err)
	}
	// letterpaper (8.5in × 11in) minus 1in margins on every side.
	if want := texSP(t, "8.5in") - 2*texSP(t, "1in"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (geometry wins)", e.hsize, want)
	}
	if want := texSP(t, "11in") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (geometry wins)", e.vsize, want)
	}
}

// ── beamer's own geometry: the options a real class actually passes ──────────

func TestGeometryPapersizePair(t *testing.T) {
	// papersize={<w>,<h>} carries a comma INSIDE braces. Splitting the option list
	// naively tore it in half and both halves were dropped as malformed, which is
	// how beamer's paper size went missing.
	e := runGeom(t, `\usepackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm]{geometry}`)
	if want := texSP(t, "12.8cm") - 2*texSP(t, "1cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
	if want := texSP(t, "9.6cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d", e.vsize, want)
	}
	if e.geom.paperW != texSP(t, "12.8cm") || e.geom.paperH != texSP(t, "9.6cm") {
		t.Errorf("paper = %d×%d, want %d×%d", e.geom.paperW, e.geom.paperH,
			texSP(t, "12.8cm"), texSP(t, "9.6cm"))
	}
}

func TestGeometryPapersizeSquare(t *testing.T) {
	// A single value makes a square page.
	e := runGeom(t, `\usepackage[papersize=10cm,margin=1cm]{geometry}`)
	if e.geom.paperW != texSP(t, "10cm") || e.geom.paperH != texSP(t, "10cm") {
		t.Errorf("paper = %d×%d, want %d square", e.geom.paperW, e.geom.paperH, texSP(t, "10cm"))
	}
}

func TestGeometryValueIsALengthRegister(t *testing.T) {
	// beamer passes its paper size as \beamer@paperwidth, not as a number. A value
	// that names a length has to be read at the value the length holds.
	e := runGeom(t, `\newlength\pw\setlength\pw{12.8cm}\newlength\ph\setlength\ph{9.6cm}`+
		`\usepackage[papersize={\pw,\ph},hmargin=1cm,vmargin=0cm]{geometry}`)
	if e.geom.paperW != 23867907 || e.geom.paperH != 17900937 {
		t.Errorf("paper = %d×%d, want 23867907×17900937", e.geom.paperW, e.geom.paperH)
	}
}

func TestGeometryValueIsADimenRegister(t *testing.T) {
	e := runGeom(t, `\newdimen\pw\pw=20cm\usepackage[paperwidth=\pw,margin=1cm]{geometry}`)
	if e.geom.paperW != texSP(t, "20cm") {
		t.Errorf("paperW = %d, want %d", e.geom.paperW, texSP(t, "20cm"))
	}
}

func TestGeometryEvalDoesNotEatTheDocument(t *testing.T) {
	// The dimension scan for a control-sequence value runs on an isolated input
	// stack: what follows the option list must still be there afterwards.
	e := runGeom(t, `\newdimen\pw\pw=20cm\usepackage[paperwidth=\pw]{geometry}\def\after{ok}`)
	if m := e.eq["after"]; m == nil {
		t.Error(`\def after the option list was swallowed by the value scan`)
	}
}

func TestGeometryHeadAndFootBands(t *testing.T) {
	// head/headsep/foot sit between the margins and the text block, so they come
	// off \vsize. They default to zero: a document that never names them keeps the
	// plain top/bottom arithmetic.
	e := runGeom(t, `\usepackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm,`+
		`head=0.5cm,headsep=0pt,foot=0.5cm]{geometry}`)
	if want := texSP(t, "9.6cm") - 2*texSP(t, "0.5cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (paper − head − foot)", e.vsize, want)
	}
	// The renderer's vertical margin is the top margin plus the head band, which is
	// the whole distance from the paper edge down to the first line.
	if got, want := e.renderVMargin(72), spToPt(texSP(t, "0.5cm")); got != want {
		t.Errorf("renderVMargin = %v, want %v", got, want)
	}
	// Horizontally it is still the left margin, and the two now differ.
	if got, want := e.renderMargin(72), spToPt(texSP(t, "1cm")); got != want {
		t.Errorf("renderMargin = %v, want %v", got, want)
	}
}

func TestGeometryEqualMarginsKeepOneRenderMargin(t *testing.T) {
	// With no head/foot band the two render margins agree, so nothing changes for a
	// document that asks for the same margin all round.
	e := runGeom(t, `\usepackage[a4paper,margin=2cm]{geometry}`)
	if e.renderMargin(72) != e.renderVMargin(72) {
		t.Errorf("renderMargin %v != renderVMargin %v with uniform margins",
			e.renderMargin(72), e.renderVMargin(72))
	}
}

func TestGeometryPublishesPageLengths(t *testing.T) {
	// A class reads the paper back from \paperwidth/\paperheight, and decides which
	// geometry interface it is talking to by testing \Gm@lmargin.
	e := runGeom(t, `\usepackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm]{geometry}`)
	for _, c := range []struct {
		name string
		want int
	}{
		{"paperwidth", texSP(t, "12.8cm")},
		{"paperheight", texSP(t, "9.6cm")},
		{"Gm@lmargin", texSP(t, "1cm")},
		{"Gm@rmargin", texSP(t, "1cm")},
		{"Gm@tmargin", 0},
		{"Gm@bmargin", 0},
	} {
		m := e.eq[c.name]
		if m == nil {
			t.Errorf(`\%s is not allocated`, c.name)
			continue
		}
		if got := e.dimen[m.code]; got != c.want {
			t.Errorf(`\%s = %d, want %d`, c.name, got, c.want)
		}
	}
}
