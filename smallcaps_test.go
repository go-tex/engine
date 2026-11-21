// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// The embedded small-caps face is parsed when the default family is built, and a
// face that does not parse refuses the whole build rather than leaving \scshape a
// no-op. The embedded bytes cannot fail to parse, so the test truncates them: the
// refusal is buildEngine's, the only caller of the "sc" binding. A host that names
// its own roman binds no small caps and is not affected.
func TestATruncatedSmallCapsFaceRefusesTheDefaultFamily(t *testing.T) {
	saved := lmRomanCaps
	t.Cleanup(func() { lmRomanCaps = saved })
	lmRomanCaps = saved[:len(saved)/2]
	if _, err := buildEngine(Options{}, true); err == nil {
		t.Error("buildEngine with a truncated small-caps face: want an error, got nil")
	}
	lmRomanCaps = saved
	if _, err := buildEngine(Options{}, true); err != nil {
		t.Errorf("buildEngine with the embedded small-caps face: %v", err)
	}
}

// \scshape and \textsc set Latin Modern small capitals, against tectonic on
// testdata/parity/smallcaps.tex. \scshape was an empty macro and \textsc was
// undefined in article (amsart's stand-in was {#1}), so both came out in the
// roman's lower case: every amsart \section heading, every \section*, every
// \textsc. Where each line's last glyph ENDS, in bp from the margin (tectonic's
// x1 − 133.77):
//
//	                                tectonic   before
//	Alfa {\scshape …} bravo.        172.85     narrower, lower case
//	Charlie \textsc{…} delta.       164.35     narrower, lower case
//
// The caps face is the one tulmr.fd names for every size ("<-> ec-lmcsc10"), and
// the PDF's BaseFont reads LMRomanCaps10-Regular on both engines.
func TestParitySmallCapsMatchTheReference(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "smallcaps.tex"))
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
	want := []float64{172.85, 164.35}
	// Two body lines and the folio.
	if len(ends) != len(want)+1 {
		t.Fatalf("%d lines, want %d plus the folio: %v", len(ends), len(want), ends)
	}
	for i, w := range want {
		if g := ends[i]; g < w-0.05 || g > w+0.05 {
			t.Errorf("line %d ends %.2fbp from the margin, tectonic's ends %.2f", i+1, g, w)
		}
	}
}

// lastGlyphEnd is where the last glyph of an hbox ends, in pt from the box's left
// edge, descending into boxes that hold glyphs.
func lastGlyphEnd(b *boxNode) (float64, bool) {
	x, end, found := 0.0, 0.0, false
	for _, c := range b.list {
		switch v := c.(type) {
		case charNode:
			x += spToPt(v.width)
			end, found = x, true
		case *boxNode:
			if v.kind == hbox {
				if dx, ok := lastGlyphEnd(v); ok {
					end, found = x+dx, true
				}
			}
			x += spToPt(v.width)
		case kernNode:
			x += spToPt(v.width)
		case glueNode:
			x += spToPt(b.setWidth(v.spec))
		}
	}
	return end, found
}
