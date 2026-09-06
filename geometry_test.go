// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

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

// TestNewGeometry: \newgeometry re-applies geometry like \geometry; the
// bracketing/save commands are safe no-ops that leave the geometry put.
func TestNewGeometry(t *testing.T) {
	// \newgeometry applies its options exactly like \geometry.
	e := runGeom(t, `\usepackage[a4paper]{geometry}\newgeometry{margin=2in}`)
	if want := texSP(t, "210mm") - 2*texSP(t, "2in"); e.hsize != want {
		t.Errorf("hsize after \\newgeometry = %d, want %d", e.hsize, want)
	}
	// \savegeometry / \loadgeometry gobble their name and \restoregeometry is a
	// no-op: none errors, and the geometry set before them stays put.
	e2 := runGeom(t, `\usepackage[a4paper,margin=1in]{geometry}\savegeometry{s}\loadgeometry{s}\restoregeometry`)
	if want := texSP(t, "210mm") - 2*texSP(t, "1in"); e2.hsize != want {
		t.Errorf("hsize after save/load/restore = %d, want %d (unchanged)", e2.hsize, want)
	}
}

// TestGeometryInheritsClassPaper: geometry with no paper size of its own inherits the
// \documentclass paper option, so \documentclass[a4paper] + \usepackage[margin]{geometry}
// lays out on A4 (297mm tall), not US letter — the previous default, which shrank the
// text height and overflowed European papers onto extra pages.
func TestGeometryInheritsClassPaper(t *testing.T) {
	e := runGeom(t, `\documentclass[11pt,a4paper]{article}\usepackage[margin=2cm]{geometry}`)
	if want := texSP(t, "297mm") - 2*texSP(t, "2cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (A4 height − 2×2cm, inherited from the class)", e.vsize, want)
	}
	if want := texSP(t, "210mm") - 2*texSP(t, "2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (A4 width − 2×2cm)", e.hsize, want)
	}
	// A class with no paper option still defaults to letter.
	e2 := runGeom(t, `\documentclass[11pt]{article}\usepackage[margin=1in]{geometry}`)
	if want := texSP(t, "11in") - 2*texSP(t, "1in"); e2.vsize != want {
		t.Errorf("no class paper option: vsize = %d, want %d (letter)", e2.vsize, want)
	}
}

// TestGeometryScale: geometry's scale=<f> option sizes the text body to that fraction
// of the paper (on the inherited A4 here), matching the reference exactly.
func TestGeometryScale(t *testing.T) {
	e := runGeom(t, `\documentclass[11pt,a4paper]{article}\usepackage[scale=0.775]{geometry}`)
	if want := int(0.775 * float64(texSP(t, "297mm"))); e.vsize != want {
		t.Errorf("scale vsize = %d, want %d (0.775 × A4 height)", e.vsize, want)
	}
	if want := int(0.775 * float64(texSP(t, "210mm"))); e.hsize != want {
		t.Errorf("scale hsize = %d, want %d (0.775 × A4 width)", e.hsize, want)
	}
}

// TestGeometryTextBodyTotalWidthHeight: the text={w,h} / body={w,h} / total={w,h} block
// options and the width= / height= aliases all set the text block directly, matching the
// reference (they were ignored, falling back to default 1in margins).
func TestGeometryTextBodyTotalWidthHeight(t *testing.T) {
	for _, opt := range []string{"text={16cm,24cm}", "body={16cm,24cm}", "total={16cm,24cm}"} {
		e := runGeom(t, `\documentclass[a4paper]{article}\usepackage[`+opt+`]{geometry}`)
		if e.hsize != texSP(t, "16cm") || e.vsize != texSP(t, "24cm") {
			t.Errorf("%s: hsize=%d vsize=%d, want %d %d", opt, e.hsize, e.vsize, texSP(t, "16cm"), texSP(t, "24cm"))
		}
	}
	e := runGeom(t, `\documentclass[a4paper]{article}\usepackage[width=15cm,height=22cm]{geometry}`)
	if e.hsize != texSP(t, "15cm") || e.vsize != texSP(t, "22cm") {
		t.Errorf("width/height: hsize=%d vsize=%d, want %d %d", e.hsize, e.vsize, texSP(t, "15cm"), texSP(t, "22cm"))
	}
}

// TestGeometryIncludeHeadFoot: includehead / includefoot fold the header / footer into
// the text body, so the text height loses headheight+headsep (37pt) and/or footskip
// (30pt) — matching the reference; without them the header/footer sit in the margins.
func TestGeometryIncludeHeadFoot(t *testing.T) {
	base := texSP(t, "297mm") - 2*texSP(t, "1in") // A4 height − 2×1in (default: head/foot in margins)
	e := runGeom(t, `\documentclass[11pt,a4paper]{article}\usepackage[margin=1in]{geometry}`)
	if e.vsize != base {
		t.Errorf("plain: vsize=%d, want %d", e.vsize, base)
	}
	eh := runGeom(t, `\documentclass[11pt,a4paper]{article}\usepackage[margin=1in,includehead]{geometry}`)
	if want := base - ptToSP(12) - ptToSP(25); eh.vsize != want {
		t.Errorf("includehead: vsize=%d, want %d (−headheight−headsep)", eh.vsize, want)
	}
	ef := runGeom(t, `\documentclass[11pt,a4paper]{article}\usepackage[margin=1in,includefoot]{geometry}`)
	if want := base - ptToSP(30); ef.vsize != want {
		t.Errorf("includefoot: vsize=%d, want %d (−footskip)", ef.vsize, want)
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
	e := runGeom(t, `\usepackage[foo=3cm,bar,twoside,nosuchflag]{geometry}`)
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
	// geometry's vertical equation is paperheight = top + height + bottom, and by
	// default height is \textheight ALONE: the running head sits INSIDE the top
	// margin, so naming headheight/headsep/footskip does not move the first line or
	// shorten the body. Only includehead/includefoot fold a band into height.
	//
	// Folding them unconditionally cost five lines on every page of a document whose
	// style names them: automl.sty's \newgeometry{textheight=9in, top=1in,
	// headheight=12\p@, headsep=20\p@, footskip=0.5in} lost 32pt at the top and 36pt
	// at the bottom, setting 44 lines a page where the reference sets 49.
	e := runGeom(t, `\usepackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm,`+
		`head=0.5cm,headsep=0pt,foot=0.5cm]{geometry}`)
	if want := texSP(t, "9.6cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (the bands are NOT part of the body)", e.vsize, want)
	}
	if got, want := e.renderVMargin(72), spToPt(texSP(t, "0cm")); got != want {
		t.Errorf("renderVMargin = %v, want %v (the head sits in the margin)", got, want)
	}
	// Horizontally it is still the left margin, and the two now differ.
	if got, want := e.renderMargin(72), spToPt(texSP(t, "1cm")); got != want {
		t.Errorf("renderMargin = %v, want %v", got, want)
	}
}

// includeheadfoot is what folds the bands INTO the body: then they do come off the
// text height and the first line moves down by the head band.
func TestGeometryIncludeHeadFootFoldsTheBands(t *testing.T) {
	e := runGeom(t, `\usepackage[papersize={12.8cm,9.6cm},hmargin=1cm,vmargin=0cm,`+
		`head=0.5cm,headsep=0pt,foot=0.5cm,includeheadfoot]{geometry}`)
	if want := texSP(t, "9.6cm") - 2*texSP(t, "0.5cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (paper − head − foot)", e.vsize, want)
	}
	if got, want := e.renderVMargin(72), spToPt(texSP(t, "0.5cm")); got != want {
		t.Errorf("renderVMargin = %v, want %v", got, want)
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

// ── acmart / IEEEtran emulation geometry ─────────────────────────────────────
//
// acmart and IEEEtran are not embedded, and the papers that need this do not
// bundle the .cls, so they fall to the article emulation. applyAcmartGeometry /
// applyIEEEtranGeometry give the page builder the real class's single-column-
// equivalent text block and base leading; these tests pin the resulting
// \hsize / \vsize / \baselineskip, source them against acmartFormats, and check
// the class file, when resolvable, is loaded instead of the floor being applied.

func TestAcmartManuscriptGeometry(t *testing.T) {
	e, err := compile([]byte(`\documentclass[manuscript,screen,review]{acmart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("acmart manuscript: %v", err)
	}
	g := acmartFormats["manuscript"]
	if e.hsize != ptToSP(g.inkedW) || e.vsize != ptToSP(g.textH) || e.baselineskip != ptToSP(g.leading) {
		t.Errorf("manuscript block = %d×%d bls %d, want %d×%d bls %d",
			e.hsize, e.vsize, e.baselineskip, ptToSP(g.inkedW), ptToSP(g.textH), ptToSP(g.leading))
	}
	if e.baseBaselineskip != e.baselineskip {
		t.Errorf("baseBaselineskip = %d, want %d (the setspace 1.0 reference)", e.baseBaselineskip, e.baselineskip)
	}
}

func TestAcmartDefaultFormatIsManuscript(t *testing.T) {
	// With no format option acmart defaults to manuscript, so the geometry must
	// match the manuscript entry exactly.
	e, err := compile([]byte(`\documentclass{acmart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("acmart default: %v", err)
	}
	g := acmartFormats["manuscript"]
	if e.hsize != ptToSP(g.inkedW) || e.vsize != ptToSP(g.textH) || e.baselineskip != ptToSP(g.leading) {
		t.Errorf("default block = %d×%d bls %d, want manuscript %d×%d bls %d",
			e.hsize, e.vsize, e.baselineskip, ptToSP(g.inkedW), ptToSP(g.textH), ptToSP(g.leading))
	}
}

func TestAcmartTwoColumnFormat(t *testing.T) {
	twoCol := [][]string{
		{"sigconf", "screen"}, {"sigplan"}, {"acmtog"}, {"siggraph"},
		{"format=sigconf"}, {"review=false", "format=sigplan"},
	}
	for _, opts := range twoCol {
		if !acmartTwoColumnFormat(opts) {
			t.Errorf("acmartTwoColumnFormat(%v) = false, want true", opts)
		}
	}
	oneCol := [][]string{
		{"manuscript"}, {"acmsmall"}, {"acmlarge", "review"}, {"format=acmsmall"}, {},
	}
	for _, opts := range oneCol {
		if acmartTwoColumnFormat(opts) {
			t.Errorf("acmartTwoColumnFormat(%v) = true, want false (single column)", opts)
		}
	}
}

func TestAcmartSigconfGeometry(t *testing.T) {
	e, err := compile([]byte(`\documentclass[sigconf,screen]{acmart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("acmart sigconf: %v", err)
	}
	g := acmartFormats["sigconf"]
	// e.hsize is the column measure (half the block less the gutter) because an
	// emulated acmart sigconf now sets two columns, as the format does; the full
	// text block is fullWidth().
	if e.fullWidth() != ptToSP(g.inkedW) || e.vsize != ptToSP(g.textH) || e.baselineskip != ptToSP(g.leading) {
		t.Errorf("sigconf block = %d×%d bls %d, want %d×%d bls %d",
			e.fullWidth(), e.vsize, e.baselineskip, ptToSP(g.inkedW), ptToSP(g.textH), ptToSP(g.leading))
	}
	if !e.twoColumn {
		t.Error("acmart sigconf must set two columns: the format is two-column by nature")
	}
}

func TestAcmartGeometryPackageOverrides(t *testing.T) {
	// A paper's own \usepackage{geometry} must win over the acmart floor, since
	// \documentclass runs before \usepackage.
	e, err := compile([]byte(`\documentclass[sigconf]{acmart}\usepackage[margin=1in]{geometry}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("acmart+geometry: %v", err)
	}
	// sigconf is two-column, so e.hsize is the column measure; the geometry-set
	// text block is the full width.
	if want := texSP(t, "8.5in") - 2*texSP(t, "1in"); e.fullWidth() != want {
		t.Errorf("full width = %d, want %d (geometry wins)", e.fullWidth(), want)
	}
	if want := texSP(t, "11in") - 2*texSP(t, "1in"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (geometry wins)", e.vsize, want)
	}
}

func TestIEEEtranJournalGeometry(t *testing.T) {
	// The real paper writes "10 pt" with a space, which is not a size keyword; the
	// journal default geometry must still apply.
	e, err := compile([]byte(`\documentclass[letterpaper, 10 pt]{IEEEtran}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("IEEEtran journal: %v", err)
	}
	// The TEXT BLOCK is 504pt wide; e.hsize is the column measure, half of it less
	// the gutter, because an emulated IEEEtran now sets two columns (packages.go).
	if e.fullWidth() != ptToSP(504) || e.vsize != ptToSP(696) || e.baselineskip != ptToSP(12) {
		t.Errorf("journal block = %d×%d bls %d, want %d×%d bls %d",
			e.fullWidth(), e.vsize, e.baselineskip, ptToSP(504), ptToSP(696), ptToSP(12))
	}
	if !e.twoColumn {
		t.Error("an emulated IEEEtran must set two columns: the class is two-column by nature")
	}
}

func TestIEEEtranConferenceGeometry(t *testing.T) {
	e, err := compile([]byte(`\documentclass[conference]{IEEEtran}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("IEEEtran conference: %v", err)
	}
	if e.vsize != ptToSP(668) {
		t.Errorf("conference vsize = %d, want %d (9.25in)", e.vsize, ptToSP(668))
	}
	if e.fullWidth() != ptToSP(504) { // e.hsize is the column measure (two columns)
		t.Errorf("conference text block = %d, want %d", e.fullWidth(), ptToSP(504))
	}
}

func TestIEEEtranTechnoteGeometry(t *testing.T) {
	e, err := compile([]byte(`\documentclass[technote]{IEEEtran}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("IEEEtran technote: %v", err)
	}
	if e.baselineskip != ptToSP(11) {
		t.Errorf("technote leading = %d, want %d (9pt body)", e.baselineskip, ptToSP(11))
	}
}

func TestClassFileResolvable(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if !e.classFileResolvable("amsart") {
		t.Error("amsart should be resolvable (it is embedded)")
	}
	if e.classFileResolvable("acmart") {
		t.Error("acmart should not be resolvable (it is not embedded)")
	}
}

func TestAcmartRealClassNotOverriddenByFloor(t *testing.T) {
	// When the paper bundles acmart.cls the real class is loaded and sizes its own
	// page; the emulation floor must NOT be applied. A minimal stand-in on the
	// search path is enough to exercise that branch.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "acmart.cls"), []byte(`\LoadClass{article}\endinput`), 0o644); err != nil {
		t.Fatalf("write acmart.cls: %v", err)
	}
	t.Setenv("GOTEX_TEXMF", dir)
	e, err := compile([]byte(`\documentclass[manuscript]{acmart}\begin{document}x\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("acmart with bundled cls: %v", err)
	}
	if e.hsize == ptToSP(acmartFormats["manuscript"].inkedW) {
		t.Error("emulation floor was applied even though acmart.cls was resolvable")
	}
}

// achemso's manuscript mode is the double-spaced review layout ACS requires for
// submission, and it is what arXiv preprints in that class use. Without it the
// article emulation kept the size-default leading and packed twice the text onto
// every page: corpus paper 2209.13121 came out in 23 pages against the real
// class's 42, with 98% of its words present — compressed, not lost.
func TestAchemsoManuscriptIsDoubleSpaced(t *testing.T) {
	for _, c := range []struct {
		name    string
		opts    []string
		wantMan bool
	}{
		{"the arXiv form", []string{"manuscript=article", "layout=traditional"}, true},
		{"another article type", []string{"manuscript=note"}, true},
		{"the bare option", []string{"manuscript"}, true},
		{"the journal layout", []string{"journal=jacsat"}, false},
		{"no options at all", nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := achemsoManuscript(c.opts); got != c.wantMan {
				t.Fatalf("achemsoManuscript(%v) = %v, want %v", c.opts, got, c.wantMan)
			}
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			before := e.baselineskip
			e.applyAchemsoGeometry(c.opts)
			if !c.wantMan {
				if e.baselineskip != before {
					t.Errorf("baselineskip moved to %d for a non-manuscript layout", e.baselineskip)
				}
				return
			}
			// 23.9pt between baselines, measured from the class's own reference PDF.
			if want := ptToSP(23.9); e.baselineskip != want {
				t.Errorf("baselineskip = %d, want %d (23.9pt, double-spaced)", e.baselineskip, want)
			}
			// The single-spacing reference moves with it, so a \singlespacing inside
			// the document does not snap back to the emulation's default.
			if e.baseBaselineskip != e.baselineskip {
				t.Errorf("baseBaselineskip = %d, want it to follow at %d", e.baseBaselineskip, e.baselineskip)
			}
			if want := ptToSP(468); e.hsize != want {
				t.Errorf("hsize = %d, want %d (468pt, letter less 1in margins)", e.hsize, want)
			}
			if want := ptToSP(648); e.vsize != want {
				t.Errorf("vsize = %d, want %d (648pt)", e.vsize, want)
			}
		})
	}
}

// elsarticle sizes nothing for its default (preprint) type — the class is
// \LoadClass{article} plus Elsevier front matter — but its JOURNAL types install
// a text block through geometry. The values here are elsarticle.cls's own.
func TestElsarticleJournalTypes(t *testing.T) {
	for _, c := range []struct {
		opts         []string
		want         bool
		wantW, wantH float64
	}{
		{[]string{"10pt", "3p"}, true, 468, 622},
		{[]string{"1p"}, true, 384, 562},
		{[]string{"5p", "twocolumn"}, true, 522, 682},
		{[]string{"preprint", "12pt", "a4paper"}, false, 0, 0},
		{nil, false, 0, 0},
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		lead := e.baselineskip
		got := e.applyElsarticleGeometry(c.opts)
		if got != c.want {
			t.Errorf("applyElsarticleGeometry(%v) = %v, want %v", c.opts, got, c.want)
			continue
		}
		if !c.want {
			continue
		}
		if want := ptToSP(c.wantW); e.hsize != want {
			t.Errorf("%v: hsize = %d, want %d", c.opts, e.hsize, want)
		}
		if want := ptToSP(c.wantH); e.vsize != want {
			t.Errorf("%v: vsize = %d, want %d", c.opts, e.vsize, want)
		}
		// A journal type keeps \baselinestretch at 1, so the size option's leading
		// must survive untouched.
		if e.baselineskip != lead {
			t.Errorf("%v: leading moved to %d, want it left at %d", c.opts, e.baselineskip, lead)
		}
	}
}

// The paper size a class defaults to must not override one the paper chose.
func TestNamesPaperSize(t *testing.T) {
	for _, c := range []struct {
		opts []string
		want bool
	}{
		{[]string{"preprint", "12pt", "a4paper"}, true},
		{[]string{"letterpaper"}, true},
		{[]string{"10pt", "3p"}, false},
		{nil, false},
	} {
		if got := namesPaperSize(c.opts); got != c.want {
			t.Errorf("namesPaperSize(%v) = %v, want %v", c.opts, got, c.want)
		}
	}
}

// KOMA-Script's classes default to 11pt where the standard classes default to
// 10pt. They are not embedded, so a paper that does not bundle one falls to the
// article-shaped emulation — close enough in shape, but it started from the wrong
// base size: corpus paper 2212.13760 came out at a 12pt leading against the
// reference's 13.6.
func TestKomaClassesDefaultTo11pt(t *testing.T) {
	for _, c := range []struct {
		src  string
		want float64
	}{
		{`\documentclass{scrartcl}`, 13.6},
		{`\documentclass[a4paper]{scrarticle}`, 13.6},
		{`\documentclass{scrbook}`, 13.6},
		{`\documentclass[12pt]{scrartcl}`, 14.5}, // a named size still wins
		{`\documentclass[10pt]{scrartcl}`, 12},   // including back down to 10pt
		{`\documentclass{article}`, 12},          // a standard class is unchanged
	} {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.src + `\begin{document}A\end{document}`); err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		if want := ptToSP(c.want); e.baseBaselineskip != want {
			t.Errorf("%s: base leading = %d, want %d (%.1fpt)", c.src, e.baseBaselineskip, want, c.want)
		}
	}
}
