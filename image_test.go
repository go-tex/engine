package engine

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

// A GIF (which the standard-library-only path could not decode) is loaded through
// the go-gfx codec and re-encoded to PNG for embedding: \includegraphics places it
// as an imageNode of format imgPNG at its intrinsic size, proving the broadened
// format support is wired.
func TestIncludeGraphicsGIFViaGoGfx(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{10, 200, 30, 255})
		}
	}
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	uri := "data:image/gif;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	e := New()
	if _, err := e.Run(`\noindent\includegraphics{` + uri + `}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	im, ok := firstImage(e.mvl)
	if !ok {
		t.Fatal("no imageNode placed (GIF was not decoded)")
	}
	if im.format != imgPNG {
		t.Errorf("format = %v, want imgPNG (GIF re-encoded)", im.format)
	}
	if im.width != 12*unity || im.height != 8*unity {
		t.Errorf("size = %d×%d sp, want 12×8 pt", im.width, im.height)
	}
}

// pngDataURI builds a w×h PNG and returns it as a base64 data: URI, so image tests
// need no files on disk (the same path the browser build uses).
func pngDataURI(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{200, 30, 60, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// firstImage returns the first imageNode reachable in a node tree.
func firstImage(nodes []node) (imageNode, bool) {
	for _, n := range nodes {
		switch c := n.(type) {
		case imageNode:
			return c, true
		case *boxNode:
			if im, ok := firstImage(c.list); ok {
				return im, true
			}
		}
	}
	return imageNode{}, false
}

// \includegraphics loads an image (here from a data: URI), sizes it from the
// options preserving aspect ratio, and places it as an image box on the baseline.
func TestIncludegraphics(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	uri := pngDataURI(t, 40, 20) // 2:1 aspect
	if _, err := e.Run(`\noindent\includegraphics[width=60pt]{` + uri + `}`); err != nil {
		t.Fatal(err)
	}
	im, ok := firstImage(e.mvl)
	if !ok {
		t.Fatal("no imageNode placed")
	}
	if im.format != imgPNG {
		t.Errorf("format = %v, want imgPNG", im.format)
	}
	if im.width != 60*unity {
		t.Errorf("width = %d sp, want 60pt", im.width)
	}
	if im.height != 30*unity { // 60pt × 20/40 = 30pt, aspect preserved
		t.Errorf("height = %d sp, want 30pt", im.height)
	}
}

// graphicsSize resolves intrinsic size, single-dimension aspect scaling, both
// dimensions, and scale.
func TestGraphicsSize(t *testing.T) {
	cases := []struct {
		iw, ih, wReq, hReq int
		scale              float64
		dpiX, dpiY         float64
		wantW, wantH       int
	}{
		{100, 50, 0, 0, 0, 0, 0, 100 * unity, 50 * unity},                  // intrinsic, no dpi (1px=1pt)
		{100, 50, 200 * unity, 0, 0, 0, 0, 200 * unity, 100 * unity},       // width → aspect height
		{100, 50, 0, 25 * unity, 0, 0, 0, 50 * unity, 25 * unity},          // height → aspect width
		{100, 50, 60 * unity, 40 * unity, 0, 0, 0, 60 * unity, 40 * unity}, // both explicit
		{100, 50, 0, 0, 2.0, 0, 0, 200 * unity, 100 * unity},               // scale, no dpi
		// A declared resolution converts pixels through px/dpi×72.27pt: a 144px
		// image at 144 dpi is 1in = 72.27pt, half its 1px=1pt fallback size.
		{144, 72, 0, 0, 0, 144, 144, natSP(144, 144), natSP(72, 144)},
		// dpi participates in scale and in single-dimension aspect (via natural size).
		{144, 72, 0, 0, 0.5, 144, 144, int(float64(natSP(144, 144))*0.5 + 0.5), int(float64(natSP(72, 144))*0.5 + 0.5)},
		{144, 72, 100 * unity, 0, 0, 144, 144, 100 * unity, 50 * unity}, // aspect from natural (2:1)
		{144, 72, 0, 50 * unity, 0, 144, 144, 100 * unity, 50 * unity},  // aspect from natural
		{0, 0, 0, 0, 0, 300, 300, 0, 0},                                 // zero pixels → zero box
	}
	for _, c := range cases {
		w, h := graphicsSize(c.iw, c.ih, c.wReq, c.hReq, c.scale, c.dpiX, c.dpiY)
		if w != c.wantW || h != c.wantH {
			t.Errorf("graphicsSize(%d,%d,%d,%d,%v,dpi %v/%v) = (%d,%d), want (%d,%d)",
				c.iw, c.ih, c.wReq, c.hReq, c.scale, c.dpiX, c.dpiY, w, h, c.wantW, c.wantH)
		}
	}
	// natSP with a zero resolution keeps the pixel-as-point fallback.
	if got := natSP(10, 0); got != 10*unity {
		t.Errorf("natSP(10,0) = %d, want %d", got, 10*unity)
	}
}

// parseDimenStr converts TeX dimension strings to scaled points; an unknown unit
// defaults to points.
func TestParseDimenStr(t *testing.T) {
	cases := map[string]int{
		"10pt": 10 * unity,
		"50":   50 * unity, // no unit → pt
		"1in":  ptToSP(72.27),
		"12pc": 144 * unity, // 1pc = 12pt
	}
	for in, want := range cases {
		if got := parseDimenStr(in); got != want {
			t.Errorf("parseDimenStr(%q) = %d, want %d", in, got, want)
		}
	}
	// cm/mm within one sp of the exact conversion.
	if got, want := parseDimenStr("2.54cm"), ptToSP(72.27); abs(got-want) > 1 {
		t.Errorf("parseDimenStr(2.54cm) = %d, want ~%d", got, want)
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
