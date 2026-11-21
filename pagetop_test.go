// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// A mark, a whatsit or a page-style node at the top of a page does not end TeX's
// discard: tex.web:19495-19502 drops glue, kern and penalty until a box or a rule
// arrives (:19509-19516). The kept node moves to just before the box.
func TestPageStartDiscardsGlueBehindANonBox(t *testing.T) {
	box := &boxNode{height: ptToSP(7)}
	list := []node{
		pageStyleNode{style: "plain"},
		penaltyNode{penalty: -300},
		glueNode{spec: glueSpec{width: ptToSP(8.4)}},
		markNode{},
		kernNode{width: ptToSP(2)},
		box,
		glueNode{spec: glueSpec{width: ptToSP(4)}},
	}
	start := skipDiscardable(list, 0)
	if start != 3 {
		t.Fatalf("the page starts at %d, want 3 (three discardables dropped)", start)
	}
	if _, ok := list[start].(pageStyleNode); !ok {
		t.Errorf("list[%d] is %T, want the pageStyleNode kept first", start, list[start])
	}
	if _, ok := list[start+1].(markNode); !ok {
		t.Errorf("list[%d] is %T, want the mark kept second", start+1, list[start+1])
	}
	if list[start+2] != node(box) {
		t.Errorf("list[%d] is %T, want the box", start+2, list[start+2])
	}
	for _, n := range list[start:] {
		if g, ok := n.(glueNode); ok && g.spec.width == ptToSP(8.4) {
			t.Error("the 8.4pt glue before the first box is still on the page")
		}
	}
	if again := skipDiscardable(list, 0); again != start {
		t.Errorf("a second call starts at %d, want %d", again, start)
	}
}

// Every amsart paper opens its list with \thispagestyle{plain} (amsart.cls:1894),
// and the \section beforeskip behind it (.7\linespacing = 8.40pt) was charged to the
// page. tectonic, from the top of the page:
//
//	                                 tectonic   before   after
//	amstop.tex     first heading     126.01     134.36   125.99
//	amstopctl.tex  first text line   126.01
//
// The heading (6.80pt) and the text line are both shorter than \topskip, so both
// baselines sit \topskip below the top of the text block, and tectonic puts them
// on the same baseline. That equality is what is asserted, in our own SVG frame:
// the SVG page is not the PDF's 612x792, so its y cannot be compared with
// tectonic's directly.
func TestParityAmsartFirstHeadingIsAtTheTopOfThePage(t *testing.T) {
	first := func(name string) float64 {
		src, err := os.ReadFile(filepath.Join("testdata", "parity", name+".tex"))
		if err != nil {
			t.Fatal(err)
		}
		pages, err := CompileToSVGPages(src, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) == 0 {
			t.Fatalf("%s: no page", name)
		}
		return firstBaselineY(t, pages[0])
	}
	heading, text := first("amstop"), first("amstopctl")
	if d := heading - text; d < -0.05 || d > 0.05 {
		t.Errorf("the first heading's baseline is %.2f bp below a first text line's, tectonic's is 0", d)
	}
}

// A page whose first item is a \markright (go-tex/engine#834, its first comment):
// the paragraph's \parskip behind the mark stayed on the page. tectonic, from the
// top of the page, on testdata/parity/markfirst.tex:
//
//	           tectonic   before   after
//	page 1     134.76     139.89   134.74   (opens with the mark)
//	page 2     134.76     134.74   134.74   (opens with the text)
func TestParityAMarkFirstOnAPageDoesNotLowerIt(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "markfirst.tex"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("%d pages, want 2", len(pages))
	}
	// The running head is the topmost text; the body line is the second baseline.
	p1, p2 := baselineYs(t, pages[0]), baselineYs(t, pages[1])
	if len(p1) < 2 || len(p2) < 2 {
		t.Fatalf("baselines %v and %v, want a head and a body line on each page", p1, p2)
	}
	if d := p1[1] - p2[1]; d < -0.05 || d > 0.05 {
		t.Errorf("page 1's first line is %.2f bp below page 2's, tectonic's are level", d)
	}
}
