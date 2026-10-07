// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	runs := []Run{{X: 1, Y: 700, W: 1, Size: 10, Text: "near the top"}, {X: 1, Y: 100, W: 1, Size: 10, Text: "far down"}}
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

// pageHeight comes from /MediaBox, and a page without a readable one reports 0
// rather than a guess — the baselines then come out as PDF user-space y, which
// the heading's "height 0.00 pt" tells the reader.
func TestPageHeight(t *testing.T) {
	doc, err := openPDF(pdfOf(t, `\documentclass{article}
\begin{document}x\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := pageHeight(doc, 1); got != 792 {
		t.Errorf("pageHeight = %v, want 792 (US letter)", got)
	}
	if got := pageHeight(doc, 99); got != 0 {
		t.Errorf("pageHeight of a page that does not exist = %v, want 0", got)
	}
}

func TestFtoa(t *testing.T) {
	if got := ftoa(8.4949); got != "8.49" {
		t.Errorf("ftoa = %q, want %q", got, "8.49")
	}
}

// A simple (one byte per glyph) font keys its widths on the byte through
// /FirstChar and /Widths. Reading a simple font as a composite one — or the
// reverse — halves or doubles every position in the report.
func TestSimpleAndCompositeWidths(t *testing.T) {
	simple := &fontInfo{widths: map[int]float64{65: 500, 32: 250}, missing: 600}
	if got := simple.Width([]byte("AA"), 10, 0, 0); got != 10 {
		t.Errorf("two 500-unit glyphs at 10pt = %v, want 10", got)
	}
	if got := simple.Width([]byte("Z"), 10, 0, 0); got != 6 {
		t.Errorf("an unlisted glyph = %v, want /MissingWidth 600 at 10pt = 6", got)
	}
	if got := simple.Width([]byte(" "), 10, 0, 2); got != 2.5+2 {
		t.Errorf("word spacing on byte 32 = %v, want 4.5", got)
	}
	// The same bytes read as a composite font are ONE glyph, code 0x4141.
	composite := &fontInfo{twoByte: true, widths: map[int]float64{0x4141: 500}, missing: 1000}
	if got := composite.Width([]byte("AA"), 10, 0, 0); got != 5 {
		t.Errorf("one 500-unit CID at 10pt = %v, want 5", got)
	}
	// Word spacing never applies to a two-byte code, however the bytes read.
	if got := composite.Width([]byte("AA"), 10, 0, 99); got != 5 {
		t.Errorf("word spacing reached a composite font: %v", got)
	}
	// A font whose widths could not be read reports 0, which the line marks "?".
	if got := (&fontInfo{}).Width([]byte("AA"), 10, 0, 0); got != 0 {
		t.Errorf("a font with no widths returned %v, want 0", got)
	}
	// And a font with no ToUnicode decodes to nothing rather than to mojibake.
	if got := (&fontInfo{}).Decode([]byte("AA")); got != "" {
		t.Errorf("a font with no ToUnicode decoded %q", got)
	}
}

// handPDF assembles a minimal one-page PDF with a SIMPLE font (one byte per
// glyph, /FirstChar + /Widths) and whatever content stream the caller names.
// Neither go-tex nor tectonic emits a simple font or most of the text operators,
// so without a PDF built here those paths would go untested — and the reference
// side of every comparison is a PDF this tool did not produce.
func handPDF(t *testing.T, content string) []byte {
	t.Helper()
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		"<< /Length " + strconv.Itoa(len(content)) + " >>\nstream\n" + content + "\nendstream",
		"<< /Type /Font /Subtype /Type1 /BaseFont /ABCDEF+Helvetica /FirstChar 65 /Widths [1000 500] /FontDescriptor 6 0 R >>",
		"<< /Type /FontDescriptor /FontName /Helvetica /MissingWidth 250 >>",
	}
	var sb strings.Builder
	sb.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = sb.Len()
		fmt.Fprintf(&sb, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := sb.Len()
	fmt.Fprintf(&sb, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offs {
		fmt.Fprintf(&sb, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&sb, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return []byte(sb.String())
}

// The text operators a producer other than these two engines may use. Every one
// of them moves the pen, so every one of them can put a line in the wrong place.
func TestTheOtherTextOperators(t *testing.T) {
	// A: 1000 units wide, B: 500, anything else: /MissingWidth 250.
	doc, err := openPDF(handPDF(t, `BT /F1 10 Tf 20 TL
10 90 Td (A) Tj
0 -10 TD (B) Tj
T* (AB) Tj
(A) '
3 1 (B) "
ET
q 2 0 0 2 0 0 cm 5 Tc BT /F1 10 Tf 5 5 Td (A) Tj ET Q
BT /F1 10 Tf 200 Tz 0 20 Td (A) Tj ET`))
	if err != nil {
		t.Fatal(err)
	}
	runs, err := PageRuns(doc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 7 {
		t.Fatalf("%d runs, want 7: %+v", len(runs), runs)
	}
	want := []struct {
		x, y, w float64
		note    string
	}{
		{10, 90, 10, "Td"},
		{10, 80, 5, "TD moves AND overrides the 20 TL with its own 10"},
		{10, 70, 15, "T* uses TD's leading, not TL's"},
		{10, 60, 10, "' is T* then show"},
		{10, 50, 5 + 1, `" also sets the character spacing, which the width must carry`},
		// The " above left the character spacing at 1, and a text-state parameter
		// outlives its BT/ET: the only thing that takes it back is a Q.
		{10, 10, 2 * (10 + 5), "cm doubles the position AND the advance; 5 Tc applies inside the q"},
		{0, 20, 2 * (10 + 1), "after Q the spacing is 1 again, not the 5 set inside"},
	}
	for i, w := range want {
		if runs[i].X != w.x || runs[i].Y != w.y || runs[i].W != w.w {
			t.Errorf("run %d (%s) = x %v y %v w %v, want %v %v %v",
				i, w.note, runs[i].X, runs[i].Y, runs[i].W, w.x, w.y, w.w)
		}
	}
	// Q restored the CTM: the last run is back at scale 1.
	if runs[6].Size != 10 {
		t.Errorf("after Q the size reads %v, want 10: the CTM was not restored", runs[6].Size)
	}
	// A simple font with no ToUnicode has no text, and says so rather than
	// guessing that its bytes are Latin-1.
	if runs[0].Text != "" {
		t.Errorf("a font with no ToUnicode decoded %q", runs[0].Text)
	}
	if runs[0].Font != "Helvetica" {
		t.Errorf("font name %q, want the subset tag stripped", runs[0].Font)
	}
}

// TJ's numbers move the pen BACK by n/1000 of the size: that is how a producer
// kerns, and reading them as forward motion mirrors every kern on the line.
func TestTJNumbersMoveBackwards(t *testing.T) {
	doc, err := openPDF(handPDF(t, "BT /F1 10 Tf 0 50 Td [(A) -1000 (A)] TJ ET"))
	if err != nil {
		t.Fatal(err)
	}
	runs, err := PageRuns(doc, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 2 {
		t.Fatalf("%d runs, want 2", len(runs))
	}
	// A is 10pt wide; -1000 moves 10pt further on. The second A starts at 20.
	if runs[1].X != 20 {
		t.Errorf("the kerned run starts at %v, want 20", runs[1].X)
	}
}
