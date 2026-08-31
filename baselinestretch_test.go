// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// applyBaselineStretch honors a native \renewcommand{\baselinestretch}{f} at
// \begin{document}: the active \baselineskip becomes the single-spaced reference
// times f. A value of 1 (or the default) changes nothing.
func TestBaselineStretchNativeApplied(t *testing.T) {
	cases := []struct {
		def   string
		wantF float64
	}{
		{"", 1.0}, // default \baselinestretch is 1
		{`\renewcommand{\baselinestretch}{1.5}`, 1.5},     // stretch
		{`\renewcommand{\baselinestretch}{0.9}`, 0.9},     // compression
		{`\renewcommand{\baselinestretch}{1}`, 1.0},       // explicit single: no-op
		{`\renewcommand{\baselinestretch}{garbage}`, 1.0}, // malformed: unchanged
	}
	for _, c := range cases {
		e := New()
		e.LoadLaTeX()
		e.SetFont(spMock{})
		if _, err := e.Run(c.def + `\begin{document}Text.\end{document}`); err != nil {
			t.Fatalf("%q: %v", c.def, err)
		}
		want := int(float64(e.baseBaselineskip)*c.wantF + 0.5)
		if e.baselineskip != want {
			t.Errorf("def %q: baselineskip=%d, want %d (%.2f × %d)",
				c.def, e.baselineskip, want, c.wantF, e.baseBaselineskip)
		}
	}
}

// setspace's size-adjusted spacing is NOT clobbered by the \begin{document} hook:
// \onehalfspacing sets the baseline skip itself (explicitStretch), so
// applyBaselineStretch leaves it alone even though \baselinestretch may read 1.
func TestBaselineStretchDoesNotClobberSetspace(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\onehalfspacing\begin{document}Text.\end{document}`); err != nil {
		t.Fatal(err)
	}
	if !e.explicitStretch {
		t.Fatal("\\onehalfspacing did not mark an explicit stretch")
	}
	want := int(float64(e.baseBaselineskip)*onehalfStretch(e.ptsizeCode()) + 0.5)
	if e.baselineskip != want {
		t.Errorf("onehalfspacing baselineskip=%d, want the size-adjusted %d (not reset by the hook)",
			e.baselineskip, want)
	}
}
