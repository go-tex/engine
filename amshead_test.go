// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// amsart's headings, against tectonic, on testdata/parity/amshead.tex: a \section,
// a run-in \subsection and \subsubsection, and a \section*. amsart.cls defines its
// own \@startsection and \@sect (amsart.cls:1020-1077), and they end in the kernel's
// \@xsect and \@afterheading (ltsect.dtx:481-517, :615-627).
//
// Before, both were stand-ins:
//
//	                                tectonic   before
//	section heading → its text      17.94bp    11.95    \@xsect dropped the afterskip
//	text → \subsection              17.93bp    11.95    \@afterheading left \@nobreak
//	                                                    true, so \addvspace was skipped
//	\subsection, \subsubsection     run-in     a line of their own
//
// The line count is the run-in: with the stand-in each run-in heading was a
// paragraph of its own, two lines more.
func TestParityAmsartHeadingsMatchTheReference(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "amshead.tex"))
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
	// tectonic's baselines in bp, differenced and converted to pt (×72.27/72):
	// heading→text, text, text→\subsection, its 2nd line, →\subsubsection,
	// →\section*, →its text.
	want := []float64{18.01, 12.0, 18.0, 12.0, 18.0, 20.4, 18.01}
	// 8 body baselines and the folio's.
	if len(ys) != len(want)+2 {
		t.Fatalf("%d baselines, want %d (a run-in heading on a line of its own?): %v", len(ys), len(want)+2, ys)
	}
	for i, w := range want {
		if g := ys[i+1] - ys[i]; g < w-0.05 || g > w+0.05 {
			t.Errorf("gap %d (line %d to %d) is %.2fpt, tectonic's is %.2fpt", i+1, i+1, i+2, g, w)
		}
	}
	// Run-in: each heading's number on the SAME baseline as its text. The count above
	// cannot tell a run-in heading from a heading that vanished.
	for _, pair := range [][2]string{{"1.1.", "subsection,"}, {"1.1.1.", "subsubsection,"}} {
		h, ok1 := wordY(pages[0], pair[0])
		x, ok2 := wordY(pages[0], pair[1])
		if !ok1 || !ok2 {
			t.Errorf("%q found %v, %q found %v: a heading or its text is missing", pair[0], ok1, pair[1], ok2)
			continue
		}
		if h != x {
			t.Errorf("%q is on baseline %.2f and its text %q on %.2f: not run in", pair[0], h, pair[1], x)
		}
	}
}

// wordY is the baseline of the first <tspan> whose text is exactly word.
func wordY(svg, word string) (float64, bool) {
	re := regexp.MustCompile(`<tspan[^>]*\sy="([0-9.]+)"[^>]*>([^<]*)</tspan>`)
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		if strings.TrimSpace(m[2]) != word {
			continue
		}
		v, err := strconv.ParseFloat(m[1], 64)
		return v, err == nil
	}
	return 0, false
}
