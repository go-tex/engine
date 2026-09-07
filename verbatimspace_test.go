// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A verbatim space is a KERN, so that it cannot stretch. The text layer decides on
// a space geometrically — a gap wider than a quarter of an em — and a verbatim
// space falls just under that: measured on "AA BB", the advance across the space is
// 9.2pt against 7.1 and 6.2 for the letters, leaving about 2.1pt of clear gap
// against a 2.5pt threshold. So the layer wrote "AABB", and a reader copying a code
// listing out of the output got "//Findthedatapointsinthesamebucket".
//
// The kern now carries the fact that it stands for a typed space. That is exact
// information and outranks the geometric guess, which is the same route ordinary
// inter-word glue already takes (boxrender.go).
func TestVerbatimSpaceIsMarkedOnTheKern(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	nodes := e.verbNodes("a b", spMock{}, 1)
	if len(nodes) != 3 {
		t.Fatalf("verbNodes gave %d nodes, want 3", len(nodes))
	}
	k, ok := nodes[1].(kernNode)
	if !ok {
		t.Fatalf("the space is a %T, want kernNode", nodes[1])
	}
	if !k.space {
		t.Error("the kern does not say it stands for a typed space")
	}
	if k.width <= 0 {
		t.Errorf("the space kern has width %d, want the font's space", k.width)
	}
}

// An ordinary \kern is NOT a space and must stay silent, or every italic
// correction would put a space in the text layer.
func TestPlainKernIsNotASpace(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`A\kern3pt B\par`); err != nil {
		t.Fatal(err)
	}
	for _, n := range e.mvl {
		if k, ok := n.(kernNode); ok && k.space {
			t.Error("a plain \\kern was marked as a typed space")
		}
	}
}
