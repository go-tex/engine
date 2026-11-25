// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A \c@page write inside a paragraph waits for the paragraph's end; outside one it goes
// straight onto the main vertical list.
func TestPageCounterWriteInsideAParagraphWaits(t *testing.T) {
	e := New()
	e.inPar = true
	e.contributePageCounterSet(pageCounterSetNode{value: 9})
	if len(e.mvl) != 0 || len(e.parSets) != 1 {
		t.Fatalf("in a paragraph: mvl=%d parSets=%d, want 0 and 1", len(e.mvl), len(e.parSets))
	}
	e = New()
	e.contributePageCounterSet(pageCounterSetNode{value: 9})
	if len(e.mvl) != 1 || len(e.parSets) != 0 {
		t.Fatalf("outside a paragraph: mvl=%d parSets=%d, want 1 and 0", len(e.mvl), len(e.parSets))
	}
}

// A revtex body shares the page with frontmatter on the list, but a \c@page write alone
// is no frontmatter: no span region may be made from it.
func TestRevtexBodyIgnoresAPageCounterWriteAlone(t *testing.T) {
	e := New()
	e.hsize = 100 * unity
	e.vsize = 200 * unity
	e.mvl = []node{pageCounterSetNode{value: 3}}
	e.startRevtexBody()
	for _, r := range e.colRegions {
		if r.span != nil {
			t.Fatalf("a page counter write alone became a frontmatter span")
		}
	}
	e = New()
	e.hsize = 100 * unity
	e.vsize = 200 * unity
	e.mvl = []node{pageCounterSetNode{value: 3}, shipPicNode{}}
	e.startRevtexBody()
	if len(e.colRegions) != 1 || e.colRegions[0].span == nil {
		t.Fatalf("frontmatter on the list made no span region: %d regions", len(e.colRegions))
	}
}

// \prevdepth is reset by the clearpage strut only when the page holds no material; a
// \c@page write is no material, anything else is.
func TestClearpageStrutIgnoresOnlyPageCounterWrites(t *testing.T) {
	e := New()
	e.mvl = []node{pageCounterSetNode{value: 3}}
	e.prevDepth = 7 * unity
	e.doClearpageStrut()
	if e.prevDepth != 0 {
		t.Fatalf("a write alone left prevDepth %v, want 0", e.prevDepth)
	}
	e.mvl = []node{pageCounterSetNode{value: 3}, shipPicNode{}}
	e.prevDepth = 7 * unity
	e.doClearpageStrut()
	if e.prevDepth != 7*unity {
		t.Fatalf("material on the list changed prevDepth to %v", e.prevDepth)
	}
}
