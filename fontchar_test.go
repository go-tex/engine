// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ ε-TeX's glyph-metric primitives were undefined, and skipping one releases what follows:
// lmcs.cls:817 sizes an ORCID logo from a capital X's height,
//
//	\setlength{\@lmcscurXheight}{\fontcharht\font`X}%
//
// and it runs from \author, so 2603.18955 lost that \setlength's argument and leaked a group
// there. Measured on 999 papers: \fontcharht in 14, \XeTeXLinkBox in 11; the fix removes one
// leaked group on each of 2603.18955 and 2603.13142, and moves no page or glyph path.
//
// ⭐ The values are EXACT rather than a design-size fraction: fontFace.charDimsSP is the same
// measurement that sets every character on the page.

func fontCharDimOf(t *testing.T, src string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\newlength{\zzl}\newdimen\zzd`+
		`\begin{document}`+src+`\typeout{ZZV=\the\zzd/\the\zzl}x\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	msg := e.Diagnostics().Messages
	i := strings.Index(msg, "ZZV=")
	if i < 0 {
		t.Fatalf("no value reported: %q", msg)
	}
	rest := msg[i+4:]
	if j := strings.IndexAny(rest, "\n"); j >= 0 {
		rest = rest[:j]
	}
	return rest
}

// ⛔ BOTH readings must work, and the \setlength one is what the corpus writes. A first
// version of this change pushed the value as TEXT: \zzd=\fontcharht… and
// \setlength{\zzl}{\fontcharht…} then both read 0.0pt while the digits were typeset, which a
// test asserting only "not undefined" would have passed. Registering the primitives as
// INTERNAL DIMENSIONS is what makes the scanner consume them.
func TestFontCharHtIsReadableAsADimensionBothWays(t *testing.T) {
	got := fontCharDimOf(t, "\\zzd=\\fontcharht\\font`X \\setlength{\\zzl}{\\fontcharht\\font`X}")
	a, b, ok := strings.Cut(got, "/")
	if !ok {
		t.Fatalf("unexpected report %q", got)
	}
	if a != b {
		t.Errorf("assignment read %s and \\setlength read %s: they must agree", a, b)
	}
	if a == "0.0pt" {
		t.Error("0.0pt: the primitive is not being read as an internal dimension")
	}
}

// A capital is taller than a lowercase x, and a descender has depth where a capital has
// none. Asserting the ORDER rather than absolute points is what keeps the test true under a
// font substitution — ours measures 6.57pt for X where tectonic's Computer Modern gives
// 6.83pt, and the difference is the face, not the arithmetic.
func TestGlyphMetricsOrderThemselvesLikeRealGlyphs(t *testing.T) {
	capHt := fontCharDimOf(t, "\\zzd=\\fontcharht\\font`X ")
	lowHt := fontCharDimOf(t, "\\zzd=\\fontcharht\\font`x ")
	if ptLess(t, capHt) <= 0 || ptLess(t, lowHt) <= 0 {
		t.Fatalf("a height came out zero: X=%s x=%s", capHt, lowHt)
	}
	if ptLess(t, capHt) <= ptLess(t, lowHt) {
		t.Errorf("X measured %s and x measured %s: a capital must be taller", capHt, lowHt)
	}
	capDp := fontCharDimOf(t, "\\zzd=\\fontchardp\\font`X ")
	gDp := fontCharDimOf(t, "\\zzd=\\fontchardp\\font`g ")
	if ptLess(t, gDp) <= ptLess(t, capDp) {
		t.Errorf("g has depth %s and X has %s: a descender must go below the baseline",
			gDp, capDp)
	}
}

func ptLess(t *testing.T, s string) float64 {
	t.Helper()
	s = strings.TrimSuffix(strings.TrimSpace(strings.SplitN(s, "/", 2)[0]), "pt")
	var v float64
	for i, c := range s {
		if c == '.' {
			frac := 0.1
			for _, d := range s[i+1:] {
				if d < '0' || d > '9' {
					break
				}
				v += float64(d-'0') * frac
				frac /= 10
			}
			break
		}
		if c < '0' || c > '9' {
			break
		}
		v = v*10 + float64(c-'0')
	}
	return v
}

// \XeTeXLinkBox{<material>} must keep its material and CONSUME its braces. Skipped, the
// material survived by accident — released into the stream — and the braces were left to the
// surrounding group, which is the leak.
func TestXeTeXLinkBoxKeepsItsMaterial(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`[\XeTeXLinkBox{Z}]\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("%d group(s) left open, want 0", d.OpenGroups)
	}
	// [ Z ] and the page number: four glyph paths.
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "<path"); n != 4 {
		t.Errorf("%d glyph path(s), want 4 ([, Z, ] and the page number)", n)
	}
}

func TestIsFontCharDimNamesExactlyTheFour(t *testing.T) {
	for _, n := range fontCharDimNames {
		if !isFontCharDim(n) {
			t.Errorf("%s is in the list but isFontCharDim says no", n)
		}
	}
	for _, n := range []string{"wd", "ht", "dp", "fontdimen", "fontname"} {
		if isFontCharDim(n) {
			t.Errorf("%s is not a glyph-metric primitive", n)
		}
	}
}

// ⛔ And the INTEGER coercion, which is a SECOND path: TeX reads an internal dimension used
// where a number is wanted as its value in scaled points, so \number\fontcharht\font`X has
// to answer. That path is isInternalDimen's, not scanDimenValue's.
//
// The assertion is on the VALUE, not on a comparison. A first version wrote
// \ifnum\fontcharht\font`X>0 and passed with the entry removed too — because without it the
// primitive runs and pushes "6.57pt" as text, \ifnum reads the 6, and 6 > 0 as well. What
// the missing entry really does is leak: \number then reports "0\fontcharht \font `X".
func TestFontCharHtCoercesToScaledPoints(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		"\\typeout{ZZNUM=[\\number\\fontcharht\\font`X]}x"+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	msg := e.Diagnostics().Messages
	i := strings.Index(msg, "ZZNUM=[")
	if i < 0 {
		t.Fatalf("no value reported: %q", msg)
	}
	got := msg[i+7:]
	if j := strings.IndexByte(got, ']'); j >= 0 {
		got = got[:j]
	}
	// A capital X at 10pt is some 430000sp — tectonic's Computer Modern says 447611, and
	// ours differs by the face, not by the arithmetic. Anything with a backslash in it is
	// the primitive leaking instead of being read.
	if strings.Contains(got, "\\") {
		t.Fatalf("\\number read %q: the primitive leaked instead of coercing", got)
	}
	if v := ptLess(t, got); v < 100000 {
		t.Errorf("\\number read %q (%.0f): want a scaled-point value of order 400000", got, v)
	}
}
