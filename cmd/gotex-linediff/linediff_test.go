// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"strings"
	"testing"

	engine "github.com/go-tex/engine"
)

// pdfOf typesets a document with the engine itself, so the tests measure a real
// PDF — the very kind this tool is pointed at — without a fixture to go stale.
func pdfOf(t *testing.T, src string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := engine.CompileToPDF([]byte(src), engine.Options{}, &buf); err != nil {
		t.Fatalf("CompileToPDF: %v", err)
	}
	return buf.Bytes()
}

// The whole point of the tool: a line's text has to come back. Reading the x of
// every run is no use if the report cannot say WHICH line each one is.
func TestRunsCarryTheirText(t *testing.T) {
	b := pdfOf(t, `\documentclass{article}
\begin{document}
Ambidextrous penguins.
\end{document}`)
	doc, err := openPDF(b)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := PageRuns(doc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) == 0 {
		t.Fatal("no text runs on a page that has text")
	}
	var all strings.Builder
	for _, r := range runs {
		all.WriteString(r.Text)
	}
	if !strings.Contains(all.String(), "Ambidextrous") {
		t.Errorf("the page's text came back as %q; the ToUnicode CMap was not read", all.String())
	}
}

// A run has to know how wide it is, or a report cannot say where a line ENDS —
// and "ends at the right margin" is half of what a contents entry is checked for.
func TestRunsCarryTheirWidth(t *testing.T) {
	doc, err := openPDF(pdfOf(t, `\documentclass{article}
\begin{document}
Hamburgefonstiv
\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	runs, err := PageRuns(doc, 1)
	if err != nil {
		t.Fatal(err)
	}
	var wide int
	for _, r := range runs {
		if r.W > 0 {
			wide++
		}
	}
	if wide == 0 {
		t.Error("every run reported width 0: the font's widths were not read")
	}
}

// A dot leader is the one thing a text-only reader CANNOT see: "a. . . . 7" reads
// the same whether the tiles are 4.8pt apart or 8.5. The pitch has to come from
// the drawn positions, and it has to be right.
func TestLeaderPitchComesFromThePositions(t *testing.T) {
	rs := []Run{
		{X: 10, Text: "Title"},
		{X: 40, Text: "."}, {X: 48, Text: "."}, {X: 56, Text: "."}, {X: 64, Text: "."},
		{X: 100, Text: "7"},
	}
	n, pitch := leaderOf(rs)
	if n != 4 || pitch != 8 {
		t.Errorf("leaderOf = %d tiles at %v, want 4 at 8", n, pitch)
	}
	// Two tiles are not a leader: a pitch from a single gap is noise.
	if n, pitch := leaderOf([]Run{{X: 1, Text: "."}, {X: 9, Text: "."}}); n != 2 || pitch != 0 {
		t.Errorf("two tiles reported %d at %v, want 2 at 0", n, pitch)
	}
	if n, _ := leaderOf(nil); n != 0 {
		t.Errorf("an empty line reported %d tiles", n)
	}
}

// The matrix has to compose the way PDF composes, or every position is wrong by
// whatever the page's CTM is.
func TestMatrixComposition(t *testing.T) {
	scale := matrix{2, 0, 0, 2, 0, 0}
	move := matrix{1, 0, 0, 1, 5, 7}
	// Scale first, then translate: the translation is NOT scaled.
	x, y := scale.mul(move).apply(1, 1)
	if x != 7 || y != 9 {
		t.Errorf("scale then move = (%v,%v), want (7,9)", x, y)
	}
	// Translate first, then scale: it is.
	x, y = move.mul(scale).apply(1, 1)
	if x != 12 || y != 16 {
		t.Errorf("move then scale = (%v,%v), want (12,16)", x, y)
	}
	if got := (matrix{3, 0, 0, 3, 0, 0}).scale(); got != 3 {
		t.Errorf("scale() = %v, want 3", got)
	}
}

// Lines are read down the page, not up it: a reader comparing two engines counts
// from the top, and a report that counted from the bottom would pair line 1 with
// the last line of the other document.
func TestLinesComeOutTopDown(t *testing.T) {
	runs := []Run{{X: 1, Y: 100, Text: "lower"}, {X: 1, Y: 700, Text: "upper"}}
	got := groupLines(runs, 792, reportOptions{})
	if len(got) != 2 || got[0].Text != "upper" {
		t.Fatalf("lines = %+v, want the upper one first", got)
	}
	if got[0].Y != 92 {
		t.Errorf("the upper line's y = %v, want 92 points from the top", got[0].Y)
	}
}

// utf16BEOffset advances the LAST unit of the destination, which is what a
// bfrange means; advancing the whole value would walk off into another plane.
func TestUTF16BEOffset(t *testing.T) {
	if got := utf16BEOffset([]byte{0x00, 'A'}, 2); got != "C" {
		t.Errorf("offset 2 from A = %q, want C", got)
	}
	if got := utf16BEOffset([]byte{0x00, 'A'}, 0); got != "A" {
		t.Errorf("offset 0 from A = %q, want A", got)
	}
	if got := utf16BEOffset([]byte{0x41}, 1); got != "" {
		t.Errorf("a half unit decoded as %q, want the empty string", got)
	}
}

func TestStripSubsetTag(t *testing.T) {
	for in, want := range map[string]string{
		"ABCDEF+LMRoman10-Regular": "LMRoman10-Regular",
		"LMRoman10-Regular":        "LMRoman10-Regular",
		"AB+Short":                 "AB+Short", // a '+' that is not a subset tag
	} {
		if got := stripSubsetTag(in); got != want {
			t.Errorf("stripSubsetTag(%q) = %q, want %q", in, got, want)
		}
	}
}

// Neither engine draws a space glyph, so a line read back run by run says
// "Firstsection1" unless the gaps are read as word breaks. A report nobody can
// read is a report nobody checks.
func TestWordBreaksComeBackFromTheGaps(t *testing.T) {
	// Two runs 10pt wide at size 10, the second starting 5pt after the first ends.
	runs := []Run{
		{X: 0, W: 10, Size: 10, Text: "First", Y: 700},
		{X: 15, W: 10, Size: 10, Text: "section", Y: 700},
		{X: 25, W: 5, Size: 10, Text: "1", Y: 700}, // touching: no break
	}
	got := groupLines(runs, 792, reportOptions{})
	if len(got) != 1 || got[0].Text != "First section1" {
		t.Fatalf("line text = %q, want %q", got[0].Text, "First section1")
	}
	if got[0].X1 != 30 {
		t.Errorf("line ends at %v, want 30 (the last run's origin plus its width)", got[0].X1)
	}
	// A run of unknown width must say so rather than report a short line.
	got = groupLines([]Run{{X: 0, Size: 10, Text: "x", Y: 700}}, 792, reportOptions{})
	if !got[0].unknownW {
		t.Error("a run with no width did not mark the line's end unknown")
	}
}
