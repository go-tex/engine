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
		{`\mathbf{\mathrm{x}}`, `x`},    // nested
		{`x_{\mathrm{enc}}`, `x_{enc}`}, // the subscript keeps ITS braces
		{`\mathrm {spaced}`, `spaced`},  // a space before the group
		// a SYMBOL keeps its name: its character lives in the maths package's table,
		// and duplicating it here would put one truth in two places.
		{`\alpha+\Omega`, `\alpha+\Omega`},
		{`\frac{a}{b}`, `\frac{a}{b}`},
		{`\mathrm`, `\mathrm`},                   // no group: unchanged
		{`\mathrm{unclosed`, `\mathrm{unclosed`}, // unclosed brace: unchanged
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
