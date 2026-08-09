package engine

import "testing"

type mockFont struct{}

func (mockFont) CharDims(r rune) (float64, float64, float64) { return 1, 0.7, 0.2 }
func (mockFont) Space() (float64, float64, float64)          { return 0.5, 0.25, 0.15 }
func TestTypesetPipeline(t *testing.T) {
	// \hi expands to "Hello"; then a space and "world" → 11 glyphs + 1 interword glue.
	p, ok := New().Typeset(`\def\hi{Hello}\hi world`, mockFont{}, 100, 10, 10, 1.2)
	if !ok {
		t.Fatal("typeset failed")
	}
	// gather the glyphs actually set
	var glyphs []rune
	var hbox HBox
	for _, n := range p.Box.List {
		if hb, ok := n.(HBox); ok {
			hbox = hb
		}
	}
	for _, n := range hbox.List {
		if c, ok := n.(Char); ok {
			glyphs = append(glyphs, c.R)
		}
	}
	got := string(glyphs)
	if got != "Helloworld" {
		t.Errorf("glyphs=%q want Helloworld (macro expanded, chars boxed)", got)
	}
	// one line (fits in width 100), width packed to 100, height from metrics
	if hbox.W != 100 || hbox.H != 0.7 || hbox.D != 0.2 {
		t.Errorf("hbox W=%v H=%v D=%v", hbox.W, hbox.H, hbox.D)
	}
}
func TestTypesetWraps(t *testing.T) {
	// narrow width forces multiple lines from a run of words
	p, ok := New().Typeset(`aa aa aa aa aa aa`, mockFont{}, 5, 20, 10, 1.2)
	if !ok {
		t.Fatal("typeset")
	}
	nlines := 0
	for _, n := range p.Box.List {
		if _, ok := n.(HBox); ok {
			nlines++
		}
	}
	if nlines < 2 {
		t.Errorf("expected wrapping into >=2 lines, got %d", nlines)
	}
}
