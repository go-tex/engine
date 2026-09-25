package engine

import "testing"

// revtex4-2 declares the five standard paper options and EXECUTES none of them
// (revtex4-2.cls:5928-5951), so its sheet is letter unless the document asks for
// another. This emulation loads no class file, so nothing else published it.
//
// Measured on corpus paper 2308.02700, \documentclass[a4paper,twocolumn]{revtex4-2}:
//
//	reference   595.28  x 841.89
//	before      612     x 792
//	after       595.276 x 841.89
//
// Twelve pages either way — the OUTPUT was wrong, not the pagination (#419).
// Shaped after TestAcmartFormatsCarryTheirPaper, which asks the same question of
// the same field through the whole engine rather than of the table alone.
func TestRevtexPaperOption(t *testing.T) {
	for _, c := range []struct {
		opt          string
		wantW, wantH float64 // TeX points
	}{
		{"a4paper", 210 / 25.4 * 72.27, 297 / 25.4 * 72.27},
		{"a5paper", 148 / 25.4 * 72.27, 210 / 25.4 * 72.27},
		{"b5paper", 176 / 25.4 * 72.27, 250 / 25.4 * 72.27},
		{"legalpaper", 8.5 * 72.27, 14 * 72.27},
		// Letter is the engine's default and revtex's, so it must stay letter.
		{"letterpaper", 8.5 * 72.27, 11 * 72.27},
		{"twocolumn", 8.5 * 72.27, 11 * 72.27},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass[` + c.opt + `]{revtex4-2}\begin{document}X\end{document}`); err != nil {
			t.Fatalf("%s: %v", c.opt, err)
		}
		w, h, ok := e.paperSizePt()
		if !ok {
			t.Errorf("%s: pas de taille de feuille", c.opt)
			continue
		}
		if w < c.wantW-0.01 || w > c.wantW+0.01 || h < c.wantH-0.01 || h > c.wantH+0.01 {
			t.Errorf("%s: feuille %.2f x %.2f pt, want %.2f x %.2f", c.opt, w, h, c.wantW, c.wantH)
		}
	}
}
