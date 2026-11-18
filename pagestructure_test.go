// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// pageTextList is the content of a page's TEXT BLOCK.
//
// ⛔ WHY A HELPER AND NOT p.list. A page with a running foot ships
// [text block][glue \footskip][foot] — \@outputpage's \vbox around \@outputbox
// (latex.ltx) — so p.list's boxes are the block and the folio, not the lines. A
// walker that takes p.list measures a container and a page number: that is how
// \rightskip=100pt came out as "narrowed by -199.56pt", the width of the whole
// block rather than of a line.
//
// It reads the mark the assembler sets rather than guessing from the page's
// shape, because the shape differs by page style and a sniff would quietly pick
// the wrong box on the style it was not written for.
func pageTextList(p *boxNode) []node {
	for _, n := range p.list {
		if b, ok := n.(*boxNode); ok && b.textBlock {
			return b.list
		}
	}
	return p.list
}

// ⛔⛔ AND THE RUNNING-HEAD PAGE IS THE SAME PAGE, which is the whole point: it
// carries nearly every page of a real document and it had its own copy of the
// defect. assembleFancyPage packed the head, the body, a \vfil and the foot into
// ONE box, so the body's glue stretched by nothing — #721's first comment
// measured 616.87 against tectonic's 672.75 on \pagestyle{headings}, a page
// holding 55.9pt less than it should.
//
// \@outputpage keeps the three apart (latex.ltx): the head, \vskip\headsep, then
// \box\@outputbox — the block ALONE packed to \@colht — then \baselineskip\footskip
// and the foot. So the invariant is the same one as for a plain page, and this is
// the row that was missing when the whole suite stayed green through the change.
//
// ⚠ MEASURED on \pagestyle{headings}, article at 10pt, \flushbottom, last body
// baseline of page 1: tectonic 672.75, before 655.87, after 667.92. The residual
// 4.83pt is NOT this defect — our running head sits 4.85pt high and our first
// text baseline 3.94pt low, so the band between them is 8.79pt too tall and the
// block is that much shorter. Filed separately; this test asserts the structure,
// not that residual.
func TestARunningHeadPageAlsoWrapsItsTextBlock(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// ⛔⛔ PAGE TWO, AND THE REASON IS A FIXTURE THAT FOOLED THIS TEST ONCE.
	// \chapter sets \thispagestyle{plain}, so page ONE of a book chapter is a
	// PLAIN page and goes down the other branch entirely. Written against page
	// one, this test passed against main — where assembleFancyPage marks no text
	// block at all — because it was measuring the plain path a second time. The
	// red-before check is what caught it; the assertions had not changed.
	src := `\documentclass{book}\begin{document}\chapter{Alfa}` +
		strings.Repeat(`Opening paragraph. `, 400) + `\par\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if !e.hasLatexHead() {
		t.Skip("book.cls defined no \\@oddhead here; there is no running-head page to test")
	}
	pages := e.Pages()
	if len(pages) < 2 {
		t.Fatalf("%d page(s): the witness needs a page that is NOT the chapter "+
			"opening, because that one is \\thispagestyle{plain}", len(pages))
	}
	var block *boxNode
	for _, n := range pages[1].list {
		if b, ok := n.(*boxNode); ok && b.textBlock {
			block = b
		}
	}
	if block == nil {
		t.Fatal("a running-head page must wrap a marked text block: \\@outputpage " +
			"packs \\@outputbox alone and places the head and foot around it")
	}
	if e.vsize > 0 && block.height != e.vsize {
		t.Errorf("text block = %d, want \\vsize = %d — the head band is not part of "+
			"the text (renderVMargin lifts the page by it)", block.height, e.vsize)
	}
	if pages[1].height <= block.height {
		t.Errorf("page = %d, not taller than its text block %d: the head and foot "+
			"are inside the block again (#721)", pages[1].height, block.height)
	}
}

// ⛔ THE TEXT BLOCK IS \vsize AND THE PAGE AROUND IT IS TALLER.
//
// This is the invariant the folio's move created, asserted on its own rather
// than only as a side condition of the tests that walk pages. \@makecol packs
// the block to \@colht (ltoutput.dtx:2128-2146) and \@outputpage puts the foot
// UNDER it, so a page that carries a folio cannot be as short as its text.
//
// Both rows matter: "empty" has no foot and its page IS the block, which is the
// control that stops this passing for an engine that simply made every page
// taller.
func TestAPageWithAFootIsTallerThanItsTextBlock(t *testing.T) {
	const body = `\hbox{\vrule width5pt height10pt}`
	for _, c := range []struct {
		style    string
		wantFoot bool
	}{
		{"empty", false},
		{"plain", true},
	} {
		t.Run(c.style, func(t *testing.T) {
			e := New()
			e.vsize = 300 * unity
			if _, err := e.Run(`\pagestyle{` + c.style + `}` + body); err != nil {
				t.Fatal(err)
			}
			pages := e.Pages()
			if len(pages) != 1 {
				t.Fatalf("%d page(s), want 1", len(pages))
			}
			var block *boxNode
			for _, n := range pages[0].list {
				if b, ok := n.(*boxNode); ok && b.textBlock {
					block = b
				}
			}
			if !c.wantFoot {
				if block != nil {
					t.Errorf("\\pagestyle{empty} has no running foot, so there is no "+
						"\\@outputbox to wrap: the page should BE the text block, "+
						"found a marked block of %d", block.height)
				}
				if pages[0].height != e.vsize {
					t.Errorf("page = %d, want \\vsize = %d", pages[0].height, e.vsize)
				}
				return
			}
			if block == nil {
				t.Fatal("\\pagestyle{plain} ships a folio, so the page must wrap a " +
					"marked text block; none found")
			}
			if block.height != e.vsize {
				t.Errorf("text block = %d, want \\vsize = %d (tex.web §19813 packs "+
					"box255 exactly)", block.height, e.vsize)
			}
			if pages[0].height <= block.height {
				t.Errorf("page = %d, not taller than its text block %d — the foot is "+
					"inside the block again (#721)", pages[0].height, block.height)
			}
		})
	}
}
