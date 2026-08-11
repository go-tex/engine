package engine

import "testing"

// scaleMock is a font whose metrics scale linearly with its px size and which can
// re-face at any size — enough to exercise \large/\small/… (spMock can't rescale).
type scaleMock struct{ px int }

func (s scaleMock) charDimsSP(rune) (int, int, int) {
	return s.px * unity / 2, s.px * unity * 7 / 10, s.px * unity * 2 / 10
}
func (s scaleMock) spaceSP() glueSpec        { return glueSpec{width: s.px * unity * 3 / 10} }
func (scaleMock) glyphPathAt(rune) string    { return "M0 0" }
func (scaleMock) kernSP(_, _ rune) int       { return 0 }
func (s scaleMock) sizePt() int              { return s.px }
func (s scaleMock) atSizePx(px int) fontFace { return scaleMock{px: px} }

// Size commands scale the base font relative to \normalsize, glyphs carry the size
// they were set at, and { … } scoping reverts the size afterwards.
func TestFontSize(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(scaleMock{px: 10})
	if _, err := e.Run(`\noindent{\Large A}B{\small C}D`); err != nil {
		t.Fatal(err)
	}
	sizes := map[rune]int{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case charNode:
				if _, ok := sizes[c.ch]; !ok {
					sizes[c.ch] = c.size
				}
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	// \Large=1440‰→14, base=10, \small=900‰→9 ; B and D revert to 10.
	for ch, want := range map[rune]int{'A': 14, 'B': 10, 'C': 9, 'D': 10} {
		if sizes[ch] != want {
			t.Errorf("glyph %q size = %d, want %d", ch, sizes[ch], want)
		}
	}
}

// glyphScale gives the path scale factor relative to the render font's size.
func TestGlyphScale(t *testing.T) {
	base := scaleMock{px: 10}
	if got := glyphScale(charNode{size: 14}, base); got != 1.4 {
		t.Errorf("glyphScale(14/10) = %v, want 1.4", got)
	}
	if got := glyphScale(charNode{size: 10}, base); got != 1 {
		t.Errorf("glyphScale(10/10) = %v, want 1", got)
	}
	if got := glyphScale(charNode{size: 0}, base); got != 1 { // unset → no scaling
		t.Errorf("glyphScale(unset) = %v, want 1", got)
	}
}

// A font that can't rescale leaves the size unchanged (safe fallback).
func TestFontSizeUnscalable(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{}) // no atSizePx
	if _, err := e.Run(`\noindent{\Large A}B`); err != nil {
		t.Fatal(err)
	}
	// spMock reports size 10 throughout; \Large is a no-op, no crash.
	if e.baseFontPx != 10 {
		t.Errorf("base size = %d, want 10", e.baseFontPx)
	}
}
