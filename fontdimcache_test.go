package engine

import (
	"testing"

	texmath "github.com/go-tex/math"
)

// CharDims measures a glyph's y-extent by running the (CFF Type2) outline
// interpreter, which is expensive. A face is a fixed size, so a glyph's
// dimensions never change; CharDims must therefore memoise them. Without the
// cache a glyph-heavy document re-interprets the outline of every character it
// sets (rawAppendChar → charDimsSP → CharDims), which dominated run time (a
// systolic-geometry paper took ~110s). This proves the cache is populated on
// the first call and consulted on the second.
func TestCharDimsCached(t *testing.T) {
	f, err := NewOpenTypeFont(texmath.DefaultFont(), 10)
	if err != nil {
		t.Fatal(err)
	}
	const r = 'M'
	w, h, d := f.CharDims(r)
	if h == 0 && d == 0 {
		t.Fatalf("CharDims(%q) returned no ink extent; test glyph unsuitable", r)
	}
	got, ok := f.dimCache[r]
	if !ok {
		t.Fatalf("CharDims did not populate the cache for %q", r)
	}
	if got != [3]float64{w, h, d} {
		t.Fatalf("cached %v != returned (%v,%v,%v)", got, w, h, d)
	}
	// Poison the cache with a sentinel: a cache-consulting CharDims must return
	// it verbatim rather than recomputing from the outline.
	f.dimCache[r] = [3]float64{-1, -2, -3}
	if w2, h2, d2 := f.CharDims(r); w2 != -1 || h2 != -2 || d2 != -3 {
		t.Fatalf("CharDims recomputed instead of reading the cache: got (%v,%v,%v)", w2, h2, d2)
	}
}
