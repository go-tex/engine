// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \/ after anything that is not a character adds nothing: a kern, glue, a box and
// an empty list have no glyph whose correction it could be.
func TestItalicCorrectionAfterANonCharacterAddsNothing(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	list := e.currentList()
	*list = append(*list, kernNode{width: 5})
	e.appendItalicCorrection()
	if len(*list) != 1 {
		t.Errorf("\\/ after a kern added %d node(s)", len(*list)-1)
	}
}

// A glyph's correction is read once and kept: a cached value is the answer, and a
// miss fills the cache.
func TestItalicCorrectionIsCached(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	o, ok := e.curFont.(*OpenTypeFont)
	if !ok {
		t.Fatalf("the current font is %T, not an OpenType font", e.curFont)
	}
	o.icCache = map[rune]int{'f': 12345}
	if got := o.italCorrSP('f'); got != 12345 {
		t.Errorf("a cached correction was recomputed: %d, want 12345", got)
	}
	o.icCache = nil
	first := o.italCorrSP('f')
	if v, ok := o.icCache['f']; !ok || v != first {
		t.Errorf("a computed correction was not kept: cache %v", o.icCache)
	}
}

// \fontcharXX of a character code that is not positive, or with no current font,
// is 0 rather than the metric of some other glyph.
func TestFontCharMetricOfNoCharacterIsZero(t *testing.T) {
	got := say(t, `\message{[\the\fontcharwd\font 0][\the\fontcharht\font -3][\the\fontcharic\font 0]}`)
	if want := "[0.0pt][0.0pt][0.0pt]"; !contains(got, want) {
		t.Errorf("got %q, want it to contain %s", got, want)
	}
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.curFont = nil
	e.push(stringToToks(`\font 65 `))
	if v := e.fontCharDim("fontcharwd"); v != 0 {
		t.Errorf("no current font: %d, want 0", v)
	}
}

// \lastpenalty reads as an integer wherever one is scanned.
func TestLastPenaltyIsScannedAsAnInteger(t *testing.T) {
	got := say(t, `x\penalty77 \count1=\lastpenalty \message{[\the\count1]}`)
	if !contains(got, "[77]") {
		t.Errorf("got %q, want [77]", got)
	}
}
