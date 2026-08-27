// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// A page is the PAPER, and the text block sits inside it where the class says.
//
// The engine used to derive a page from its content — content plus a uniform margin
// — which is right only when the two coincide. classes.dtx has article, report and
// book run \ExecuteOptions{letterpaper,10pt,…}, so \paperwidth/\paperheight hold
// 8.5in x 11in unless an option says otherwise; [a4paper] sets 210mm x 297mm; and
// geometry publishes its own paper into the same two registers. One source of truth.
//
// Position comes from the same file, "Page Layout": "All margin dimensions are
// measured from a point one inch from the top and lefthand side of the page", with
// \oddsidemargin computed as .5(\paperwidth - \textwidth) - 1in.

var svgSizeRe = regexp.MustCompile(`width="([0-9.]+)pt" height="([0-9.]+)pt"`)

func pageSize(t *testing.T, src string) (w, h float64) {
	t.Helper()
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(pages) == 0 {
		t.Fatalf("no page produced for %q", src)
	}
	m := svgSizeRe.FindStringSubmatch(pages[0])
	if m == nil {
		t.Fatalf("no page size in the SVG for %q", src)
	}
	w, _ = strconv.ParseFloat(m[1], 64)
	h, _ = strconv.ParseFloat(m[2], 64)
	return w, h
}

func near(got, want float64) bool { return got-want < 0.05 && want-got < 0.05 }

func TestPageIsThePaper(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		w, h float64
	}{
		{"article defaults to letterpaper", `\documentclass{article}\begin{document}X\end{document}`, 614.29, 794.97},
		{"a4paper", `\documentclass[a4paper]{article}\begin{document}X\end{document}`, 597.51, 845.05},
		{"report defaults to letterpaper", `\documentclass{report}\begin{document}X\end{document}`, 614.29, 794.97},
		// 20cm x 25cm in TeX points, not big points: 1in = 72.27pt, so 1cm =
		// 28.45276pt and 20cm = 569.06pt. (72bp/in would give 567, which is what a
		// PDF viewer's "cm" means and what TeX's "bp" unit is for.)
		{"geometry states its own", `\documentclass{article}\usepackage[paperwidth=20cm,paperheight=25cm,margin=1cm]{geometry}\begin{document}X\end{document}`, 569.06, 711.32},
	} {
		w, h := pageSize(t, c.src)
		if !near(w, c.w) || !near(h, c.h) {
			t.Errorf("%s: page %gx%g, want %gx%g", c.name, w, h, c.w, c.h)
		}
	}
}

func TestPageSizeDoesNotDependOnHowMuchIsOnIt(t *testing.T) {
	// The whole point: a nearly empty page is the same sheet as a full one.
	w1, h1 := pageSize(t, `\documentclass{article}\begin{document}X\end{document}`)
	w2, h2 := pageSize(t, `\documentclass{article}\begin{document}`+strings.Repeat("mot ", 400)+`\end{document}`)
	if !near(w1, w2) || !near(h1, h2) {
		t.Errorf("a short page is %gx%g and a long one %gx%g — the sheet must not change", w1, h1, w2, h2)
	}
}

func TestTextBlockSitsWhereTheClassPutsIt(t *testing.T) {
	// classes.dtx: left = 1in + \hoffset + \oddsidemargin. article at 10pt on
	// letterpaper sets \oddsidemargin=62pt, so the block starts 134.27pt in; the
	// first glyph of an indented paragraph adds \parindent (15pt) on top.
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}X\end{document}`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := e.renderMargin(72); !near(got, 134.27) {
		t.Errorf("left margin %g, want 134.27 (1in + \\oddsidemargin 62pt)", got)
	}
	// top = 1in + \voffset + \topmargin + \headheight + \headsep = 72.27+16+12+25.
	if got := e.renderVMargin(72); !near(got, 125.27) {
		t.Errorf("top margin %g, want 125.27", got)
	}
}

func TestPaperUnknownKeepsTheOldModel(t *testing.T) {
	// With no class — plain TeX — nothing states a paper, and the page is still
	// derived from the content plus the caller's margin.
	e, err := buildEngine(Options{}, false)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, _, ok := e.paperSizePt(); ok {
		t.Errorf("plain TeX should not claim a paper size")
	}
	if got := e.renderMargin(72); !near(got, 72) {
		t.Errorf("fallback margin %g, want the caller's 72", got)
	}
}
