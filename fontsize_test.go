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

// firstGlyphSize returns the point size recorded on the first charNode in the
// engine's main vertical list (0 if none) — the size at which body text is set.
func firstGlyphSize(e *Engine) int {
	var found int
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			if found != 0 {
				return
			}
			switch c := n.(type) {
			case charNode:
				found = c.size
				return
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return found
}

// A [10pt]/[11pt]/[12pt] document class sets \normalsize body text at 10/11/12pt
// (110%/120% of the 10pt design) and \baselineskip at the size clo's leading
// (12/13.6/14.5pt), so 11/12pt text wraps like real LaTeX. 10pt is byte-identical
// to the pre-existing 100% behaviour.
func TestClassBaseSize(t *testing.T) {
	for _, tc := range []struct {
		opt        string
		wantSize   int
		wantBLskip int
	}{
		{"10pt", 10, 12 * unity},
		{"11pt", 11, ptToSP(13.6)},
		{"12pt", 12, ptToSP(14.5)},
	} {
		src := `\documentclass[` + tc.opt + `]{article}\begin{document}Hello world.\end{document}`
		e, err := compile([]byte(src), Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.opt, err)
		}
		if got := firstGlyphSize(e); got != tc.wantSize {
			t.Errorf("[%s] body glyph size = %dpt, want %dpt", tc.opt, got, tc.wantSize)
		}
		if e.baselineskip != tc.wantBLskip {
			t.Errorf("[%s] \\baselineskip = %d sp, want %d sp", tc.opt, e.baselineskip, tc.wantBLskip)
		}
		if e.baseBaselineskip != tc.wantBLskip {
			t.Errorf("[%s] baseBaselineskip = %d sp, want %d sp", tc.opt, e.baseBaselineskip, tc.wantBLskip)
		}
	}
}

// scaleClassFontsToBase re-faces the base and the bound family faces to the class
// base size, so a [12pt] class sets 12pt body text; \large/\small then scale off
// the new base through \gotexsize.
func TestClassScaledFonts(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(scaleMock{px: 10})
	e.bindFont("bf", scaleMock{px: 10})
	e.scaleClassFontsToBase(1200) // [12pt]
	if e.baseFontPx != 12 {
		t.Fatalf("baseFontPx = %d, want 12", e.baseFontPx)
	}
	if e.baseFont.sizePt() != 12 || e.curFont.sizePt() != 12 {
		t.Errorf("base/cur size = %d/%d, want 12/12", e.baseFont.sizePt(), e.curFont.sizePt())
	}
	if bf := e.eq["bf"]; bf == nil || bf.font.sizePt() != 12 {
		t.Errorf("\\bf face not rescaled to the 12pt class base")
	}
	if _, err := e.Run(`\noindent{\large L}N`); err != nil {
		t.Fatal(err)
	}
	sizes := map[rune]int{}
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok {
			for _, m := range b.list {
				if c, ok := m.(charNode); ok {
					if _, seen := sizes[c.ch]; !seen {
						sizes[c.ch] = c.size
					}
				}
			}
		}
	}
	if sizes['N'] != 12 { // body text at the 12pt base
		t.Errorf("body glyph in 12pt class = %dpt, want 12pt", sizes['N'])
	}
	if sizes['L'] != 14 { // \large = 1200‰ off the 12pt base: round(12*1.2)=14
		t.Errorf("\\large in 12pt class = %dpt, want 14pt", sizes['L'])
	}
}

// scaleClassFontsToBase is a no-op for a 10pt (default) class and for an unset
// factor, is safe with no base size, keeps a non-scalable base face as-is, and
// clamps a degenerate factor to at least 1pt.
func TestClassScaledFontsEdges(t *testing.T) {
	e := New()
	e.SetFont(scaleMock{px: 10})
	e.scaleClassFontsToBase(1000) // 100%: no change
	e.scaleClassFontsToBase(0)    // unset: no change
	if e.baseFontPx != 10 || e.baseFont.sizePt() != 10 {
		t.Errorf("10pt/unset class changed the base: px=%d size=%d", e.baseFontPx, e.baseFont.sizePt())
	}

	// A non-scalable base (spMock has no atSizePx): the design size is recorded but
	// the mock face is left unchanged.
	e2 := New()
	e2.SetFont(spMock{})
	e2.scaleClassFontsToBase(1100)
	if e2.baseFontPx != 11 {
		t.Errorf("non-scalable base: baseFontPx = %d, want 11", e2.baseFontPx)
	}
	if e2.curFont.sizePt() != 10 {
		t.Errorf("non-scalable base: glyph size = %d, want 10 (unchanged)", e2.curFont.sizePt())
	}

	// No base font set yet: safe no-op, no panic — for scaleClassFontsToBase and
	// for \gotexsize (which needs a base font to scale).
	e3 := New()
	e3.LoadLaTeX()
	e3.scaleClassFontsToBase(1100)
	if e3.baseFontPx != 0 {
		t.Errorf("no base: baseFontPx = %d, want 0", e3.baseFontPx)
	}
	if _, err := e3.Run(`\gotexsize1000\relax`); err != nil {
		t.Fatal(err)
	}

	// \gotexsize on a non-scalable base leaves the size unchanged (no atSizePx).
	e5 := New()
	e5.LoadLaTeX()
	e5.SetFont(spMock{})
	if _, err := e5.Run(`\gotexsize1440\relax X`); err != nil {
		t.Fatal(err)
	}
	if e5.curFont.sizePt() != 10 {
		t.Errorf("non-scalable \\gotexsize changed size to %d, want 10", e5.curFont.sizePt())
	}

	// A degenerate \gotexsize factor clamps the resulting face to at least 1pt.
	e6 := New()
	e6.LoadLaTeX()
	e6.SetFont(scaleMock{px: 10})
	if _, err := e6.Run(`\gotexsize1\relax`); err != nil {
		t.Fatal(err)
	}
	if e6.curFont.sizePt() != 1 {
		t.Errorf("degenerate \\gotexsize1 = %dpt, want 1pt (clamped)", e6.curFont.sizePt())
	}

	// Degenerate factor clamps the size to at least 1pt.
	e4 := New()
	e4.SetFont(scaleMock{px: 10})
	e4.scaleClassFontsToBase(1)
	if e4.baseFontPx != 1 || e4.baseFont.sizePt() != 1 {
		t.Errorf("degenerate factor: px=%d size=%d, want 1/1", e4.baseFontPx, e4.baseFont.sizePt())
	}
}
