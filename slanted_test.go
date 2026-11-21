// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ⛔ THE LINE ENDS BELOW CANNOT TELL SLANTED FROM UPRIGHT. Latin Modern's slanted
// shape is the roman sheared, with the roman's advances: a mutant that bound no
// slanted face at all, so \slshape fell to the upright roman, ended all three
// lines of slanted.tex at the same 0.01bp as the fix. What separates them is the
// FACE, so that is asserted here, at the file tulmr.fd's m/sl names per size.
func TestFontnameSaysTheSlantedShapeIsLatinModernsOwn(t *testing.T) {
	for _, tc := range []struct{ cmd, want string }{
		{`\footnotesize`, "lmromanslant8-regular"},
		{`\small`, "lmromanslant9-regular"},
		{`\normalsize`, "lmromanslant10-regular"},
		{`\Large`, "lmromanslant12-regular"},
		{`\LARGE`, "lmromanslant17-regular"},
	} {
		got := say(t, `{`+tc.cmd+`\slshape\message{[F=\fontname\font]}}`)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s\\slshape: \\fontname said %q, want it to name %s", tc.cmd, got, tc.want)
		}
	}
}

// \slshape sets Latin Modern's own slanted shape, at the design size tulmr.fd's
// m/sl names, and not the italic. Against tectonic on testdata/parity/slanted.tex,
// where each line's last glyph ENDS, in bp from the margin (x1 − 133.77):
//
//	                          tectonic face            tectonic   before (italic)
//	\textsl{…}       10pt     LMRomanSlant10           179.69     177.31
//	\footnotesize    8pt      LMRomanSlant8            152.61     152.25
//	\Large           14.4pt   LMRomanSlant12           148.29     146.91
func TestParitySlantedMatchesTheReference(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "slanted.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var ends []float64
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && directChar(b) {
				if x, ok := lastGlyphEnd(b); ok {
					ends = append(ends, x*72/72.27)
				}
				continue
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	want := []float64{179.69, 152.61, 148.29}
	// Three body lines and the folio.
	if len(ends) != len(want)+1 {
		t.Fatalf("%d lines, want %d plus the folio: %v", len(ends), len(want), ends)
	}
	for i, w := range want {
		if g := ends[i]; g < w-0.05 || g > w+0.05 {
			t.Errorf("line %d ends %.2fbp from the margin, tectonic's ends %.2f", i+1, g, w)
		}
	}
}
