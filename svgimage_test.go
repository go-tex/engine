// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-pdfkit/pdfkit"
)

// svgDataURIB64 wraps SVG markup in a base64 data:image/svg+xml URI.
func svgDataURIB64(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

const sampleSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="40" height="20">` +
	`<rect x="0" y="0" width="40" height="20" fill="#3366cc"/>` +
	`<circle cx="30" cy="10" r="5" fill="red"/></svg>`

// \includegraphics with an inline SVG data URI places an imageNode of format "svg"
// sized from the SVG root (40×20), and [width=80pt] scales it to 80×40pt (aspect
// preserved). The SVG driver embeds it as a data:image/svg+xml <image>.
func TestIncludegraphicsSVG(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	uri := svgDataURIB64(sampleSVG)
	if _, err := e.Run(`\noindent\includegraphics[width=80pt]{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	im, ok := firstImage(e.mvl)
	if !ok {
		t.Fatal("no imageNode placed")
	}
	if im.format != "svg" {
		t.Errorf("format = %q, want svg", im.format)
	}
	if im.width != 80*unity {
		t.Errorf("width = %d sp, want 80pt", im.width)
	}
	if im.height != 40*unity { // 80pt × 20/40 = 40pt, aspect preserved
		t.Errorf("height = %d sp, want 40pt (aspect)", im.height)
	}
	if im.mime() != "image/svg+xml" {
		t.Errorf("mime = %q, want image/svg+xml", im.mime())
	}
	if !strings.HasPrefix(im.dataURI(), "data:image/svg+xml;base64,") {
		t.Errorf("dataURI prefix = %q", im.dataURI()[:40])
	}
	svg := e.RenderPage(2)
	if !strings.Contains(svg, `href="data:image/svg+xml;base64,`) {
		t.Errorf("SVG driver output does not embed the SVG image; got:\n%s", svg)
	}
}

// The intrinsic size may come from a file on disk and, absent width/height, from
// the viewBox.
func TestIncludegraphicsSVGFileViewBox(t *testing.T) {
	dir := t.TempDir()
	// Forward slashes: this path is embedded into TeX source
	// (\includegraphics{...}), where a backslash (the Windows separator) is the
	// escape char. TeX and Go's os.ReadFile both accept "/" on every platform.
	fp := filepath.ToSlash(filepath.Join(dir, "pic.svg"))
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 50">` +
		`<rect width="100" height="50" fill="green"/></svg>`
	if err := os.WriteFile(fp, []byte(svg), 0644); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics{` + fp + `}`); err != nil {
		t.Fatal(err)
	}
	im, ok := firstImage(e.mvl)
	if !ok {
		t.Fatal("no imageNode placed")
	}
	if im.format != "svg" {
		t.Errorf("format = %q, want svg", im.format)
	}
	if im.width != 100*unity || im.height != 50*unity { // intrinsic, 1 user unit = 1pt
		t.Errorf("intrinsic size = %d×%d sp, want 100×50pt", im.width, im.height)
	}
}

// A non-base64 (percent/plain-text) data URI is accepted too.
func TestIncludegraphicsSVGPlainDataURI(t *testing.T) {
	uri := "data:image/svg+xml," +
		`<svg xmlns='http://www.w3.org/2000/svg' width='30' height='30'><rect width='30' height='30'/></svg>`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	im, ok := firstImage(e.mvl)
	if !ok {
		t.Fatal("no imageNode placed")
	}
	if im.format != "svg" || im.width != 30*unity || im.height != 30*unity {
		t.Errorf("got format=%q size=%d×%d, want svg 30×30pt", im.format, im.width, im.height)
	}
}

// Content sniffed as SVG (no .svg extension, no svg media type) is still routed to
// the SVG loader.
func TestIncludegraphicsSVGSniffed(t *testing.T) {
	uri := "data:text/plain;base64," +
		base64.StdEncoding.EncodeToString([]byte(sampleSVG))
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent\includegraphics{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	im, ok := firstImage(e.mvl)
	if !ok || im.format != "svg" {
		t.Fatalf("expected a sniffed svg image, got ok=%v format=%q", ok, im.format)
	}
}

// Malformed SVG, a bad base64 payload and a missing file each fail with a
// SourceError (no panic).
func TestIncludegraphicsSVGErrors(t *testing.T) {
	cases := map[string]string{
		"truncated svg": svgDataURIB64(`<svg width="40"`), // no '>' → decoder error
		"bad base64":    "data:image/svg+xml;base64,@@@not-base64@@@",
		// Forward slashes: embedded into TeX source below, where a backslash
		// (the Windows separator) is the escape char.
		"missing file":    filepath.ToSlash(filepath.Join(t.TempDir(), "nope.svg")),
		"non-svg-non-img": svgDataURIB64(`<html><body>hi</body></svg>`), // sniffed svg, bad root
	}
	for name, arg := range cases {
		t.Run(name, func(t *testing.T) {
			e := New()
			e.LoadLaTeX()
			e.SetFont(spMock{})
			_, err := e.Run(`\noindent\includegraphics{` + arg + `}`)
			if err == nil {
				t.Fatalf("%s: expected a SourceError, got nil", name)
			}
			var se SourceError
			if !errors.As(err, &se) {
				t.Fatalf("%s: error is %T, want SourceError", name, err)
			}
		})
	}
}

func TestLoadSVGImageMalformed(t *testing.T) {
	if _, _, _, _, err := loadSVGImage([]byte("not xml at all")); err != errSVG {
		t.Errorf("err = %v, want errSVG", err)
	}
	// A valid non-svg root also fails.
	if _, _, _, _, err := loadSVGImage([]byte(`<html></html>`)); err != errSVG {
		t.Errorf("non-svg root err = %v, want errSVG", err)
	}
}

func TestSVGIntrinsic(t *testing.T) {
	cases := []struct {
		attrs      map[string]string
		wantW, wtH int
	}{
		{map[string]string{"width": "40", "height": "20"}, 40, 20},
		{map[string]string{"width": "40px", "height": "20px"}, 40, 20},
		{map[string]string{"viewBox": "0 0 100 50"}, 100, 50},
		{map[string]string{"width": "12"}, 12, 12},            // width only → square
		{map[string]string{"height": "8"}, 8, 8},              // height only → square
		{map[string]string{}, svgDefaultSize, svgDefaultSize}, // nothing → default
	}
	for _, c := range cases {
		w, h := svgIntrinsic(c.attrs)
		if w != c.wantW || h != c.wtH {
			t.Errorf("svgIntrinsic(%v) = %d×%d, want %d×%d", c.attrs, w, h, c.wantW, c.wtH)
		}
	}
}

func TestSVGViewport(t *testing.T) {
	if x, y, w, h := svgViewport(map[string]string{"viewBox": "1 2 30 40"}); x != 1 || y != 2 || w != 30 || h != 40 {
		t.Errorf("viewBox viewport = %v,%v,%v,%v", x, y, w, h)
	}
	if _, _, w, h := svgViewport(map[string]string{"width": "10", "height": "5"}); w != 10 || h != 5 {
		t.Errorf("width/height viewport = %v×%v, want 10×5", w, h)
	}
	if _, _, w, h := svgViewport(map[string]string{}); w != svgDefaultSize || h != svgDefaultSize {
		t.Errorf("default viewport = %v×%v", w, h)
	}
}

func TestParseSVGLen(t *testing.T) {
	cases := map[string]float64{
		"40": 40, "40px": 40, "2.5pt": 2.5, "-3": -3, "": 0, "auto": 0, "1e2": 100,
	}
	for in, want := range cases {
		if got := parseSVGLen(in); got != want {
			t.Errorf("parseSVGLen(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseSVGFill(t *testing.T) {
	type r struct {
		c    uint32
		none bool
		ok   bool
	}
	cases := map[string]r{
		"":             {0, false, false},
		"none":         {0, true, true},
		"transparent":  {0, true, true},
		"inherit":      {0, false, false},
		"currentColor": {0, false, false},
		"#f00":         {0xff0000, false, true},
		"#3366cc":      {0x3366cc, false, true},
		"#zz":          {0, false, false}, // bad hex length
		"#gggggg":      {0, false, false}, // bad hex digits
		"rgb(255,0,0)": {0xff0000, false, true},
		"rgb(0,128,0)": {0x008000, false, true},
		"rgb(1,2)":     {0, false, false}, // wrong arity
		"red":          {0xff0000, false, true},
		"teal":         {0x008080, false, true},
		"chartreuse":   {0, false, false}, // unknown name
	}
	for in, want := range cases {
		c, none, ok := parseSVGFill(in)
		if c != want.c || none != want.none || ok != want.ok {
			t.Errorf("parseSVGFill(%q) = (%#x,%v,%v), want (%#x,%v,%v)", in, c, none, ok, want.c, want.none, want.ok)
		}
	}
}

func TestParseHexColor(t *testing.T) {
	if c, ok := parseHexColor("fff"); !ok || c != 0xffffff {
		t.Errorf("parseHexColor(fff) = %#x,%v", c, ok)
	}
	if c, ok := parseHexColor("112233"); !ok || c != 0x112233 {
		t.Errorf("parseHexColor(112233) = %#x,%v", c, ok)
	}
	if _, ok := parseHexColor("xyz"); ok {
		t.Error("parseHexColor(xyz) should fail")
	}
	if _, ok := parseHexColor("qqqqqq"); ok {
		t.Error("parseHexColor(qqqqqq) should fail")
	}
	if _, ok := parseHexColor("12"); ok {
		t.Error("parseHexColor(12) should fail on length")
	}
}

func TestClamp255(t *testing.T) {
	if clamp255(-5) != 0 || clamp255(300) != 255 || clamp255(128) != 128 {
		t.Errorf("clamp255 out of range: %d %d %d", clamp255(-5), clamp255(300), clamp255(128))
	}
}

func TestLooksLikeSVGAndDataURI(t *testing.T) {
	if !looksLikeSVG([]byte("  \n<?xml version='1.0'?><svg></svg>")) {
		t.Error("prolog + <svg> should look like SVG")
	}
	if looksLikeSVG([]byte("\x89PNG\r\n\x1a\n")) {
		t.Error("PNG magic should not look like SVG")
	}
	// <svg beyond the 512-byte sniff window is not detected.
	far := append(bytes.Repeat([]byte(" "), 600), []byte("<svg/>")...)
	if looksLikeSVG(far) {
		t.Error("<svg> past the sniff window should not match")
	}
	if !svgDataURI("data:image/svg+xml;base64,AAA") {
		t.Error("svg media type should be detected")
	}
	if svgDataURI("data:image/png;base64,AAA") {
		t.Error("png media type should not be an svg URI")
	}
	if svgDataURI("no-comma-here") {
		t.Error("a data URI without a comma is not an svg URI")
	}
}

func TestDecodeDataURI(t *testing.T) {
	// base64 payload
	if b, err := decodeDataURI("data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("hi"))); err != nil || string(b) != "hi" {
		t.Errorf("base64 decode = %q,%v", b, err)
	}
	// percent-encoded plain payload
	if b, err := decodeDataURI("data:image/svg+xml,%3Csvg%3E"); err != nil || string(b) != "<svg>" {
		t.Errorf("plain decode = %q,%v", b, err)
	}
	// invalid percent escape falls back to raw payload
	if b, err := decodeDataURI("data:image/svg+xml,%zz"); err != nil || string(b) != "%zz" {
		t.Errorf("raw fallback = %q,%v", b, err)
	}
	// no comma → error
	if _, err := decodeDataURI("data:nope"); err != errDataURI {
		t.Errorf("no-comma err = %v, want errDataURI", err)
	}
}

// drawSVGImage draws the supported element set into a real PDF page without panics,
// producing a non-trivial content stream.
func TestDrawSVGImagePDF(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<g fill="#204080" transform="translate(0,0) scale(1,1)">` +
		`<rect x="10" y="10" width="30" height="30" fill="red"/>` +
		`<circle cx="70" cy="30" r="15" fill="green"/>` +
		`<ellipse cx="30" cy="70" rx="20" ry="10" fill="rgb(10,20,30)"/>` +
		`<polygon points="60,60 90,60 75,90" fill="orange"/>` +
		`<path d="M5 95 L95 95 L50 50 Z"/>` + // inherits g fill
		`<rect x="0" y="0" width="5" height="5" fill="none"/>` + // skipped
		`<line x1="0" y1="0" x2="10" y2="10"/>` + // unsupported, skipped
		`</g></svg>`
	doc := pdfkit.New(pdfkit.Options{})
	p := doc.AddPage(pdfkit.NewPageSize(200, 200))
	drawSVGImage(p, []byte(svg), pdfkit.Rect{X: 20, Y: 20, Width: 120, Height: 120})
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a valid PDF (%d bytes)", len(b))
	}
}

// drawSVGImage on a malformed or zero-viewport SVG is a no-op (guards no panic).
func TestDrawSVGImageDegenerate(t *testing.T) {
	doc := pdfkit.New(pdfkit.Options{})
	p := doc.AddPage(pdfkit.NewPageSize(50, 50))
	drawSVGImage(p, []byte("garbage"), pdfkit.Rect{X: 0, Y: 0, Width: 10, Height: 10})           // no root
	drawSVGImage(p, []byte(`<svg viewBox="0 0 0 0"></svg>`), pdfkit.Rect{Width: 10, Height: 10}) // zero viewport
	// Unbalanced closing tags (Strict=false) must not underflow the frame stack.
	drawSVGImage(p, []byte(`<svg width="10" height="10"><rect/></g></g></svg>`), pdfkit.Rect{Width: 10, Height: 10})
	drawSVGPolygon(p, "1,2", identity(), 0, 0)      // too few points
	drawSVGEllipse(p, 0, 0, 0, 0, identity(), 0, 0) // zero radii
}

// The full PDF driver draws an included SVG as vectors (integration path through
// pdfDraw.drawImage). Needs an embeddable system font.
func TestRenderPDFWithSVGImage(t *testing.T) {
	fp := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	if _, err := os.Stat(fp); err != nil {
		t.Skip("no system font")
	}
	dir := t.TempDir()
	// Forward slashes: this path is embedded into TeX source
	// (\includegraphics{...}), where a backslash (the Windows separator) is the
	// escape char. TeX and Go's os.ReadFile both accept "/" on every platform.
	pic := filepath.ToSlash(filepath.Join(dir, "pic.svg"))
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="60" height="40">` +
		`<rect width="60" height="40" fill="#88bbdd"/>` +
		`<circle cx="30" cy="20" r="12" fill="crimson"/></svg>` // crimson is unknown → default black
	if err := os.WriteFile(pic, []byte(svg), 0644); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=300pt \baselineskip=15pt `)
	if _, err := e.Run(`\noindent A picture \includegraphics[width=120pt]{` + pic + `} here.\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("not a valid PDF (%d bytes)", len(b))
	}
}
