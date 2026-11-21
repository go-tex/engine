// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// The foot of a "plain" page is the page style's own \@oddfoot, not a folio of
// the engine's. amsart's \ps@plain sets it in \scriptsize (amsart.cls:302-303),
// and its first page is \thispagestyle{plain} (amsart.cls:1894). Against tectonic
// on testdata/parity/amsfolio.tex:
//
//	                    tectonic          before
//	folio face          LMRoman7 @6.97    LMRoman10 @9.96
//	folio x0, bp        304.02            303.52
//
// so the folio starts 177.35bp from the margin (126.67) and is set at 7pt.
func TestParityAmsartFolioIsTheStylesFoot(t *testing.T) {
	got := bodyLineIndents(t, "amsfolio")
	if len(got) != 2 {
		t.Fatalf("%d lines, want the body line and the folio: %v", len(got), got)
	}
	if x, err := strconv.ParseFloat(got[1], 64); err != nil || x < 177.30 || x > 177.40 {
		t.Errorf("the folio starts %s bp from the margin, tectonic's starts 177.35", got[1])
	}
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "amsfolio.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var last *charNode
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case charNode:
				c := v
				last = &c
			case *boxNode:
				walk(v.list)
			}
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	if last == nil {
		t.Fatal("no glyph on the page")
	}
	if last.ch != '1' || last.size < 6.9 || last.size > 7.1 {
		t.Errorf("the folio is %q at %.2fpt, tectonic sets \"1\" at 7pt (\\scriptsize)", last.ch, last.size)
	}
}

// A style whose foot field is empty draws no foot at all: \ps@headings defines an
// empty \@oddfoot (latex.go), so a \thispagestyle{headings} override under a plain
// document style typesets an empty list, and pageFooter answers nil rather than
// packing it into an empty hbox that would still cost a \footskip. The control is
// the document's own plain style, whose foot is the folio.
func TestAStyleWithAnEmptyFootFieldDrawsNoFoot(t *testing.T) {
	e, err := compile([]byte("\\documentclass{article}\n\\begin{document}\nx\n\\end{document}\n"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if e.pageStyle != "plain" {
		t.Fatalf("the document style is %q, the witness needs plain", e.pageStyle)
	}
	if foot := e.pageFooter(1, "plain"); foot == nil {
		t.Fatal("the plain style's foot is nil, the control has no folio")
	}
	if foot := e.pageFooter(1, "headings"); foot != nil {
		t.Errorf("a style with an empty \\@oddfoot drew a foot of %d nodes", len(foot.list))
	}
	body := vpackSP(nil, packNatural, 0)
	if got := e.withFootAtFootskip(body, 1, "headings"); got != body {
		t.Error("an empty foot was put under the text block")
	}
}

// The override's \ps@ runs at output time, side effects included. amsart's
// \maketitle sets \global\topskip42\p@ for the title page (amsart.cls:623), and
// it is the output routine running \thispagestyle{plain}'s \ps@plain — whose
// \ps@empty ends \global\topskip\normaltopskip (amsart.cls:301) — that puts it
// back to 10pt. The engine never ran the style there, so every later page kept
// the 42pt \topskip: on testdata/parity/amspage2.tex page 2's first line sat
// 55.79bp below its head where tectonic sets it 23.91bp below (126.01 − 102.10),
// and each page held 3 to 4 lines fewer. 2406.10437 ran 38 pages where it now
// runs 36.
func TestParityTheTitlePagesTopskipEndsWithTheTitlePage(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "amspage2.tex"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) < 2 {
		t.Fatalf("%d page(s), the witness needs two", len(pages))
	}
	re := regexp.MustCompile(`<tspan[^>]*\sy="([0-9.]+)"`)
	var ys []float64
	for _, m := range re.FindAllStringSubmatch(pages[1], -1) {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			ys = append(ys, v)
		}
	}
	sort.Float64s(ys)
	head, body := -1.0, -1.0
	for _, y := range ys {
		switch {
		case head < 0:
			head = y
		case y > head+1:
			body = y
		}
		if body >= 0 {
			break
		}
	}
	if body < 0 {
		t.Fatalf("page 2 has no line below its head: %v", ys)
	}
	// Read off the SVG text layer, which places a head and a body line a few
	// hundredths apart from the PDF (24.00 here where the PDF of the same build
	// reads 23.91, tectonic's value). The tolerance covers that and nothing near
	// the 32bp the title page's \topskip cost (56.00 here before).
	if gap := body - head; gap < 23.76 || gap > 24.06 {
		t.Errorf("page 2's first line is %.2fbp below its head, tectonic's 23.91", gap)
	}
}
