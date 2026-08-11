package engine

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

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
	if im.format != "png" {
		t.Errorf("format = %q, want png", im.format)
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
		wantW, wantH       int
	}{
		{100, 50, 0, 0, 0, 100 * unity, 50 * unity},                  // intrinsic (1px=1pt)
		{100, 50, 200 * unity, 0, 0, 200 * unity, 100 * unity},       // width → aspect height
		{100, 50, 0, 25 * unity, 0, 50 * unity, 25 * unity},          // height → aspect width
		{100, 50, 60 * unity, 40 * unity, 0, 60 * unity, 40 * unity}, // both explicit
		{100, 50, 0, 0, 2.0, 200 * unity, 100 * unity},               // scale
	}
	for _, c := range cases {
		w, h := graphicsSize(c.iw, c.ih, c.wReq, c.hReq, c.scale)
		if w != c.wantW || h != c.wantH {
			t.Errorf("graphicsSize(%d,%d,%d,%d,%v) = (%d,%d), want (%d,%d)",
				c.iw, c.ih, c.wReq, c.hReq, c.scale, w, h, c.wantW, c.wantH)
		}
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
