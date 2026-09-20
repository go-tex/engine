// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// spaceInLine returns the width of the interword glue the engine put on the page.
func firstSpaceWidth(t *testing.T, src string) int {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(scaleMock{px: 10})
	if _, err := e.Run(`\noindent ` + src); err != nil {
		t.Fatal(err)
	}
	var find func([]node) (int, bool)
	find = func(ns []node) (int, bool) {
		for _, n := range ns {
			switch c := n.(type) {
			case glueNode:
				if c.spec.width > 0 {
					return c.spec.width, true
				}
			case *boxNode:
				if w, ok := find(c.list); ok {
					return w, true
				}
			}
		}
		return 0, false
	}
	w, ok := find(e.mvl)
	if !ok {
		t.Fatal("no interword glue on the page")
	}
	return w
}

// TeX reads the interword glue from the font's parameters — 2 nominal, 3 stretch,
// 4 shrink — so \fontdimen2\font=<dimen> IS the space from that point on, not
// advice. We consumed the assignment (which stopped a "==-=" leak) and then dropped
// the value, so a document that set its own interword space silently got ours.
//
// Measured against tectonic: \fontdimen2\font=8pt moves the reference's space to
// exactly 8pt and left ours at 2pt; =1pt gives 1pt there and 2pt here.
// IEEEtran.cls sets all three per font-size switch (l.1045-1050), and 12 of the 200
// corpus papers use it, 6 of them shipping the class.
//
// The READ side is unchanged and already covered by TestFontdimenReadConsumesNothing
// (leakedvalue_test.go): it consumes nothing and leaks nothing.
func TestFontdimenAssignmentSetsTheInterwordSpace(t *testing.T) {
	base := firstSpaceWidth(t, `A A`)
	for _, c := range []struct{ set, want int }{{8, 8}, {1, 1}, {3, 3}} {
		got := firstSpaceWidth(t, `\fontdimen2\font=`+itoa(c.set)+`pt A A`)
		if got != c.want*unity {
			t.Errorf("\\fontdimen2\\font=%dpt gave a space of %d sp (%.2fpt), want %dpt",
				c.set, got, float64(got)/float64(unity), c.want)
		}
	}
	if base == 8*unity {
		t.Error("the base space is already 8pt, so the test proves nothing")
	}
}
