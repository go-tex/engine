package engine

import "testing"

// em/ex are font-relative: with a 10pt font, 1em = 10pt and 1ex = 5pt.
func TestEmExUnits(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // sizePt() = 10
	e.Run(`\dimen0=1em \dimen1=2.5em \dimen2=1ex \message{\the\dimen0|\the\dimen1|\the\dimen2}`)
	if got := trimNL(e.out.String()); got != "10.0pt|25.0pt|5.0pt" {
		t.Errorf("em/ex got %q want 10.0pt|25.0pt|5.0pt", got)
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
