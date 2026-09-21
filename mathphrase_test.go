// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// The phrase standing for a formula in the text layer is its SOURCE, so that a
// reader searching for an equation types what the author typed. The maths alphabet
// commands are where that reasoning inverts: they only choose a FACE, the page shows
// the argument, and \mathrm{abc} is the one string that finds nothing.
func TestMathAlphabetWrappersAreUnwrapped(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`\mathrm{abc}`, `abc`},
		{`\mathbb{R}`, `R`},
		{`\mathcal{X}`, `X`},
		{`\operatorname{argmax}`, `argmax`},
		{`\text{if } x>0`, `if  x>0`},
		{`\mathbf{\mathrm{x}}`, `x`},    // imbriqué
		{`x_{\mathrm{enc}}`, `x_{enc}`}, // l'indice garde SES accolades
		{`\mathrm {spaced}`, `spaced`},  // espace avant le groupe
		// un symbole GARDE son nom: son caractère vit dans la table du paquet math,
		// le dupliquer ici mettrait une vérité à deux endroits.
		{`\alpha+\Omega`, `\alpha+\Omega`},
		{`\frac{a}{b}`, `\frac{a}{b}`},
		{`\mathrm`, `\mathrm`},                   // sans groupe: inchangé
		{`\mathrm{unclosed`, `\mathrm{unclosed`}, // accolade non fermée: inchangé
		{`plain text`, `plain text`},
	} {
		if got := unwrapMathAlphabets(c.in); got != c.want {
			t.Errorf("unwrapMathAlphabets(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// End to end: the phrase the text layer receives, through the same collapseSpace
// the SVG and PDF drivers both call.
func TestCollapseSpaceUnwrapsAlphabets(t *testing.T) {
	if got, want := collapseSpace(`\mathrm{abc}`), `abc`; got != want {
		t.Errorf("collapseSpace = %q, want %q", got, want)
	}
}
