package engine

import "testing"

// ligFont is spMock with every glyph present, so ligature substitutions fire.
type ligFont struct{ spMock }

func (ligFont) glyphPathAt(rune) string { return "M0 0" }

// ligature() folds the standard TeX text pairs and quote/dash forms when the font
// has the combined glyph, and declines otherwise.
func TestLigatureLogic(t *testing.T) {
	e := New()
	e.SetFont(ligFont{})
	cases := []struct {
		prev, cur, want rune
		ok              bool
	}{
		{'f', 'f', ligFF, true},
		{'f', 'i', ligFI, true},
		{'f', 'l', ligFL, true},
		{ligFF, 'i', ligFFI, true},
		{ligFF, 'l', ligFFL, true},
		{'-', '-', enDash, true},
		{enDash, '-', emDash, true},
		{lsQuote, '`', ldQuote, true},
		{rsQuote, '\'', rdQuote, true},
		{'a', 'b', 0, false}, // no ligature
		{'f', 'x', 0, false}, // f only ligates with f/i/l
	}
	for _, c := range cases {
		g, ok := e.ligature(c.prev, c.cur)
		if ok != c.ok || (ok && g != c.want) {
			t.Errorf("ligature(%q,%q) = (%q,%v), want (%q,%v)", c.prev, c.cur, g, ok, c.want, c.ok)
		}
	}
	if e.singleForm('`') != lsQuote || e.singleForm('\'') != rsQuote || e.singleForm('a') != 'a' {
		t.Error("singleForm quote mapping wrong")
	}
}

// In a paragraph, "office" collapses its f-f-i run into a single ﬃ glyph.
func TestLigatureIntegration(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(ligFont{})
	if _, err := e.Run(`\noindent office`); err != nil {
		t.Fatal(err)
	}
	if got, want := mvlText(e.mvl), "oﬃce"; got != want {
		t.Errorf("ligated %q, want %q", got, want)
	}
}

// Dashes and quotes fold: "a--b---c" and “x” produce en/em dashes and curly
// double quotes.
func TestDashQuoteLigatures(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(ligFont{})
	if _, err := e.Run("\\noindent a--b---c ``x''"); err != nil {
		t.Fatal(err)
	}
	got := mvlText(e.mvl)
	want := "a–b—c“x”" // spaces are glue, dropped
	if got != want {
		t.Errorf("dash/quote ligatures = %q, want %q", got, want)
	}
}

// Safety: a font without the ligature glyph keeps the separate characters rather
// than dropping a glyph. spMock's glyphPathAt is empty, so "ff" stays "ff".
func TestLigatureFallbackNoGlyph(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\noindent ff`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "ff" {
		t.Errorf("without the ligature glyph, expected %q, got %q", "ff", got)
	}
}
