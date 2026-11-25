// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// With the default family, \textsf{\textbf{W}} is LMSans10-Bold, \textsf{\textit{W}}
// LMSans10-Oblique and \textsf{\textbf{\textit{W}}} LMSans10-BoldOblique, as tulmss.fd
// declares them (bx/n, m/sl with m/it a substitution of it, bx/sl) and as tectonic
// 0.17.0 embeds them (pdffonts on \textsf{\textbf{W} \textit{W} \textbf{\textit{W}}}:
// LMSans10-Bold, LMSans10-Oblique, LMSans10-BoldOblique). Before, all three were the
// regular LMSans10 (no sans bold or oblique was bound, so the roman faces were set).
func TestLMSansBoldAndObliqueShapes(t *testing.T) {
	size := float64(Options{}.size())
	for _, tc := range []struct {
		name  string
		src   string
		bytes []byte
	}{
		{"regular", `\textsf{W}`, lmSansRegular},
		{"bold", `\textsf{\textbf{W}}`, lmSansBold},
		{"bold in the other order", `\textbf{\textsf{W}}`, lmSansBold},
		// Switches, not \textit: that one adds an italic correction after the W.
		{"italic is the oblique", `{\sffamily\itshape W}`, lmSansOblique},
		{"slanted is the oblique", `{\sffamily\slshape W}`, lmSansOblique},
		{"bold italic is the bold oblique", `{\sffamily\bfseries\itshape W}`, lmSansBoldOblique},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, err := NewOpenTypeFont(tc.bytes, size)
			if err != nil {
				t.Fatal(err)
			}
			want, _, _ := f.charDimsSP('W')
			e, err := buildEngine(Options{}, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Run(`\setbox0=\hbox{` + tc.src + `}`); err != nil {
				t.Fatal(err)
			}
			if got := e.box[0].width; got != want {
				t.Errorf("%s: width %d sp, want %d sp (the shape's own advance)", tc.src, got, want)
			}
		})
	}
	// A slant keeps the advance, so the width cannot tell an oblique from its upright:
	// the outline can.
	w := map[string]string{}
	for name, b := range map[string][]byte{"regular": lmSansRegular, "bold": lmSansBold, "oblique": lmSansOblique, "boldoblique": lmSansBoldOblique} {
		f, err := NewOpenTypeFont(b, size)
		if err != nil {
			t.Fatal(err)
		}
		p := f.glyphPathAt('W')
		if prev, dup := w[p]; dup {
			t.Errorf("%s and %s have the same W outline", name, prev)
		}
		w[p] = name
	}
}

// m/sl is optical: <-8.5> 8  8.5-9.5 9  9.5-11 10  11-15.5 12  15.5- 17 (tulmss.fd).
func TestLMSansObliqueOpticalSizes(t *testing.T) {
	if opticalOf(lmSansOblique) == nil {
		t.Error("the oblique has no optical table")
	}
	for _, c := range []struct {
		pt   float64
		name string
	}{
		{8, "lmsans8-oblique"}, {9, "lmsans9-oblique"}, {10, "lmsans10-oblique"},
		{11, "lmsans12-oblique"}, {15.49, "lmsans12-oblique"}, {15.5, "lmsans17-oblique"},
	} {
		if _, name := lmSansObliqueSizes.pick(c.pt); name != c.name {
			t.Errorf("%g pt: %s, want %s", c.pt, name, c.name)
		}
	}
}
