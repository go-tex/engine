package engine

import (
	"image"
	"image/color"
	"testing"
)

// \includepdf (pdfpages) appends pages of an existing PDF. Undefined, it dropped
// them in silence: 2312.05895 ships a 12-page supplement that way.
//
// The seam is the one \includegraphics already uses for a PDF figure, with a page
// number added. These tests inject it, so they need no renderer.
func withStubPageRasterizer(t *testing.T, calls *[]int) {
	t.Helper()
	saveR, saveN := RasterizePDFPage, PDFPageCount
	RasterizePDFPage = func(data []byte, page int, dpi float64) (image.Image, error) {
		*calls = append(*calls, page)
		img := image.NewRGBA(image.Rect(0, 0, 60, 80))
		img.Set(1, 1, color.RGBA{0, 0, 0, 255})
		return img, nil
	}
	PDFPageCount = func(data []byte) int { return 12 }
	t.Cleanup(func() { RasterizePDFPage, PDFPageCount = saveR, saveN })
}

func TestIncludepdfAsksForEachRequestedPage(t *testing.T) {
	for _, c := range []struct {
		opts string
		want []int
	}{
		{`[pages={1,2}]`, []int{1, 2}},
		{`[pages={2-5}]`, []int{2, 3, 4, 5}},
		{`[pages=3]`, []int{3}},
		{`[pages={1,3,7}]`, []int{1, 3, 7}},
		// An empty entry is pdfpages' EMPTY page; we have nothing to put on it.
		{`[pages={1,,3}]`, []int{1, 3}},
		// Options we accept and ignore must not eat the page list.
		{`[angle=90,pages={4,5},fitpaper]`, []int{4, 5}},
		// No options at all: every page.
		{``, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}},
	} {
		var calls []int
		withStubPageRasterizer(t, &calls)
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\includepdf` + c.opts + `{testdata/stub.pdf}`); err != nil {
			t.Fatalf("%s: %v", c.opts, err)
		}
		if len(calls) != len(c.want) {
			t.Errorf("%s asked for %v, want %v", c.opts, calls, c.want)
			continue
		}
		for i := range calls {
			if calls[i] != c.want[i] {
				t.Errorf("%s asked for %v, want %v", c.opts, calls, c.want)
				break
			}
		}
	}
}

// resolvePageList is the part that decides what is included, and an UNKNOWN page
// count must not clamp it to nothing — bounding a list against zero is how a
// byte-counting heuristic silently dropped every page but the first.
func TestResolvePageListDoesNotClampAgainstZero(t *testing.T) {
	if got := resolvePageList("1,2", 0); len(got) != 0 {
		t.Errorf("with a zero count the caller retries unbounded; got %v", got)
	}
	if got := resolvePageList("1,2", 1<<20); len(got) != 2 {
		t.Errorf("unbounded = %v, want two pages", got)
	}
	if got := resolvePageList("-", 0); got != nil {
		t.Errorf(`"every page" of an unknown count = %v, want nothing`, got)
	}
}
