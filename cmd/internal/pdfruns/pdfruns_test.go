// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package pdfruns

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	engine "github.com/go-tex/engine"
)

// pdfOf typesets a document with the engine itself, so the tests measure a real
// PDF — the very kind this package is pointed at — without a fixture to go stale.
//
// ⚠ gotex-linediff's own tests keep a copy of this helper. Two three-line test
// helpers are cheaper than a third package that exists only to hold one, and a
// shared one would make every test here wait on a change made for that tool.
func pdfOf(t *testing.T, src string) []byte {
	t.Helper()
	var buf strings.Builder
	if _, err := engine.CompileToPDF([]byte(src), engine.Options{}, &buf); err != nil {
		t.Fatalf("CompileToPDF: %v", err)
	}
	return []byte(buf.String())
}

// handPDF assembles a minimal one-page PDF with a SIMPLE font (one byte per
// glyph, /FirstChar + /Widths) and whatever content stream the caller names.
// Neither go-tex nor tectonic emits a simple font or most of the text operators,
// so without a PDF built here those paths would go untested — and the reference
// side of every comparison is a PDF this package did not produce.
func handPDF(t *testing.T, content string) []byte {
	t.Helper()
	return handPDFWith(t, "", content)
}

// handPDFWith is handPDF with extra keys in the content stream's dictionary —
// " /Filter /FlateDecode" to declare a filter the bytes may not honour.
func handPDFWith(t *testing.T, streamDict, content string) []byte {
	t.Helper()
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		"<< /Length " + strconv.Itoa(len(content)) + streamDict + " >>\nstream\n" + content + "\nendstream",
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

// ⛔⛔ A DAMAGED PAGE IS COUNTED IN bad — and until this change it never could be.
//
// Glyphs has long said "a page that fails to decode contributes nothing and is
// counted in bad, because a damaged page is not an empty one". But PageRuns read
// pages through reader.PageOperations, which SALVAGES an undecodable stream and
// returns no error, so the bad branch was unreachable: a corrupt file read as a
// document that had lost its text — the exact reading the comment exists to
// prevent. The comment promised a branch the code could not take.
//
// Two halves, asserted against each other: a page that decodes and reads, and a
// page whose stream claims FlateDecode and does not inflate. Without the control,
// a Glyphs that reported every page bad would pass the second row.
//
// ⚠ The control is a PDF the ENGINE wrote, not handPDF's. handPDF's simple font
// has no ToUnicode, so its runs decode with Text "" and Glyphs counts 0 for a
// page that read perfectly well — the first version of this control did exactly
// that and failed on a page with nothing wrong with it.
func TestGlyphsCountsAPageThatDidNotDecodeAsBad(t *testing.T) {
	good, err := Open(pdfOf(t, `\documentclass{article}
\begin{document}
Quarantine.
\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	if total, bad := Glyphs(good); bad != 0 || total == 0 {
		t.Errorf("a page that decodes: total=%d bad=%d, want total>0 bad=0", total, bad)
	}

	damaged, err := Open(handPDFWith(t, " /Filter /FlateDecode", "this is not a zlib stream at all"))
	if err != nil {
		t.Fatalf("the damaged PDF did not open (%v) — the test would measure Open, not the page", err)
	}
	if n := damaged.PageCount(); n != 1 {
		t.Fatalf("the damaged PDF has %d pages, want 1", n)
	}
	if total, bad := Glyphs(damaged); bad != 1 || total != 0 {
		t.Errorf("a page whose stream does not decode: total=%d bad=%d, want total=0 bad=1 — read as an empty page instead of a damaged one", total, bad)
	}
}

// The whole point of the package: a line's text has to come back. Reading the x
// of every run is no use if the caller cannot say WHICH line each one is.
func TestRunsCarryTheirText(t *testing.T) {
	b := pdfOf(t, `\documentclass{article}
\begin{document}
Ambidextrous penguins.
\end{document}`)
	doc, err := Open(b)
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

// A run has to know how wide it is, or a caller cannot say where a line ENDS —
// and "ends at the right margin" is half of what a contents entry is checked for.
func TestRunsCarryTheirWidth(t *testing.T) {
	doc, err := Open(pdfOf(t, `\documentclass{article}
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

// PageHeight comes from /MediaBox, and a page without a readable one reports 0
// rather than a guess — the baselines then come out as PDF user-space y, which a
// caller's own heading can tell its reader.
func TestPageHeight(t *testing.T) {
	doc, err := Open(pdfOf(t, `\documentclass{article}
\begin{document}x\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := PageHeight(doc, 1); got != 792 {
		t.Errorf("PageHeight = %v, want 792 (US letter)", got)
	}
	if got := PageHeight(doc, 99); got != 0 {
		t.Errorf("PageHeight of a page that does not exist = %v, want 0", got)
	}
}

// A simple (one byte per glyph) font keys its widths on the byte through
// /FirstChar and /Widths. Reading a simple font as a composite one — or the
// reverse — halves or doubles every position a caller reports.
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
	// A font whose widths could not be read reports 0, which a caller marks "?".
	if got := (&fontInfo{}).Width([]byte("AA"), 10, 0, 0); got != 0 {
		t.Errorf("a font with no widths returned %v, want 0", got)
	}
	// And a font with no ToUnicode decodes to nothing rather than to mojibake.
	if got := (&fontInfo{}).Decode([]byte("AA")); got != "" {
		t.Errorf("a font with no ToUnicode decoded %q", got)
	}
}

// The text operators a producer other than these two engines may use. Every one
// of them moves the pen, so every one of them can put a line in the wrong place.
func TestTheOtherTextOperators(t *testing.T) {
	// A: 1000 units wide, B: 500, anything else: /MissingWidth 250.
	doc, err := Open(handPDF(t, `BT /F1 10 Tf 20 TL
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
	doc, err := Open(handPDF(t, "BT /F1 10 Tf 0 50 Td [(A) -1000 (A)] TJ ET"))
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

// ⭐ Glyphs is what gotex-abdiff gained from this move: the ink on the page, in
// the unit texmf/LICENSE-NOTE.md's xkeyval entry already reports.
//
// ⛔ The control is the point of the test: a document with MORE text must count
// more glyphs than one with less. Asserting a bare number would pin whatever the
// engine happens to emit today; asserting the ORDER pins the meaning.
func TestGlyphsCountsInkAndNotPages(t *testing.T) {
	short, err := Open(pdfOf(t, `\documentclass{article}
\begin{document}xy\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	long, err := Open(pdfOf(t, `\documentclass{article}
\begin{document}Ambidextrous penguins quarrel umbrella quarantine.\end{document}`))
	if err != nil {
		t.Fatal(err)
	}
	ns, bads := Glyphs(short)
	nl, badl := Glyphs(long)
	t.Logf("short %d glyph(s), long %d glyph(s)", ns, nl)
	if bads != 0 || badl != 0 {
		t.Fatalf("a page failed to decode (%d and %d): the counts cannot be compared", bads, badl)
	}
	if ns == 0 {
		t.Fatal("a document with text counted zero glyphs")
	}
	if nl <= ns {
		t.Errorf("the longer document counted %d glyphs against the shorter one's %d: "+
			"Glyphs is not measuring ink", nl, ns)
	}
	// Both are one page, so a page count could not tell them apart — which is
	// the whole reason this exists.
	if short.PageCount() != 1 || long.PageCount() != 1 {
		t.Fatalf("the two documents are %d and %d pages; this test needs both at one "+
			"page to show that a page count cannot see the difference",
			short.PageCount(), long.PageCount())
	}
}
