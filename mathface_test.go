// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/go-opentype/opentype"
)

// The default MATHS face is Latin Modern Math, the face whose metrics Computer Modern
// is, which tectonic sets formulas in (its PDF embeds CMR10, CMMI10, CMEX10, ...). The
// engine set them in STIX Two Math. testdata/parity/mathmetrics.tex prints the box
// dimensions of six formulas; tectonic's are recorded next to it.
//
// The comparison is a TOLERANCE, not an equality, and the residue is real: the
// fraction's height and depth (ref 6.95/3.45pt, here 6.17/1.94pt) and the large
// operators' widths differ because go-tex/math builds those from its own constants
// rather than TeX's \fontdimen rules. 3pt of width and 1.6pt of height or depth are the
// bounds; STIX's \sum is 5.4pt narrower than the reference's and fails the first.
func TestParityMathFaceMetrics(t *testing.T) {
	_, got, want := parityPair(t, "mathmetrics")
	re := regexp.MustCompile(`([a-z]+)(-?[0-9.]+)pt/(-?[0-9.]+)pt/(-?[0-9.]+)pt`)
	parse := func(s string) map[string][3]float64 {
		m := map[string][3]float64{}
		for _, x := range re.FindAllStringSubmatch(s, -1) {
			var v [3]float64
			for i := range v {
				v[i], _ = strconv.ParseFloat(x[2+i], 64)
			}
			m[x[1]] = v
		}
		return m
	}
	g, w := parse(got), parse(want)
	if len(w) != 6 || len(g) != 6 {
		t.Fatalf("read %d engine and %d reference formulas, want 6 each\n got: %q\nwant: %q", len(g), len(w), got, want)
	}
	for name, r := range w {
		v := g[name]
		for i, tol := range [3]float64{3, 1.6, 1.6} {
			if d := v[i] - r[i]; d > tol || d < -tol {
				t.Errorf("%s: %s is %.2fpt, tectonic's %.2fpt (tolerance %.1f)", name, [3]string{"width", "height", "depth"}[i], v[i], r[i], tol)
			}
		}
	}
}

// Latin Modern Math states its own axis in the MATH table, and it is a quarter of the
// em, exactly what \fontdimen22 is in Computer Modern. So the quarter that parbox.go
// hardcodes is the font's own answer for the default maths face, and reading it from
// the font (the held axis-from-the-font branch) would return the same number.
func TestLatinModernMathAxisIsAQuarter(t *testing.T) {
	f, err := NewOpenTypeFont(lmMath, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.f.NewFace(1000).MathConstant(opentype.AxisHeight); got != 250 {
		t.Errorf("axis = %d per 1000 em, want 250", got)
	}
}
