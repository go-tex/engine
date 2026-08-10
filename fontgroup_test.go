package engine

import "testing"

// sizedMock is a distinguishable font whose char width equals its size (pt), so a
// font switch is observable in the packed box.
type sizedMock struct{ s int }

func (m sizedMock) charDimsSP(rune) (int, int, int) { return m.s * unity, m.s * unity, 0 }
func (sizedMock) spaceSP() glueSpec                 { return glueSpec{width: 3 * unity} }
func (sizedMock) glyphPathAt(rune) string           { return "" }
func (sizedMock) kernSP(_, _ rune) int              { return 0 }
func (m sizedMock) sizePt() int                     { return m.s }

// A font selected inside { … } is restored at the group's end.
func TestFontGroupScoped(t *testing.T) {
	e := New()
	e.SetFont(sizedMock{10})
	e.eq["big"] = &meaning{kind: mFont, font: sizedMock{20}}
	// X is set inside the group with \big (width 20pt); Y after the group (10pt).
	if _, err := e.Run(`\setbox0=\hbox{{\big X}Y}`); err != nil {
		t.Fatal(err)
	}
	var widths []int
	for _, n := range e.box[0].list {
		if c, ok := n.(charNode); ok {
			widths = append(widths, c.width)
		}
	}
	if len(widths) != 2 || widths[0] != 20*unity || widths[1] != 10*unity {
		t.Fatalf("widths=%v want [20pt 10pt] (font restored after group)", widths)
	}
	// after the whole run, the current font is back to the 10pt one
	if e.curFont.sizePt() != 10 {
		t.Errorf("curFont size=%d want 10 after group", e.curFont.sizePt())
	}
}
