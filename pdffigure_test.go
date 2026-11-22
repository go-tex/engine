// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"errors"
	"image"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The witness is testdata/pdffig/witness.tex: a 100x60bp PDF figure
// (testdata/pdffig/fig.pdf, made by mkfig.tex) set with width=60pt at the
// left margin of an article page. Tectonic (XeTeX's xdvipdfmx) puts it in the
// output as a Form XObject with /BBox [0 0 100 60] and draws it with
//
//	q .59778 0 0 .59778 24.84447 -35.66428 cm  q 1 0 0 1 61.76837 -88.66859 cm /Fm0 Do
//
// that is, one uniform scale of 59.778bp / 100 = .59778 (60 TeX pt is 59.778 bp),
// the form's lower-left corner at the text area's left edge (61.768bp, the
// article.cls left margin) and its top on the first line's top. The numbers
// below are read from that reference PDF (qpdf --qdf), not from this engine.
const (
	tectonicScale = 0.59778
	tectonicBBoxW = 100
	tectonicBBoxH = 60
)

var fmDo = regexp.MustCompile(`(?m)^q\n([-0-9.]+) ([-0-9.]+) ([-0-9.]+) ([-0-9.]+) ([-0-9.]+) ([-0-9.]+) cm\n/Fm0 Do\nQ$`)

func renderWitness(t *testing.T, src string) []byte {
	t.Helper()
	e := New()
	e.SetFont(testFont(t))
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 10); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPDFFigureIsAFormXObject(t *testing.T) {
	pdf := renderWitness(t, `\noindent\includegraphics[width=60pt]{testdata/pdffig/fig.pdf}\par`)
	if !regexp.MustCompile(`/Subtype\s*/Form`).Match(pdf) {
		t.Fatal("no Form XObject in the output")
	}
	bb := regexp.MustCompile(`/BBox\s*\[\s*0\s+0\s+(\d+)\s+(\d+)\s*\]`).FindSubmatch(pdf)
	if bb == nil || string(bb[1]) != strconv.Itoa(tectonicBBoxW) || string(bb[2]) != strconv.Itoa(tectonicBBoxH) {
		t.Fatalf("form /BBox = %q, want [0 0 %d %d]", bb, tectonicBBoxW, tectonicBBoxH)
	}
	content := pdfContent(t, pdf)
	m := fmDo.FindStringSubmatch(content)
	if m == nil {
		t.Fatalf("no 'cm /Fm0 Do' in the page content:\n%s", content)
	}
	var v [6]float64
	for i := range v {
		v[i], _ = strconv.ParseFloat(m[i+1], 64)
	}
	// The page matrix (TeX pt to bp) is applied before this one.
	const bp = 72.0 / 72.27
	if got := v[0] * bp; math.Abs(got-tectonicScale) > 1e-4 {
		t.Errorf("x scale = %.5f bp per unit, tectonic %.5f", got, tectonicScale)
	}
	if got := v[3] * bp; math.Abs(got-tectonicScale) > 1e-4 {
		t.Errorf("y scale = %.5f bp per unit, tectonic %.5f", got, tectonicScale)
	}
	if v[1] != 0 || v[2] != 0 {
		t.Errorf("figure is sheared or turned: %v", v)
	}
	// Left edge on the text area's edge (the render margin is 10 TeX pt).
	if v[4] != 10 {
		t.Errorf("left edge at %v, want the 10pt margin", v[4])
	}
	// Top edge on the first line's top: the form is 36pt tall and the page
	// content area starts 10pt below the top, so the bottom is 46pt down.
	mb := regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([0-9.]+)\s+([0-9.]+)\s*\]`).FindSubmatch(pdf)
	if mb == nil {
		t.Fatal("no MediaBox")
	}
	pageH, _ := strconv.ParseFloat(string(mb[2]), 64)
	if top := v[5] + v[3]*tectonicBBoxH; math.Abs(top-(pageH/bp-10)) > 0.05 {
		t.Errorf("top edge at %.3f, want %.3f (10pt below the page top)", top, pageH/bp-10)
	}
}

// A PDF the importer refuses keeps the box it had (the page is not lost).
func TestPDFFigureRefusedStaysEmptyBox(t *testing.T) {
	e := New()
	e.lenient = true
	e.SetFont(testFont(t))
	if _, err := e.Run(`\noindent\includegraphics[width=60pt,height=30pt]{data:application/pdf;base64,JVBERi0xLjQKJSAvTWVkaWFCb3hbMCAwIDYwMCAyMDBdCiUlRU9GCg==}\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 10); err != nil {
		t.Fatal(err)
	}
	pdf := buf.Bytes()
	if regexp.MustCompile(`/Subtype\s*/Form`).Match(pdf) {
		t.Error("a refused PDF produced a Form XObject")
	}
}

func TestNativePDFFigureSize(t *testing.T) {
	data, err := os.ReadFile("testdata/pdffig/fig.pdf")
	if err != nil {
		t.Fatal(err)
	}
	w, h, ok := nativePDFFigure(data, 1)
	if !ok || w != 100*pdfFigurePrecision || h != 60*pdfFigurePrecision {
		t.Errorf("nativePDFFigure = %d, %d, %v; want a 100x60bp page", w, h, ok)
	}
	for _, bad := range [][]byte{nil, []byte("%PDF-1.4\n% /MediaBox[0 0 600 200]\n%%EOF\n")} {
		if w, h, ok := nativePDFFigure(bad, 1); ok || w != 0 || h != 0 {
			t.Errorf("nativePDFFigure(%q) = %d, %d, %v; want a refusal", bad, w, h, ok)
		}
	}
}

func TestPDFSVGFallbacks(t *testing.T) {
	defer func(f func([]byte, float64) (image.Image, error)) { RasterizePDF = f }(RasterizePDF)
	n := imageNode{data: []byte("x")}
	cases := []struct {
		name string
		fn   func([]byte, float64) (image.Image, error)
		want string
	}{
		{"none wired", nil, "<rect "},
		{"rasteriser fails", func([]byte, float64) (image.Image, error) { return nil, errors.New("no") }, "<rect "},
		{"picture cannot be encoded", func([]byte, float64) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 0, 0)), nil }, "<rect "},
		{"picture", func([]byte, float64) (image.Image, error) { return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil }, "<image "},
	}
	for _, c := range cases {
		RasterizePDF = c.fn
		if got := n.pdfSVG("0", "0", "1", "1"); !strings.HasPrefix(got, c.want) {
			t.Errorf("%s: %s, want a prefix %s", c.name, got, c.want)
		}
	}
}

// testdata/pdffig/fig2.pdf has two pages: 100x60bp, then 200x50bp. Tectonic
// (witness2.tex, page=2, width=60pt) includes the second as a Form XObject with
// /BBox [0 0 200 50] drawn at one scale of .29887 (60 TeX pt over 200 units), and
// stops with "image inclusion failed for fig2.pdf (page=3)" for a page past the end
// (witness3.tex). Those are the values asserted here.
func TestPDFFigurePageN(t *testing.T) {
	pdf := renderWitness(t, `\noindent\includegraphics[page=2,width=60pt]{testdata/pdffig/fig2.pdf}\par`)
	bb := regexp.MustCompile(`/BBox\s*\[\s*0\s+0\s+(\d+)\s+(\d+)\s*\]`).FindAllSubmatch(pdf, -1)
	if len(bb) != 1 || string(bb[0][1]) != "200" || string(bb[0][2]) != "50" {
		t.Fatalf("form /BBox = %q, want exactly [0 0 200 50]", bb)
	}
	m := fmDo.FindStringSubmatch(pdfContent(t, pdf))
	if m == nil {
		t.Fatal("no 'cm /Fm0 Do' in the page content")
	}
	a, _ := strconv.ParseFloat(m[1], 64)
	if got := a * 72 / 72.27; math.Abs(got-0.29887) > 1e-4 {
		t.Errorf("scale = %.5f, tectonic .29887", got)
	}
	// Without page= the first page is the figure.
	first := renderWitness(t, `\noindent\includegraphics[width=60pt]{testdata/pdffig/fig2.pdf}\par`)
	if !regexp.MustCompile(`/BBox\s*\[\s*0\s+0\s+100\s+60\s*\]`).Match(first) {
		t.Error("no page= did not give page 1")
	}
}

func TestPDFFigurePageOutOfRange(t *testing.T) {
	for _, page := range []string{"3", "0", "-1"} {
		src := `\includegraphics[page=` + page + `,width=60pt]{testdata/pdffig/fig2.pdf}`
		e := New()
		e.SetFont(testFont(t))
		if _, err := e.Run(src); err == nil || !strings.Contains(err.Error(), "page "+page) {
			t.Errorf("page=%s: err = %v, want a refusal naming the page", page, err)
		}
		// Tolerant mode keeps the box and draws nothing in it.
		l := New()
		l.lenient = true
		l.SetFont(testFont(t))
		if _, err := l.Run(src + `\par`); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := l.RenderPDF(&buf, 10); err != nil {
			t.Fatal(err)
		}
		if regexp.MustCompile(`/Subtype\s*/Form`).Match(buf.Bytes()) {
			t.Errorf("page=%s: a form was drawn", page)
		}
	}
	// A page that is not a number is ignored, as an unknown key is.
	pdf := renderWitness(t, `\includegraphics[page=x,width=60pt]{testdata/pdffig/fig2.pdf}\par`)
	if !regexp.MustCompile(`/BBox\s*\[\s*0\s+0\s+100\s+60\s*\]`).Match(pdf) {
		t.Error("page=x did not fall back to page 1")
	}
}
