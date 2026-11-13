// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"strings"
	"testing"
)

// ⛔ THE ENGINE DETECTED OVERFULL LINES AND TOLD NOBODY. hasBadLine decides
// which line-breaking pass to run and has done since line breaking was written;
// no caller could see it, so a document whose text runs into the margin
// compiled in silence. A real TeX's most common complaint, and the reason a
// reader opens the log at all.
//
// ⭐ MEASURED AGAINST tectonic 0.17.0 on a \texttt token at \hsize=40pt — one
// word with no break points, wider than the measure:
//
//	reference   Overfull \hbox (133.25pt too wide)
//	here        the worst by 133.25pt
//
// ⛔⛔ AND THE FIRST VERSION MISSED EXACTLY THAT CASE. It asked each Line for
// its Ratio and counted `Ratio < -1`. The ratio is the GLUE adjustment, and a
// line whose content is one unbreakable token has no shrinkable glue at all —
// so there is no ratio to be past -1, and the count was zero while the line
// hung 133pt into the margin. The width is what a reader sees, so the width is
// what is measured.
func TestAnOverfullLineIsReportedWithItsOverflow(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=40pt` +
		`\noindent \texttt{incomprehensibilitiesoverwhelming}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.OverfullLines == 0 {
		t.Fatal("an unbreakable 173pt word on a 40pt measure was not reported overfull")
	}
	// tectonic says 133.25pt too wide for this document. A tenth of a point is
	// finer than any rounding difference between the two and far coarser than
	// the 133pt the defect was hiding.
	if math.Abs(d.WorstOverfull-133.25) > 0.1 {
		t.Errorf("worst overflow = %.2fpt, tectonic reports 133.25pt", d.WorstOverfull)
	}
}

// ⛔ AND AN ORDINARY DOCUMENT MUST STAY SILENT, or the warning is noise and a
// reader learns to ignore it — which is worse than not having it.
func TestAnOrdinaryParagraphReportsNothing(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`Typography affects every line of every paper, and a measure that is ` +
		`wrong by a centimetre is wrong on every line it sets.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.OverfullLines != 0 || d.UnderfullLines != 0 {
		t.Errorf("a paragraph at the normal measure reported %d overfull and %d underfull "+
			"line(s); both must be 0", d.OverfullLines, d.UnderfullLines)
	}
	if d.WorstOverfull != 0 {
		t.Errorf("worst overflow = %.2fpt on a clean paragraph, want 0", d.WorstOverfull)
	}
}

// ⭐ THE LAST-RESORT PATH, WHICH THIS REPORT IS WHAT FOUND.
//
// An INDENTED paragraph at \hsize=45pt whose first word is `Typographical`:
// 15pt of indent leave 30pt for a word that needs 65, and since #766 a
// paragraph's first word is not hyphenated (tex.web §894/§896, and the
// reference does the same). So no feasible set of breaks exists and
// breakSegment fails.
//
// ⛔⛔ THE OLD FALLBACK PUT THE WHOLE PARAGRAPH ON ONE LINE — 254.47pt past a
// 45pt margin, every word present and none of it readable. That is what this
// counter reported within minutes of existing, on its first real document.
//
// What the reference does, measured:
//
//	    Typographical      <- alone, OVERFULL, overhanging
//	incompre-
//	hensibil-
//	ities over-
//	whelm ev-
//	ery para-
//	graph.
//
// and what greedyLines now does: the same first line, the same first break, and
// a greedy approximation after it. 33.10pt is that first line's overhang.
func TestAParagraphWithNoFeasibleBreakIsStillReadable(t *testing.T) {
	src := `\documentclass{article}\begin{document}\hsize=45pt` +
		`\hbadness=10000 \tolerance=10000 ` +
		`Typographical incomprehensibilities overwhelm every paragraph.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	d := e.Diagnostics()
	if d.OverfullLines != 1 {
		t.Errorf("%d overfull line(s), want 1 — only the unbreakable first word "+
			"should overhang", d.OverfullLines)
	}
	if math.Abs(d.WorstOverfull-33.10) > 0.1 {
		t.Errorf("worst overflow = %.2fpt, want about 33.10 — the width of "+
			"`Typographical` past the 30pt left by the indent. 254pt would mean the "+
			"old one-line fallback is back", d.WorstOverfull)
	}
	// ⛔ AND EVERY CHARACTER MUST SURVIVE. Two drafts of greedyLines lost one at
	// each break — `incom / rehensibil / ties / helm / raph.` — by advancing past
	// the node at the breakpoint, which is the letter after a discretionary and
	// not only the space after a word. Content destroyed is worse than the
	// unreadable line this replaces.
	got := pageChars(e)
	for _, want := range []string{"Typographical", "incompre", "hensibili", "whelm", "graph."} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from the page: %q", want, firstN(got, 160))
		}
	}
}
