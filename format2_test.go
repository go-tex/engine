package engine

import "testing"

func TestMiniLatexDoc(t *testing.T) {
	e := New()
	if err := e.LoadFormat(MiniLaTeX); err != nil {
		t.Fatalf("format: %v", err)
	}
	// A document using \newcommand (defined in TeX) and \LaTeX, typeset by the engine.
	p, ok := e.Typeset(`\newcommand\greeting{Hello there}\greeting from \LaTeX`, mockFont{}, 200, 20, 10, 1.2)
	if !ok {
		t.Fatal("typeset")
	}
	var glyphs []rune
	for _, n := range p.Box.List {
		if hb, ok := n.(HBox); ok {
			for _, m := range hb.List {
				if c, ok := m.(Char); ok {
					glyphs = append(glyphs, c.R)
				}
			}
		}
	}
	got := string(glyphs)
	if got != "HellotherefromLaTeX" {
		t.Errorf("glyphs=%q want HellotherefromLaTeX", got)
	}
}
