// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ THIS FILE IS NAMED a0_ ON PURPOSE. go test runs the tests of a package in
// the order of their file names, and these tests hold guards (an empty list, no
// current font) that nearly every document passes. With the guard removed, the
// first test to reach it panics and ends the whole run, so a test that sorts
// later never gets to report its value: mutsweep then read the red as a PANIC,
// luck and not an assertion. Sorting first, each test here recovers (attempt)
// and fails by naming the value, so the assertion is what catches the mutant.

// \/ after anything that is not a character adds nothing: a kern, glue, a box and
// an empty list have no glyph whose correction it could be.
func TestItalicCorrectionAfterANonCharacterAddsNothing(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	list := e.currentList()
	*list = append(*list, kernNode{width: 5})
	attempt(t, `\/ after a kern`, e.appendItalicCorrection)
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
	attempt(t, "a cached correction", func() {
		if got := o.italCorrSP('f'); got != 12345 {
			t.Errorf("a cached correction was recomputed: %d, want 12345", got)
		}
	})
	o.icCache = nil
	attempt(t, "an uncached correction", func() {
		first := o.italCorrSP('f')
		if v, ok := o.icCache['f']; !ok || v != first {
			t.Errorf("a computed correction was not kept: cache %v", o.icCache)
		}
	})
}

// \fontcharXX of a character code that is not positive, or with no current font,
// is 0 rather than the metric of some other glyph.
func TestFontCharMetricOfNoCharacterIsZero(t *testing.T) {
	attempt(t, `\fontcharXX of code 0 and -3`, func() {
		got := say(t, `\message{[\the\fontcharwd\font 0][\the\fontcharht\font -3][\the\fontcharic\font 0]}`)
		if want := "[0.0pt][0.0pt][0.0pt]"; !contains(got, want) {
			t.Errorf("got %q, want it to contain %s", got, want)
		}
	})
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.curFont = nil
	e.push(stringToToks(`\font 65 `))
	attempt(t, `\fontcharwd with no font`, func() {
		if v := e.fontCharDim("fontcharwd"); v != 0 {
			t.Errorf("no current font: %d, want 0", v)
		}
	})
}

// \lastpenalty reads as an integer wherever one is scanned.
func TestLastPenaltyIsScannedAsAnInteger(t *testing.T) {
	attempt(t, `\lastpenalty as an integer`, func() {
		got := say(t, `x\penalty77 \count1=\lastpenalty \message{[\the\count1]}`)
		if !contains(got, "[77]") {
			t.Errorf("got %q, want [77]", got)
		}
	})
}

// attempt runs f and reports a panic as a failure naming what was asked: a guard
// that is gone dereferences or indexes past the list, and that is the failure.
func attempt(t *testing.T, what string, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s panicked: %v", what, r)
		}
	}()
	f()
}

// On an empty list there is no last node: \/ adds nothing, \lastpenalty is 0.
func TestTheEndOfAnEmptyListHasNoCorrectionAndNoPenalty(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	*e.currentList() = nil
	attempt(t, `\/ on an empty list`, func() {
		e.appendItalicCorrection()
		if n := len(*e.currentList()); n != 0 {
			t.Errorf("\\/ on an empty list added %d node(s)", n)
		}
	})
	attempt(t, `\lastpenalty on an empty list`, func() {
		if p := e.lastPenalty(); p != 0 {
			t.Errorf("\\lastpenalty = %d, want 0", p)
		}
	})
}

// The same, for a metric asked of no font, or of a code that is not positive.
func TestFontCharMetricWithNoFontOrNoCodeDoesNotPanic(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatal(err)
	}
	e.curFont = nil
	for _, which := range fontCharDimNames {
		attempt(t, which+" with no current font", func() {
			e.push(stringToToks(`\font 65 `))
			if v := e.fontCharDim(which); v != 0 {
				t.Errorf("%s with no current font = %d, want 0", which, v)
			}
		})
	}
}
