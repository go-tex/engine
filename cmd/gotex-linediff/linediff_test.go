// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	engine "github.com/go-tex/engine"
	"github.com/go-tex/engine/cmd/internal/pdfruns"
)

// ⚠ The PDF-reading tests moved with the code they test, to
// cmd/internal/pdfruns: the text operators, the matrix, the font widths, the
// ToUnicode decoding, the subset tag, the page height and Glyphs. What is left
// here is what belongs to THIS tool — grouping runs into lines, the leader
// pitch, the -above filter, the report, and its formatting.

// pdfOf typesets a document with the engine itself, so the tests measure a real
// PDF without a fixture to go stale.
//
// ⚠ cmd/internal/pdfruns keeps a copy. Two three-line test helpers are cheaper
// than a package that exists to hold one, and a shared one would make these
// tests wait on a change made for that package.
func pdfOf(t *testing.T, src string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := engine.CompileToPDF([]byte(src), engine.Options{}, &buf); err != nil {
		t.Fatalf("CompileToPDF: %v", err)
	}
	return buf.Bytes()
}

// A dot leader is the one thing a text-only reader CANNOT see: "a. . . . 7" reads
// the same whether the tiles are 4.8pt apart or 8.5. The pitch has to come from
// the drawn positions, and it has to be right.
func TestLeaderPitchComesFromThePositions(t *testing.T) {
	rs := []pdfruns.Run{
		{X: 10, Text: "Title"},
		{X: 40, Text: "."}, {X: 48, Text: "."}, {X: 56, Text: "."}, {X: 64, Text: "."},
		{X: 100, Text: "7"},
	}
	n, pitch := leaderOf(rs)
	if n != 4 || pitch != 8 {
		t.Errorf("leaderOf = %d tiles at %v, want 4 at 8", n, pitch)
	}
	// Two tiles are not a leader: a pitch from a single gap is noise.
	if n, pitch := leaderOf([]pdfruns.Run{{X: 1, Text: "."}, {X: 9, Text: "."}}); n != 2 || pitch != 0 {
		t.Errorf("two tiles reported %d at %v, want 2 at 0", n, pitch)
	}
	if n, _ := leaderOf(nil); n != 0 {
		t.Errorf("an empty line reported %d tiles", n)
	}
}

// Lines are read down the page, not up it: a reader comparing two engines counts
// from the top, and a report that counted from the bottom would pair line 1 with
// the last line of the other document.
func TestLinesComeOutTopDown(t *testing.T) {
	runs := []pdfruns.Run{{X: 1, Y: 100, Text: "lower"}, {X: 1, Y: 700, Text: "upper"}}
	got := groupLines(runs, 792, reportOptions{})
	if len(got) != 2 || got[0].Text != "upper" {
		t.Fatalf("lines = %+v, want the upper one first", got)
	}
	if got[0].Y != 92 {
		t.Errorf("the upper line's y = %v, want 92 points from the top", got[0].Y)
	}
}

// Neither engine draws a space glyph, so a line read back run by run says
// "Firstsection1" unless the gaps are read as word breaks. A report nobody can
// read is a report nobody checks.
func TestWordBreaksComeBackFromTheGaps(t *testing.T) {
	// Two runs 10pt wide at size 10, the second starting 5pt after the first ends.
	runs := []pdfruns.Run{
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
	got = groupLines([]pdfruns.Run{{X: 0, Size: 10, Text: "x", Y: 700}}, 792, reportOptions{})
	if !got[0].unknownW {
		t.Error("a run with no width did not mark the line's end unknown")
	}
}

// The report end to end: two files, a heading per file, and the lines in page
// order. This is what a reader actually sees, so it is what is checked.
func TestReportReadsTwoFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.pdf")
	if err := os.WriteFile(a, pdfOf(t, `\documentclass{article}
\begin{document}
Quarantine. \par Umbrella.
\end{document}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := run([]string{a, a}, reportOptions{page: 1}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if n := strings.Count(out, "### "); n != 2 {
		t.Errorf("%d file headings, want 2:\n%s", n, out)
	}
	for _, want := range []string{"Quarantine", "Umbrella", "page 1", "height 792.00 pt"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not mention %q:\n%s", want, out)
		}
	}
	if i, j := strings.Index(out, "Quarantine"), strings.Index(out, "Umbrella"); i > j {
		t.Error("the lines came out bottom-up; a reader counts down a page")
	}
	// -grep keeps only the lines that say so.
	buf.Reset()
	if err := run([]string{a}, reportOptions{page: 1, grep: "Umbrella"}, &buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Quarantine") {
		t.Errorf("-grep let an unmatched line through:\n%s", buf.String())
	}
	// A page the document does not have is an error, not an empty report.
	if err := run([]string{a}, reportOptions{page: 99}, &buf); err == nil {
		t.Error("page 99 of a one-page document reported no error")
	}
	if err := run([]string{filepath.Join(dir, "nothing.pdf")}, reportOptions{page: 1}, &buf); err == nil {
		t.Error("a missing file reported no error")
	}
}

// -above measures from the TOP of the page, which is the only direction that
// keeps its meaning when the two documents have different page heights.
func TestAboveFiltersFromTheTop(t *testing.T) {
	runs := []pdfruns.Run{{X: 1, Y: 700, W: 1, Size: 10, Text: "near the top"}, {X: 1, Y: 100, W: 1, Size: 10, Text: "far down"}}
	got := groupLines(runs, 792, reportOptions{above: 200})
	if len(got) != 1 || got[0].Text != "near the top" {
		t.Errorf("above=200 kept %+v, want only the line 92pt from the top", got)
	}
}

// A line with an unknown width must SAY its end is unknown rather than print a
// number that is short by however much the last run advances.
func TestLineStringMarksAnUnknownEnd(t *testing.T) {
	known := Line{Y: 1, X0: 2, X1: 3, Font: "F", Size: 10, Text: "x"}
	if s := known.String(); !strings.Contains(s, "x1=    3.00") {
		t.Errorf("a known end printed as %q", s)
	}
	if s := (Line{unknownW: true, Font: "F"}).String(); !strings.Contains(s, "x1=       ?") {
		t.Errorf("an unknown end printed as %q", s)
	}
	withLeader := Line{Font: "F", Leader: 7, Pitch: 8.5}
	if s := withLeader.String(); !strings.Contains(s, "leader=  7") || !strings.Contains(s, "pitch=  8.50") {
		t.Errorf("a leader printed as %q", s)
	}
}

func TestTrunc(t *testing.T) {
	if got := trunc("abcdef", 4); got != "abc…" {
		t.Errorf("trunc = %q, want %q", got, "abc…")
	}
	if got := trunc("abc", 4); got != "abc" {
		t.Errorf("trunc shortened a string that fit: %q", got)
	}
	// Counted in runes, not bytes: an accented title must not be cut mid-character.
	if got := trunc("éééé", 3); got != "éé…" {
		t.Errorf("trunc = %q, want %q", got, "éé…")
	}
}

// ftoa came back here when the PDF reading moved out: it formats this tool's
// report, and no caller in pdfruns used it.
func TestFtoa(t *testing.T) {
	if got := ftoa(8.4949); got != "8.49" {
		t.Errorf("ftoa = %q, want %q", got, "8.49")
	}
}
