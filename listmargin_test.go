// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every \list-based environment of article.cls — quote, a two-paragraph quotation,
// verse, a wrapping description — and the text around them, line by line against
// tectonic: where each line STARTS, in bp from the margin (gotex-linediff x0 minus
// 133.77). go-tex/engine#587.
//
// Before: \list discarded its settings argument, so quote/quotation/verse were not
// indented at all (0.00 where the reference has 24.90), verse's two lines were set
// as one, and description had no bold label and no hanging continuation. Each row
// below was wrong in at least one of those ways; the golden is the reference's.
//
// What each row exercises:
//
//	Bravo ×3        \list's \parshape (the margins) on every line of a paragraph
//	Charlie, Delta  \itemindent/\listparindent, and {\@@par} keeping the shape for
//	                a SECOND paragraph (group-scoped \parshape)
//	Echo, Foxtrot   \@centercr: each verse line its own paragraph
//	Golf, Hotel     \makelabel = \descriptionlabel (\hspace\labelsep, unbraced),
//	                \itemindent -\leftmargin: label at the margin, text hanging
//	Alfa, India     the measure around the lists is untouched (\linewidth's own
//	                register: \advance\linewidth used to shrink \hsize itself)
func TestParityListMarginsMatchTheReference(t *testing.T) {
	ref, err := os.ReadFile(filepath.Join("testdata", "parity", "listmargin.tectonic.indents.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Fields(string(ref))
	got := bodyLineIndents(t, "listmargin")
	// The page number is the last line; every other line is asserted.
	if len(got) != len(want)+1 {
		t.Fatalf("read %d lines, want %d plus the folio: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("line %d starts %s bp in, tectonic's starts %s", i+1, got[i], w)
		}
	}
}

// thebibliography is a \list too (article.cls:568-575), so it takes the same path:
// \bibitem is an \item (ltbibl.dtx:154-177), numbered by \usecounter{enumiv}.
// Against tectonic on testdata/parity/biblist.tex, three entries, two of them wrapping:
//
//	                    tectonic      main
//	label x0            4.98          -15.50   (hung 20bp into the margin)
//	continuation x0     20.48         0.00
//	entry-to-entry      19.93bp       11.96bp  (\itemsep + \parsep missing)
//	first entry breaks  before "the"  before "of" (the space after \bibitem{a} typeset)
//
// Start of each line asserted here (in bp from the margin), and every baseline gap
// in pt below. ⚠ Our \@trivlist empties \@itemlabel (LaTeX's \trivlist does, its
// \@trivlist does not), and \list sets it again afterwards — without that line
// every entry lost its [N], which is the label this test reads first.
func TestParityBibliographyMatchesTheReference(t *testing.T) {
	got := bodyLineIndents(t, "biblist")
	want := []string{"14.94", "0.00", "4.98", "20.48", "4.98", "4.98", "20.48"}
	if len(got) != len(want)+1 {
		t.Fatalf("read %d lines, want %d plus the folio: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("line %d starts %s bp in, tectonic's starts %s", i+1, got[i], w)
		}
	}
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "biblist.tex"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	ys := baselineYs(t, pages[0])
	// tectonic, bp → pt: Alfa→References 32.97, then 21.92, 12, 20, 20, 12.
	gaps := []float64{33.09, 22.0, 12, 20, 20, 12}
	if len(ys) != len(gaps)+2 {
		t.Fatalf("%d baselines, want %d: %v", len(ys), len(gaps)+2, ys)
	}
	for i, w := range gaps {
		if g := ys[i+1] - ys[i]; g < w-0.05 || g > w+0.05 {
			t.Errorf("gap %d is %.2fpt, tectonic's is %.2fpt", i+1, g, w)
		}
	}
}

// The same witness, vertically: the gap between consecutive baselines, in pt,
// against tectonic's (its PDF baselines in bp, differenced and converted). The
// 22pt gaps are a list's \topsep + \partopsep around it, the 20pt one is
// description's \itemsep + \parsep between its two items. With \list setting
// \parskip\parsep (as it must, ltlists.dtx:567) and the item still adding \parsep
// itself, every list started 4pt low and the error grew by one \parsep per list —
// India 15.9bp below the reference's — while every x0 above was already exact.
func TestParityListSpacingMatchesTheReference(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "listmargin.tex"))
	if err != nil {
		t.Fatal(err)
	}
	pages, err := CompileToSVGPages(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 1 {
		t.Fatalf("%d pages, want 1", len(pages))
	}
	ys := baselineYs(t, pages[0])
	want := []float64{12, 22, 12, 12, 22, 12, 12, 12, 22, 12, 22, 12, 20, 22, 12}
	// 16 body baselines and the folio's.
	if len(ys) != len(want)+2 {
		t.Fatalf("%d baselines, want %d: %v", len(ys), len(want)+2, ys)
	}
	for i, w := range want {
		if g := ys[i+1] - ys[i]; g < w-0.05 || g > w+0.05 {
			t.Errorf("gap %d (line %d to %d) is %.2fpt, tectonic's is %.2fpt", i+1, i+1, i+2, g, w)
		}
	}
}
