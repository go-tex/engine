// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"encoding/base64"
	"testing"
)

// pdfIntrinsicPoints reads the page box (CropBox preferred, else MediaBox) as a
// literal array, tolerates the whitespace real producers emit, prefers the
// CropBox even when the MediaBox comes first, and reports failure on a missing or
// degenerate box.
func TestPDFIntrinsicPoints(t *testing.T) {
	cases := []struct {
		name   string
		body   string
		wantW  float64
		wantH  float64
		wantOK bool
	}{
		{name: "mediabox no spaces", // matplotlib/pdfcrop style
			body: "%PDF-1.5\n/MediaBox[ 0 0 875.52 300.24]\n", wantW: 875.52, wantH: 300.24, wantOK: true},
		{name: "mediabox spaced",
			body: "%PDF-1.5\n/MediaBox [ 0 0 423.5626136364 217.391875 ]\n", wantW: 423.5626136364, wantH: 217.391875, wantOK: true},
		{name: "cropbox preferred over earlier mediabox",
			body: "%PDF\n/MediaBox [0 0 612 792]\n/CropBox [10 20 210 320]\n", wantW: 200, wantH: 300, wantOK: true},
		{name: "nonzero origin",
			body: "/MediaBox [ 100 50 300 350 ]", wantW: 200, wantH: 300, wantOK: true},
		// A turned page is shown turned, so its size as displayed has width and
		// height the other way round from the box as written. These are the two
		// shapes the corpus holds: 2307.08085's web_manage2.pdf (595x842, /Rotate
		// 90) and 2408.03452's ufront-a100.pdf (612x792, /Rotate 90).
		{name: "quarter turn swaps the sides",
			body: "%PDF-1.5\n/MediaBox [0 0 595 842]\n/Rotate 90\n", wantW: 842, wantH: 595, wantOK: true},
		{name: "three quarter turn swaps them too",
			body: "%PDF-1.5\n/MediaBox [0 0 612 792]\n/Rotate 270\n", wantW: 792, wantH: 612, wantOK: true},
		{name: "a negative rotation is the same turn",
			body: "%PDF-1.5\n/MediaBox [0 0 612 792]\n/Rotate -90\n", wantW: 792, wantH: 612, wantOK: true},
		{name: "a half turn leaves the sides alone",
			body: "%PDF-1.5\n/MediaBox [0 0 612 792]\n/Rotate 180\n", wantW: 612, wantH: 792, wantOK: true},
		{name: "an upright page states zero",
			body: "%PDF-1.5\n/MediaBox [0 0 612 792]\n/Rotate 0\n", wantW: 612, wantH: 792, wantOK: true},
		{name: "a rotation the spec does not allow is ignored",
			body: "%PDF-1.5\n/MediaBox [0 0 612 792]\n/Rotate 45\n", wantW: 612, wantH: 792, wantOK: true},
		{name: "no box", body: "%PDF-1.4\nnothing here\n", wantOK: false},
		{name: "degenerate zero-height", body: "/MediaBox [0 0 100 0]", wantOK: false},
	}
	for _, c := range cases {
		w, h, ok := pdfIntrinsicPoints([]byte(c.body))
		if ok != c.wantOK {
			t.Errorf("%s: ok=%v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if !approx(w, c.wantW) || !approx(h, c.wantH) {
			t.Errorf("%s: (%v,%v), want (%v,%v)", c.name, w, h, c.wantW, c.wantH)
		}
	}
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}

// A width-only \includegraphics of an un-rasterisable PDF figure now reserves a
// placeholder with the figure's TRUE aspect (from its page box), not a square.
// This is what lets the surrounding text paginate as it would with the real
// figure. Verified end-to-end: a 3:1 (wide) box gives a 1/3-tall placeholder.
func TestPDFPlaceholderKeepsAspect(t *testing.T) {
	t.Setenv("GOTEX_PDFASPECT", "1") // aspect sizing is opt-in (see pdfAspectOptIn)
	// A minimal PDF whose only feature that matters here is its MediaBox: 600×200
	// user units = a 3:1 landscape figure.
	pdf := "%PDF-1.4\n1 0 obj<</Type/Page/MediaBox[0 0 600 200]>>endobj\ntrailer<<>>\n%%EOF\n"
	uri := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte(pdf))

	e := New()
	e.lenient = true // the placeholder path is taken only in tolerant mode
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics[width=120pt]{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no placeholder frameNode placed for the un-rasterisable PDF")
	}
	// 120pt wide, 3:1 box ⇒ 40pt tall (the old blind-square placeholder was 120pt).
	if placeholderW(fr) != 120*unity {
		t.Errorf("placeholder width = %d sp, want 120pt", placeholderW(fr))
	}
	if got, want := placeholderH(fr), 40*unity; !within(got, want, unity/2) {
		t.Errorf("placeholder height = %d sp, want ~%d (aspect-correct, not a %d square)",
			got, want, 120*unity)
	}
}

// A PDF figure with NO recoverable box keeps the default-sized placeholder — the
// aspect fix never makes the un-boxed case worse.
func TestPDFPlaceholderNoBoxDefault(t *testing.T) {
	pdf := "%PDF-1.4\nno box at all\n%%EOF\n"
	uri := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte(pdf))
	e := New()
	e.lenient = true // the placeholder path is taken only in tolerant mode
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no placeholder frameNode placed")
	}
	if placeholderW(fr) != 120*unity || placeholderH(fr) != 90*unity {
		t.Errorf("default placeholder = %dx%d sp, want 120x90pt",
			placeholderW(fr), placeholderH(fr))
	}
}

// With the opt-in OFF (the corpus default) a width-only PDF placeholder stays the
// legacy blind square — the aspect sizing must not change the default at all.
func TestPDFPlaceholderAspectGatedOff(t *testing.T) {
	// Ensure the env is unset for this test even if the runner has it exported.
	t.Setenv("GOTEX_PDFASPECT", "")
	pdf := "%PDF-1.4\n1 0 obj<</Type/Page/MediaBox[0 0 600 200]>>endobj\ntrailer<<>>\n%%EOF\n"
	uri := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte(pdf))
	e := New()
	e.lenient = true
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics[width=120pt]{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	fr, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no placeholder frameNode placed")
	}
	if placeholderW(fr) != 120*unity || placeholderH(fr) != 120*unity {
		t.Errorf("gated-off placeholder = %dx%d sp, want the 120x120 square (unchanged default)",
			placeholderW(fr), placeholderH(fr))
	}
}

func within(a, b, tol int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

// placeholderW / placeholderH are a figure placeholder's OUTER dimensions — what
// the page actually gives up for it.
//
// The frame is drawn INSIDE the space the figure asked for (image.go), so the
// inner box is smaller than the request by 2*(fboxSep+fboxRule) and it is the
// outer box that answers "how much room does this figure take". These read the
// same thing the page builder reads.
func placeholderW(fr frameNode) int { return fr.width() }
func placeholderH(fr frameNode) int { return fr.height() + fr.depth() }
