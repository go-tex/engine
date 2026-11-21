// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// \/ is the glyph's italic correction (XeTeX: max(0, bbox.xMax − advance)) and
// \textit/\emph/\textbf/… end with one unless "," or "." follows (the kernel's
// \check@icr, \nocorrlist). \/ was \def\/{} and the text commands were {#2#1}, so
// every row below except the four plain ones was short. Each want is the width
// tectonic prints for the same line of testdata/parity/italcorr.tex.
//
// \textsl{f} needs the CONTROL box (GlyphControlBox): read from the flattened
// outline it was 4.64pt. Not here: \textsf{f} is set in the roman, which is the
// missing sans family, not the correction.
func TestParityItalicCorrection(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "italcorr.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, f := range strings.Fields(e.out.String()) {
		name, val, ok := strings.Cut(f, "=")
		if !ok || !strings.HasSuffix(val, "pt") {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSuffix(val, "pt"), 64)
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		got[name] = v
	}
	want := []struct {
		name string
		pt   float64
	}{
		{"f", 3.06},
		{"f-ic", 3.56999},
		{"itshape-f", 3.07},
		{"itshape-f-ic", 4.52},
		{"itshape-d", 5.11},
		{"itshape-d-ic", 5.6},
		{"fontcharic-f", 0.50998},
		{"fontcharic-itshape-f", 1.44998},
		{"fontcharic-itshape-o", 0},
		{"textit-f", 4.52},
		{"textit-d", 5.6},
		{"textit-f-dot", 5.85},
		{"textit-f-comma", 5.85},
		{"textit-f-paren", 8.41},
		{"textit-f-nocorr", 4.52},
		{"textit-nocorr-f", 4.52},
		{"textit-f-nocorrin", 3.07},
		{"textit-f-space-x", 13.13},
		{"textit-f-skip-x", 12.8},
		{"textit-f-skip-in-x", 11.8},
		{"textit-f-penalty-x", 9.8},
		{"textit-empty", 0},
		{"textit-ff", 9.04001},
		{"x-textit-f", 9.8},
		{"emph-f", 4.52},
		{"itshape-emph-f", 4.52},
		{"textup-itshape-f", 4.52},
		{"itshape-f-textup-x", 9.8},
		{"textbf-f", 4.37},
		{"bfseries-f", 3.51},
		{"textrm-f", 3.56999},
		{"textsc-f", 5.36},
		{"texttt-f", 5.25},
		{"textsl-f", 4.69},
	}
	for _, w := range want {
		v, ok := got[w.name]
		if !ok {
			t.Errorf("%s: no width printed", w.name)
			continue
		}
		if d := v - w.pt; d > 0.005 || d < -0.005 {
			t.Errorf("%s is %.5gpt, tectonic %.5gpt", w.name, v, w.pt)
		}
	}
}

// A text command in a running head survives the mark's \edef. sn-jnl.cls:486
// writes \markboth{\textit{\s@title}}{\textit{\s@title}}; \textit is robust in
// LaTeX, and \protected here. Defined with a plain \def, it expanded inside that
// \edef into \text@command's own \edef and \futurelet, and the head of every page
// of 2209.13768 was lost with its page number.
func TestATextCommandInARunningHeadReachesTheHead(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\pagestyle{headings}\begin{document}` +
		`\markboth{\textit{Tete}}{\textit{Tete}}Premiere page.` +
		`\newpage Deuxieme page.\end{document}`); err != nil {
		t.Fatal(err)
	}
	if page := pageText(e); !strings.Contains(page, "Tete") {
		t.Errorf("the \\textit head did not reach the page: %q", page)
	}
}
