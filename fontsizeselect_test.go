// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// \fontsize records a size and a leading and \selectfont applies them
// (latex.ltx \set@fontsize). Both kernel layers defined \fontsize as a no-op, so
// every line of testdata/parity/fontsize.tex came out at 10pt on 12pt leading.
// Against tectonic:
//
//	line                                      tectonic                 before
//	{\itshape\fontsize{12}{14}\selectfont …}  LMRoman12-Italic 11.96   LMRoman10-Italic 9.96
//	{\fontsize{14pt}{20pt}\selectfont …}      LMRoman12 13.95, 19.93   LMRoman10 9.96, 11.96
//	{\fontsize{12}{14} …}  (no \selectfont)   LMRoman10 9.96           LMRoman10 9.96
//
// After, each line's face, size and baseline is tectonic's to 0.03bp.
func TestParityFontsizeIsAppliedBySelectfont(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "fontsize.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var sizes []float64
	var faces []*byte
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && directChar(b) {
				if ch, ok := firstGlyph(b); ok {
					sizes = append(sizes, ch.face.sizePt())
					faces = append(faces, faceKey(ch.face))
				}
				continue
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	key := func(b []byte) *byte {
		if fam := opticalOf(b); len(fam) > 0 {
			return &fam[0].data[0]
		}
		return &b[0]
	}
	// Five lines (the 14pt paragraph takes two) and the folio.
	want := []struct {
		pt   float64
		key  *byte
		what string
	}{
		{12, key(lmRomanItalic), `\itshape\fontsize{12}{14}\selectfont: 12pt italic`},
		{14, key(lmRomanRegular), `\fontsize{14pt}{20pt}\selectfont: 14pt, the unit read`},
		{14, key(lmRomanRegular), `the same paragraph's second line`},
		{10, key(lmRomanRegular), `\fontsize without \selectfont changes nothing`},
		{10, key(lmRomanRegular), `the body after the groups`},
	}
	if len(sizes) != len(want)+1 {
		t.Fatalf("%d lines, want %d plus the folio", len(sizes), len(want))
	}
	for i, w := range want {
		if math.Abs(sizes[i]-w.pt) > 0.01 {
			t.Errorf("line %d at %.2fpt: %s", i+1, sizes[i], w.what)
		}
		if faces[i] != w.key {
			t.Errorf("line %d is not in the Latin Modern face tectonic uses: %s", i+1, w.what)
		}
	}

	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ys := baselineYs(t, pages[0])
	if len(ys) < 3 {
		t.Fatalf("%d baselines", len(ys))
	}
	// The 14pt paragraph's two lines are \fontsize's 20pt apart (the SVG is in TeX
	// points; tectonic's PDF reads 19.93bp, before this 11.96bp).
	if gap := ys[2] - ys[1]; math.Abs(gap-20) > 0.05 {
		t.Errorf("the 14pt paragraph's leading is %.2fpt, tectonic 20pt", gap)
	}
}

// A size given as "12pt" is the size "12" is (\@defaultunits ... pt).
func TestParsePtArgReadsAnExplicitPoint(t *testing.T) {
	for _, s := range []string{"12", "12pt", " 12pt ", "12.0"} {
		if v, ok := parsePtArg(s); !ok || v != 12 {
			t.Errorf("parsePtArg(%q) = %v, %v; want 12", s, v, ok)
		}
	}
	if _, ok := parsePtArg("12mm"); ok {
		t.Errorf("12mm read as points")
	}
}
