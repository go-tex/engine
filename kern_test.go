package engine

import (
	"os"
	"testing"
)

// With a real font, a known kerning pair (AV, which kerns negative) makes an
// \hbox narrower than the sum of the two glyphs' bare advances.
func TestFontKerningTightensPair(t *testing.T) {
	// Arial carries legacy 'kern'-table pairs (Georgia's kerning is GPOS-only).
	font := "/System/Library/Fonts/Supplemental/Arial.ttf"
	if _, err := os.Stat(font); err != nil {
		t.Skip("system font not present")
	}
	e := New()
	if _, err := e.Run(`\font\rm=` + font + ` at 40pt \rm\setbox0=\hbox{AV}\setbox1=\hbox{A}\setbox2=\hbox{V}`); err != nil {
		t.Fatal(err)
	}
	av, a, v := e.box[0].width, e.box[1].width, e.box[2].width
	if av >= a+v {
		t.Fatalf("expected AV kerned narrower than A+V: AV=%d A+V=%d", av, a+v)
	}
	// the difference is exactly the kern node inserted between A and V
	if kern := av - (a + v); kern >= 0 {
		t.Errorf("kern should be negative, got %d sp", kern)
	}
}
