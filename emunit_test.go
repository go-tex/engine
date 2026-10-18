package engine

import "testing"

// em/ex are font-relative: 1em is the font's quad — the size, 10pt here — and 1ex
// its X-HEIGHT, which TeX takes from \fontdimen5 (tex.web §455, §10675).
//
// ⛔ This expected 5.0pt for 1ex, half the size, which is what the engine used to
// return and what the reference does NOT: at 10pt in Latin Modern the reference
// gives 4.31pt, 16% less, and every ex-stated class dimension inherited the error
// (go-tex/engine#620). spMock reports 7pt of height for EVERY glyph, so 7.0pt
// here is its 'x' and the number is what makes this test discriminate: half the
// size would still be 5.0pt.
func TestEmExUnits(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // sizePt() = 10, every glyph 7pt tall
	e.Run(`\dimen0=1em \dimen1=2.5em \dimen2=1ex \message{\the\dimen0|\the\dimen1|\the\dimen2}`)
	if got := trimNL(e.out.String()); got != "10.0pt|25.0pt|7.0pt" {
		t.Errorf("em/ex got %q want 10.0pt|25.0pt|7.0pt", got)
	}
}

// \quad from the Plain prelude inserts a 1em kern of glue on the line.
func TestQuadMacro(t *testing.T) {
	e := New()
	e.LoadPlain()
	e.SetFont(spMock{})
	e.Run(`\setbox0=\hbox{a\quad b}`)
	// a(5) + quad(1em=10) + b(5) = 20pt
	if b := e.box[0]; b == nil || b.width != 20*unity {
		t.Fatalf("\\quad box width %v want 20pt", e.box[0])
	}
}
