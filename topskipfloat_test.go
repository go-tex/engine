// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \topskip BELONGS UNDER THE TOP-FLOAT BLOCK, NOT ABOVE IT.
//
// \topskip's width is reduced by the height of the page's first box
// (tex.web:19509-19516). On a page that opens with a top float, that box is the
// FLOAT — tall enough to swallow the whole register — so the first line of text
// under the block was placed by whatever glue the main vertical list happened to
// carry, and the page's \topskip vanished.
//
// LaTeX cannot make this mistake, because \@cflt attaches the float block AFTER the
// page box exists and splices the page back in with \unvbox (ltoutput.dtx:3597-3604):
//
//	\vbox{<floats> \vskip-\floatsep \topfigrule \vskip\textfloatsep \unvbox\@outputbox}
//
// \unvbox keeps the \topskip glue TeX already put at the head of \@outputbox, so the
// first text baseline under a top float is set by \topskip exactly as on any other
// page.
//
// ⚠ MEASURED against tectonic (article at 11pt, a [t] float holding a 120x60pt rule,
// first text baseline below the block), with a no-float control that has to move WITH
// the reference or this reading is void:
//
//	\topskip          0pt    11pt (defaut)   30pt
//	tectonic       243.05       246.20     265.13
//	before         248.72       248.72     248.72   <- moves by nothing
//	after          243.03       246.18     265.11   <- -0.02, the standing metric offset
//	control (no float), both engines move by -3.15 and +18.93
//
// The three rows of this table are run against EVERY assertion below, controls
// included: a witness that fires on its own control is measuring something else.
func TestTopskipGoesUnderTheTopFloatBlockNotAboveIt(t *testing.T) {
	const floatHeight = 60 * unity
	for _, c := range []struct {
		name    string
		assign  string
		topskip int
	}{
		// ⛔ THE DEFAULT ROW IS NOT DECORATION: a reader that returned the register
		// unconditionally would pass the two assigned rows and still hand every
		// ordinary document the wrong glue.
		{"defaut", "", 10 * unity},
		// ⛔ ZERO IS A VALUE. \topskip=0pt must give NO glue, not the default back.
		{"zero", `\topskip=0pt`, 0},
		// Larger than the first line, so the glue is positive and the arithmetic
		// visible.
		{"trente", `\topskip=30pt`, 30 * unity},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("GOTEX_FLOATS", "1")
			e := New()
			if err := e.LoadLaTeX(); err != nil {
				t.Fatal(err)
			}
			e.SetFont(spMock{})
			src := `\documentclass{article}` + c.assign + `\begin{document}` +
				strings.Repeat(`Opening paragraph. `, 6) + `\par` +
				`\begin{figure}[t]\rule{1pt}{60pt}\end{figure}` +
				strings.Repeat(`Middle paragraph. `, 6) + `\par`
			if _, err := e.Run(src); err != nil {
				t.Fatal(err)
			}
			pages := e.Pages()
			if len(pages) == 0 {
				t.Fatal("aucune page")
			}
			list := pages[0].list

			// The float block opens the page. \topskip is NOT above it: this is the
			// half of the rule that says where the glue must not be, and it is what
			// went wrong — the glue was there, reduced to zero by a 60pt box.
			fi := firstBoxAt(list)
			if fi < 0 {
				t.Fatal("la page ne porte aucune boîte")
			}
			if w := glueWidthBefore(list, fi); w != 0 {
				t.Errorf("%s de glue au-dessus du bloc de flottants, want 0: "+
					"\\@cflt n'en met aucune (ltoutput.dtx:3597-3604)", spToStr(w))
			}
			if h := list[fi].(*boxNode).height; h != floatHeight {
				t.Fatalf("la première boîte fait %s de haut, want %s — ce n'est pas le "+
					"flottant et le reste du test porterait sur autre chose",
					spToStr(h), spToStr(floatHeight))
			}

			// Under it: \textfloatsep, then \topskip reduced by the height of the
			// first TEXT box (not of the float).
			ti := firstBoxAt(list[fi+1:])
			if ti < 0 {
				t.Fatal("aucune boîte de texte sous le bloc de flottants")
			}
			ti += fi + 1
			h := list[ti].(*boxNode).height
			want := e.textFloatSep()
			if c.topskip > h {
				want += c.topskip - h
			}
			if got := glueWidthBefore(list[fi+1:ti], len(list[fi+1:ti])); got != want {
				t.Errorf("%s entre le bloc et la première ligne, want %s "+
					"(\\textfloatsep %s + \\topskip %s réduit de la hauteur %s de CETTE "+
					"ligne, tex.web:19513) — l'écart est le \\baselineskip que "+
					"l'interligne y mettait",
					spToStr(got), spToStr(want), spToStr(e.textFloatSep()),
					spToStr(c.topskip), spToStr(h))
			}
		})
	}
}

// firstBoxAt is the index of the first box in a list, or -1.
func firstBoxAt(list []node) int {
	for i, n := range list {
		if _, ok := n.(*boxNode); ok {
			return i
		}
	}
	return -1
}

// glueWidthBefore totals the natural width of the glue in list[:upTo].
func glueWidthBefore(list []node, upTo int) int {
	if upTo > len(list) {
		upTo = len(list)
	}
	w := 0
	for _, n := range list[:upTo] {
		if g, ok := n.(glueNode); ok {
			w += g.spec.width
		}
	}
	return w
}
