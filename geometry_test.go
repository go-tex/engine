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

func TestGeometryA4Margin1in(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,margin=1in]{geometry}`)
	// \hsize = 210mm − 2in, \vsize = 297mm − 2in.
	if want := mmToSP(210) - 2*inToSP(1); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2in)", e.hsize, want)
	}
	if want := mmToSP(297) - 2*inToSP(1); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2in)", e.vsize, want)
	}
	// The render margin is the (uniform) left margin = 1in.
	if got, want := e.renderMargin(72), spToPt(inToSP(1)); got != want {
		t.Errorf("renderMargin = %v, want %v", got, want)
	}
}

func TestGeometryLetterDefault(t *testing.T) {
	// geometry's default paper is letterpaper; loading it with just a margin uses
	// 8.5in × 11in.
	e := runGeom(t, `\usepackage[margin=1in]{geometry}`)
	if want := inToSP(8.5) - 2*inToSP(1); e.hsize != want {
		t.Errorf("hsize = %d, want %d (8.5in-2in)", e.hsize, want)
	}
	if want := inToSP(11) - 2*inToSP(1); e.vsize != want {
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
	if want := mmToSP(297) - 2*inToSP(1); e.hsize != want {
		t.Errorf("landscape hsize = %d, want %d (297mm-2in)", e.hsize, want)
	}
	if want := mmToSP(210) - 2*inToSP(1); e.vsize != want {
		t.Errorf("landscape vsize = %d, want %d (210mm-2in)", e.vsize, want)
	}
}

func TestGeometryTextwidth(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,textwidth=15cm]{geometry}`)
	if want := parseDimenStr("15cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (15cm)", e.hsize, want)
	}
}

func TestGeometryTextheight(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,textheight=20cm]{geometry}`)
	if want := parseDimenStr("20cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (20cm)", e.vsize, want)
	}
}

func TestGeometryLeftRight(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,left=2cm,right=3cm]{geometry}`)
	if want := mmToSP(210) - parseDimenStr("2cm") - parseDimenStr("3cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2cm-3cm)", e.hsize, want)
	}
	// The render margin is the left margin (2cm), not the right.
	if got, want := e.renderMargin(72), spToPt(parseDimenStr("2cm")); got != want {
		t.Errorf("renderMargin = %v, want %v (left=2cm)", got, want)
	}
}

func TestGeometryTopBottom(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,top=1.5cm,bottom=2.5cm]{geometry}`)
	if want := mmToSP(297) - parseDimenStr("1.5cm") - parseDimenStr("2.5cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-1.5cm-2.5cm)", e.vsize, want)
	}
}

func TestGeometryHVMargin(t *testing.T) {
	e := runGeom(t, `\usepackage[a4paper,hmargin=2cm,vmargin=3cm]{geometry}`)
	if want := mmToSP(210) - 2*parseDimenStr("2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2*2cm)", e.hsize, want)
	}
	if want := mmToSP(297) - 2*parseDimenStr("3cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2*3cm)", e.vsize, want)
	}
}

func TestGeometryAliasKeys(t *testing.T) {
	// lmargin/rmargin/tmargin/bmargin are aliases for left/right/top/bottom.
	e := runGeom(t, `\usepackage[a4paper,lmargin=1cm,rmargin=2cm,tmargin=3cm,bmargin=4cm]{geometry}`)
	if want := mmToSP(210) - parseDimenStr("1cm") - parseDimenStr("2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
	if want := mmToSP(297) - parseDimenStr("3cm") - parseDimenStr("4cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d", e.vsize, want)
	}
}

func TestGeometryPaperWidthHeight(t *testing.T) {
	e := runGeom(t, `\usepackage[paperwidth=20cm,paperheight=25cm,margin=1cm]{geometry}`)
	if want := parseDimenStr("20cm") - 2*parseDimenStr("1cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
	if want := parseDimenStr("25cm") - 2*parseDimenStr("1cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d", e.vsize, want)
	}
}

func TestGeometryReapply(t *testing.T) {
	// \geometry{...} re-applies onto the earlier \usepackage state: paper stays a4,
	// margins become 2cm (later wins).
	e := runGeom(t, `\usepackage[a4paper,margin=1in]{geometry}\geometry{margin=2cm}`)
	if want := mmToSP(210) - 2*parseDimenStr("2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (210mm-2*2cm)", e.hsize, want)
	}
	if want := mmToSP(297) - 2*parseDimenStr("2cm"); e.vsize != want {
		t.Errorf("vsize = %d, want %d (297mm-2*2cm)", e.vsize, want)
	}
}

func TestGeometryOtherPaperSizes(t *testing.T) {
	cases := []struct {
		name    string
		w, h    int
		keyword string
	}{
		{"a5", mmToSP(148), mmToSP(210), "a5paper"},
		{"b5", mmToSP(176), mmToSP(250), "b5paper"},
		{"legal", inToSP(8.5), inToSP(14), "legalpaper"},
		{"executive", inToSP(7.25), inToSP(10.5), "executivepaper"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := runGeom(t, `\usepackage[`+c.keyword+`,margin=1cm]{geometry}`)
			if want := c.w - 2*parseDimenStr("1cm"); e.hsize != want {
				t.Errorf("hsize = %d, want %d", e.hsize, want)
			}
			if want := c.h - 2*parseDimenStr("1cm"); e.vsize != want {
				t.Errorf("vsize = %d, want %d", e.vsize, want)
			}
		})
	}
}

func TestGeometryUnknownKeysIgnored(t *testing.T) {
	// Unknown keys and bare flags leave the layout at defaults (letterpaper, 1in).
	e := runGeom(t, `\usepackage[foo=3cm,bar,twoside,includehead]{geometry}`)
	if want := inToSP(8.5) - 2*inToSP(1); e.hsize != want {
		t.Errorf("hsize = %d, want %d (defaults preserved)", e.hsize, want)
	}
	if want := inToSP(11) - 2*inToSP(1); e.vsize != want {
		t.Errorf("vsize = %d, want %d (defaults preserved)", e.vsize, want)
	}
}

func TestGeometryMalformedDimenNoPanic(t *testing.T) {
	// A malformed dimension must be ignored, not panic, and not corrupt the layout:
	// margin stays at the 1in default.
	e := runGeom(t, `\usepackage[a4paper,margin=abc]{geometry}`)
	if want := mmToSP(210) - 2*inToSP(1); e.hsize != want {
		t.Errorf("hsize = %d, want %d (malformed margin ignored ⇒ 1in default)", e.hsize, want)
	}
	// An empty value is likewise ignored.
	e2 := runGeom(t, `\usepackage[a4paper,margin=]{geometry}`)
	if want := mmToSP(210) - 2*inToSP(1); e2.hsize != want {
		t.Errorf("empty margin: hsize = %d, want %d", e2.hsize, want)
	}
}

func TestGeometryCommandStandalone(t *testing.T) {
	// \geometry without a prior \usepackage initialises the state from defaults.
	e := runGeom(t, `\geometry{a4paper,margin=2cm}`)
	if want := mmToSP(210) - 2*parseDimenStr("2cm"); e.hsize != want {
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
	if want := mmToSP(210) - 2*parseDimenStr("1cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d", e.hsize, want)
	}
}

func TestGeometryTextThenMarginLastWins(t *testing.T) {
	// textwidth then a margin: the later margin re-establishes paper-based hsize.
	e := runGeom(t, `\usepackage[a4paper,textwidth=15cm]{geometry}\geometry{margin=2cm}`)
	if want := mmToSP(210) - 2*parseDimenStr("2cm"); e.hsize != want {
		t.Errorf("hsize = %d, want %d (margin should override textwidth)", e.hsize, want)
	}
}

func TestGeometryPortraitFlag(t *testing.T) {
	// portrait after landscape restores portrait orientation.
	e := runGeom(t, `\usepackage[a4paper,landscape,portrait,margin=1in]{geometry}`)
	if want := mmToSP(210) - 2*inToSP(1); e.hsize != want {
		t.Errorf("portrait hsize = %d, want %d", e.hsize, want)
	}
}

func TestGeometryEmptyAndBlankItems(t *testing.T) {
	// An empty option group and empty comma-separated items are skipped; the
	// layout stays at defaults (letterpaper, 1in).
	e := runGeom(t, `\geometry{}`)
	if want := inToSP(8.5) - 2*inToSP(1); e.hsize != want {
		t.Errorf("empty group: hsize = %d, want %d", e.hsize, want)
	}
	// Double comma yields an empty middle item that must be skipped.
	e2 := runGeom(t, `\usepackage[a4paper,,margin=2cm]{geometry}`)
	if want := mmToSP(210) - 2*parseDimenStr("2cm"); e2.hsize != want {
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
	// A bare number in geomDimen is points, matching parseDimenStr.
	if got, ok := geomDimen("30"); !ok || got != 30*unity {
		t.Errorf("geomDimen(30) = %d,%v, want %d,true", got, ok, 30*unity)
	}
	if _, ok := geomDimen("  "); ok {
		t.Errorf("geomDimen(blank) ok = true, want false")
	}
	if _, ok := geomDimen("in"); ok {
		t.Errorf("geomDimen(no number) ok = true, want false")
	}
	if _, ok := geomDimen("1.2.3in"); ok {
		t.Errorf("geomDimen(bad float) ok = true, want false")
	}
}
