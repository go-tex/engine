package engine

import (
	"strings"
	"testing"
)

// The text layer must carry what a reader SEES, not how the formula was built.
// Each want here is tectonic's own text layer for the same source, with spaces
// removed on both sides: pdftotext inserts a space wherever two glyphs sit far
// apart, which is a property of the glyph positions and not of the string.
//
//	source      A $\{a\}$ B $x^{2}$ C $y_{i}$ D $\displaystyle z$ E $\text{\normalfont q}$ F
//	reference   A{a}Bx2CyiDzEqF
//	before      A\{a\}Bx^{2}Cy_}i{D\sdzip…
//
// go-tex/engine#372.
func TestStripMathLayout(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"la source est GARDÉE: voir TestFormulaSourceIsSearchable", `x^{2}`, `x^{2}`},
		{"un indice aussi", `y_{i}`, `y_{i}`},
		{"accolade ÉCHAPPÉE, qui est dessinée", `\{a\}`, "{a}"},
		{"bascule de style sans argument", `\displaystyle z`, "z"},
		{"bascule de fonte", `\text{\normalfont q}`, `\text{q}`},
		{"un symbole garde son nom", `\Omega`, `\Omega`},
		{"un symbole suivi d'une accolade", `\frac{a}{b}`, `\frac{a}{b}`},
		{"rien à faire", `abc`, "abc"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripMathLayout(tc.in); got != tc.want {
				t.Errorf("stripMathLayout(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// End to end, through the layer a reader copies from. pageChars reads the GLYPHS,
// which is the wrong instrument here — the defect is invisible on the page — so
// this goes through the SVG's text layer, as textlayer_test.go does.
func TestMathTextLayerMatchesWhatIsDrawn(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}\begin{document}`+
		`A $\{a\}$ B $x^{2}$ C $y_{i}$ D $\displaystyle z$ F\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := textLayerContent(e.RenderPage(1))
	for _, leaked := range []string{`\displaystyle`} {
		if strings.Contains(got, leaked) {
			t.Errorf("%q is in the text layer but is never drawn: %q", leaked, got)
		}
	}
	for _, kept := range []string{`x^{2}`, `y_{i}`, "z"} {
		if !strings.Contains(got, kept) {
			t.Errorf("%q is drawn but missing from the text layer: %q", kept, got)
		}
	}
}
