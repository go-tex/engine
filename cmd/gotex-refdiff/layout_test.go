// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// bboxDoc wraps page/word body XML in the same boilerplate pdftotext -bbox
// emits, so the fixtures exercise the real parser path (doctype, html/head/body).
func bboxDoc(body string) []byte {
	return []byte(`<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"><html xmlns="http://www.w3.org/1999/xhtml">
<head><title></title></head>
<body>
<doc>
` + body + `
</doc>
</body>
</html>`)
}

func TestParseBBoxPagesWordsAndLines(t *testing.T) {
	data := bboxDoc(`  <page width="600.000000" height="800.000000">
    <word xMin="100" yMin="100" xMax="140" yMax="112">Hello</word>
    <word xMin="150" yMin="100" xMax="190" yMax="112">world</word>
    <word xMin="100" yMin="130" xMax="140" yMax="142">next</word>
  </page>
  <page width="600" height="800">
    <word xMin="100" yMin="100" xMax="140" yMax="112">page2</word>
  </page>`)
	pl := parseBBox(data)
	if len(pl.pages) != 2 {
		t.Fatalf("pages=%d want 2", len(pl.pages))
	}
	if pl.pages[0].w != 600 || pl.pages[0].h != 800 {
		t.Fatalf("page0 dims = %v", pl.pages[0])
	}
	if len(pl.words) != 4 {
		t.Fatalf("words=%d want 4", len(pl.words))
	}
	// First word of a line/page is a line start; the second word on the same
	// baseline is not; a word on a new baseline is; the first word of page 2 is.
	wantLineStart := []bool{true, false, true, true}
	wantPage := []int{0, 0, 0, 1}
	for i, w := range pl.words {
		if w.lineStart != wantLineStart[i] {
			t.Errorf("word %d (%q) lineStart=%v want %v", i, w.text, w.lineStart, wantLineStart[i])
		}
		if w.page != wantPage[i] {
			t.Errorf("word %d (%q) page=%d want %d", i, w.text, w.page, wantPage[i])
		}
	}
}

func TestParseBBoxWordBeforePageIgnored(t *testing.T) {
	// A <word> before any <page> has no page geometry to anchor it and is dropped.
	data := bboxDoc(`  <word xMin="1" yMin="1" xMax="2" yMax="2">orphan</word>
  <page width="600" height="800">
    <word xMin="100" yMin="100" xMax="140" yMax="112">real</word>
  </page>`)
	pl := parseBBox(data)
	if len(pl.words) != 1 || pl.words[0].text != "real" {
		t.Fatalf("orphan word not dropped: %+v", pl.words)
	}
}

func TestParseBBoxDecodesEntities(t *testing.T) {
	// pdftotext escapes &, <, > as XML entities in word text; they are decoded.
	data := bboxDoc(`  <page width="600" height="800">
    <word xMin="100" yMin="100" xMax="140" yMax="112">A&amp;B</word>
  </page>`)
	pl := parseBBox(data)
	if len(pl.words) != 1 || pl.words[0].text != "A&B" {
		t.Fatalf("entity not decoded: %+v", pl.words)
	}
}

func TestParseBBoxTruncatedWord(t *testing.T) {
	// Input that ends mid-<word> (no closing </word>) contributes no word, but the
	// preceding page still stands and the scan does not panic.
	data := []byte(`<doc><page width="600" height="800"><word xMin="1" yMin="1" xMax="2" yMax="2">`)
	pl := parseBBox(data)
	if len(pl.pages) != 1 {
		t.Fatalf("pages=%d want 1", len(pl.pages))
	}
	if len(pl.words) != 0 {
		t.Fatalf("unterminated word should not be recorded: %+v", pl.words)
	}
}

func TestParseBBoxSurvivesXMLHostileBytes(t *testing.T) {
	// The regression this parser exists for: a real reference PDF's extracted text
	// carries bytes that are not valid XML (here a raw form-feed 0x0c and a bare
	// ampersand). An XML parser aborts on the first such byte, truncating a long
	// document to the pages before it. The byte scanner must parse ALL pages.
	body := "  <page width=\"600\" height=\"800\">\n" +
		"    <word xMin=\"100\" yMin=\"100\" xMax=\"140\" yMax=\"112\">bad\x0cchar & more</word>\n" +
		"  </page>\n" +
		"  <page width=\"600\" height=\"800\">\n" +
		"    <word xMin=\"100\" yMin=\"100\" xMax=\"140\" yMax=\"112\">page2word</word>\n" +
		"  </page>\n" +
		"  <page width=\"600\" height=\"800\">\n" +
		"    <word xMin=\"100\" yMin=\"100\" xMax=\"140\" yMax=\"112\">page3word</word>\n" +
		"  </page>"
	pl := parseBBox(bboxDoc(body))
	if len(pl.pages) != 3 {
		t.Fatalf("hostile bytes truncated the parse: got %d pages want 3", len(pl.pages))
	}
	if len(pl.words) != 3 || pl.words[2].text != "page3word" {
		t.Fatalf("words after the hostile byte were lost: %+v", pl.words)
	}
}

func TestParseBBoxUnparseableDimension(t *testing.T) {
	data := bboxDoc(`  <page width="abc" height="800">
    <word xMin="1" yMin="1" xMax="2" yMax="2">x</word>
  </page>`)
	pl := parseBBox(data)
	if pl.pages[0].w != 0 {
		t.Fatalf("unparseable width should be 0, got %v", pl.pages[0].w)
	}
}

func TestNormalizeMasksKnownDiffs(t *testing.T) {
	pl := pdfLayout{
		pages: []pageDim{{w: 100, h: 100}},
		words: []wordBox{
			{text: "Hello", page: 0, xMin: 10, yMin: 10, xMax: 30, yMax: 20},  // kept
			{text: "ﬁrst", page: 0, xMin: 40, yMin: 10, xMax: 60, yMax: 20},   // ligature → kept
			{text: "42", page: 0, xMin: 10, yMin: 30, xMax: 20, yMax: 40},     // numeric → dropped
			{text: "x", page: 0, xMin: 30, yMin: 30, xMax: 34, yMax: 40},      // single char → dropped
			{text: "footer", page: 0, xMin: 10, yMin: 96, xMax: 40, yMax: 99}, // footer band → dropped
		},
	}
	got := normalize(pl)
	if len(got) != 2 {
		t.Fatalf("kept %d words want 2: %+v", len(got), got)
	}
	if got[0].text != "hello" || got[1].text != "first" {
		t.Fatalf("normalised tokens = %q,%q", got[0].text, got[1].text)
	}
	// hpos/vpos are page-relative: centre (20,15) on a 100x100 page.
	if math.Abs(got[0].hpos-0.20) > 1e-9 || math.Abs(got[0].vpos-0.15) > 1e-9 {
		t.Fatalf("hello pos = (%v,%v) want (0.20,0.15)", got[0].hpos, got[0].vpos)
	}
}

func TestNormalizeSkipsMissingAndZeroPageDims(t *testing.T) {
	pl := pdfLayout{
		pages: []pageDim{{w: 0, h: 100}, {w: 100, h: 100}},
		words: []wordBox{
			{text: "onbadpage", page: 0, xMin: 10, yMin: 10, xMax: 30, yMax: 20}, // zero width → skip
			{text: "negpage", page: -1, xMin: 10, yMin: 10, xMax: 30, yMax: 20},  // page<0 → skip
			{text: "oob", page: 9, xMin: 10, yMin: 10, xMax: 30, yMax: 20},       // page out of range → skip
			{text: "good", page: 1, xMin: 10, yMin: 10, xMax: 30, yMax: 20},      // kept
		},
	}
	got := normalize(pl)
	if len(got) != 1 || got[0].text != "good" {
		t.Fatalf("normalise dim guards: got %+v", got)
	}
}

func TestFoldToken(t *testing.T) {
	cases := map[string]string{
		"Words.": "words",
		"ﬁrst":   "first",
		"co-op":  "coop",
		"42":     "42",
		"UPPER":  "upper",
		"a,b":    "ab",
	}
	for in, want := range cases {
		if got := foldToken(in); got != want {
			t.Errorf("foldToken(%q)=%q want %q", in, got, want)
		}
	}
}

func nw(text string, hpos, vpos float64, lineStart bool) normWord {
	return normWord{text: text, hpos: hpos, vpos: vpos, lineStart: lineStart}
}

func TestAlignWordsEmpty(t *testing.T) {
	if alignWords(nil, []normWord{nw("a", 0, 0, false)}) != nil {
		t.Fatal("empty ref should align to nil")
	}
	if alignWords([]normWord{nw("a", 0, 0, false)}, nil) != nil {
		t.Fatal("empty got should align to nil")
	}
}

func TestAlignWordsLCS(t *testing.T) {
	ref := []normWord{nw("a", 0, 0, false), nw("b", 0, 0, false), nw("c", 0, 0, false), nw("d", 0, 0, false)}
	got := []normWord{nw("a", 0, 0, false), nw("x", 0, 0, false), nw("c", 0, 0, false), nw("d", 0, 0, false)}
	pairs := alignWords(ref, got)
	want := []alignPair{{0, 0}, {2, 2}, {3, 3}}
	if len(pairs) != len(want) {
		t.Fatalf("pairs=%v want %v", pairs, want)
	}
	for i := range want {
		if pairs[i] != want[i] {
			t.Fatalf("pair %d = %v want %v", i, pairs[i], want[i])
		}
	}
}

func TestLCSPairsBothBacktrackBranches(t *testing.T) {
	// ref=[a,b] got=[b]: at (0,0) mismatch, dp favours advancing i (skip a).
	if p := lcsPairs([]string{"a", "b"}, []string{"b"}); len(p) != 1 || p[0] != (alignPair{1, 0}) {
		t.Fatalf("advance-i branch: %v", p)
	}
	// ref=[b] got=[a,b]: at (0,0) mismatch, dp favours advancing j (skip a).
	if p := lcsPairs([]string{"b"}, []string{"a", "b"}); len(p) != 1 || p[0] != (alignPair{0, 1}) {
		t.Fatalf("advance-j branch: %v", p)
	}
}

func TestGreedyPairsAndLookAhead(t *testing.T) {
	// Deletion on the got side (c missing 'b'): advance i.
	if p := greedyPairs([]string{"a", "b", "c"}, []string{"a", "c"}); len(p) != 2 ||
		p[0] != (alignPair{0, 0}) || p[1] != (alignPair{2, 1}) {
		t.Fatalf("deletion case: %v", p)
	}
	// Insertion on the got side (extra 'b'): advance j.
	if p := greedyPairs([]string{"a", "c"}, []string{"a", "b", "c"}); len(p) != 2 ||
		p[0] != (alignPair{0, 0}) || p[1] != (alignPair{1, 2}) {
		t.Fatalf("insertion case: %v", p)
	}
	// lookAhead: found within window, and not found (beyond / absent) → window+1.
	if got := lookAhead("z", []string{"a", "z", "b"}, 1, 8); got != 0 {
		t.Fatalf("lookAhead found=%d want 0", got)
	}
	if got := lookAhead("q", []string{"a", "b"}, 1, 8); got != 9 {
		t.Fatalf("lookAhead absent=%d want 9", got)
	}
}

func TestAlignWordsGreedyFallback(t *testing.T) {
	// Streams whose product exceeds lcsCap take the greedy path. 2001*2001 > 4e6.
	n := 2001
	ref := make([]normWord, n)
	got := make([]normWord, n)
	for i := 0; i < n; i++ {
		ref[i] = nw("w", 0, 0, false)
		got[i] = nw("w", 0, 0, false)
	}
	pairs := alignWords(ref, got)
	if len(pairs) != n {
		t.Fatalf("greedy fallback matched %d want %d", len(pairs), n)
	}
}

func TestTexts(t *testing.T) {
	got := texts([]normWord{nw("a", 0, 0, false), nw("b", 0, 0, false)})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("texts=%v", got)
	}
}

func TestDisplacements(t *testing.T) {
	ref := []normWord{nw("a", 0.2, 1.0, false), nw("b", 0.5, 2.0, false)}
	got := []normWord{nw("a", 0.5, 1.0, false), nw("b", 0.5, 2.0, false)}
	pairs := []alignPair{{0, 0}, {1, 1}}
	d := displacements(pairs, ref, got)
	if math.Abs(d[0]-0.3) > 1e-9 { // horizontal-only 0.3
		t.Fatalf("d0=%v want 0.3", d[0])
	}
	if d[1] != 0 {
		t.Fatalf("d1=%v want 0", d[1])
	}
}

func TestMedianP95(t *testing.T) {
	if m, p := medianP95(nil); m != 0 || p != 0 {
		t.Fatalf("empty=%v/%v want 0/0", m, p)
	}
	if m, p := medianP95([]float64{5}); m != 5 || p != 5 {
		t.Fatalf("single=%v/%v want 5/5", m, p)
	}
	// 20 values 1..20: median is the upper-middle (index 10 → 11), p95 is rank
	// ceil(0.95*20)-1 = 18 → value 19.
	vals := make([]float64, 20)
	for i := range vals {
		vals[i] = float64(i + 1)
	}
	m, p := medianP95(vals)
	if m != 11 || p != 19 {
		t.Fatalf("median/p95 = %v/%v want 11/19", m, p)
	}
}

func TestLineBreakAgreement(t *testing.T) {
	if a := lineBreakAgreement(nil, nil, nil); a != 0 {
		t.Fatalf("no pairs → %v want 0", a)
	}
	ref := []normWord{nw("a", 0, 0, true), nw("b", 0, 0, true), nw("c", 0, 0, false)}
	got := []normWord{nw("a", 0, 0, true), nw("b", 0, 0, false), nw("c", 0, 0, false)}
	// pair 0 agrees (true/true), pair 1 disagrees (true/false), pair 2 agrees.
	a := lineBreakAgreement([]alignPair{{0, 0}, {1, 1}, {2, 2}}, ref, got)
	if math.Abs(a-2.0/3.0) > 1e-9 {
		t.Fatalf("agreement=%v want 2/3", a)
	}
}

func TestScoreLayoutNormal(t *testing.T) {
	ref := []normWord{nw("alpha", 0.1, 0.1, true), nw("beta", 0.1, 0.2, true), nw("gamma", 0.1, 0.3, true)}
	got := []normWord{nw("alpha", 0.1, 0.1, true), nw("beta", 0.1, 0.2, true), nw("gamma", 0.1, 0.3, true)}
	s := scoreLayout(ref, got, 2, 2)
	if s.matched != 3 {
		t.Fatalf("matched=%d want 3", s.matched)
	}
	if s.medianDisp != 0 || s.p95Disp != 0 {
		t.Fatalf("identical layout should have zero displacement, got %v/%v", s.medianDisp, s.p95Disp)
	}
	if s.lineAgree != 1 {
		t.Fatalf("lineAgree=%v want 1", s.lineAgree)
	}
	if s.divergence != 0 { // p95(0) + pageDelta(0) + (1-1)=0
		t.Fatalf("divergence=%v want 0", s.divergence)
	}
}

func TestScoreLayoutPageAndNoMatchPenalty(t *testing.T) {
	ref := []normWord{nw("alpha", 0.1, 0.1, true)}
	got := []normWord{nw("omega", 0.9, 0.9, false)} // no shared token
	s := scoreLayout(ref, got, 5, 2)
	if s.matched != 0 {
		t.Fatalf("matched=%d want 0", s.matched)
	}
	// divergence = p95(0) + |5-2|(3) + (1-0)(1) + no-match(1) = 5
	if s.divergence != 5 {
		t.Fatalf("divergence=%v want 5", s.divergence)
	}
}

// layoutFakePipe builds a layoutPipeline whose engines write the given bbox XML
// as the PDF's bytes and whose extractor parses those bytes. A nil body means the
// engine fails (writes nothing).
func layoutFakePipe(refBody, gotBody *string) layoutPipeline {
	writeOrSkip := func(body *string) func(_, outPDF string) error {
		return func(_, outPDF string) error {
			if body == nil {
				return nil
			}
			return os.WriteFile(outPDF, bboxDoc(*body), 0o644)
		}
	}
	return layoutPipeline{
		compileRef: writeOrSkip(refBody),
		compileGot: writeOrSkip(gotBody),
		extract: func(pdf string) pdfLayout {
			b, err := os.ReadFile(pdf)
			if err != nil {
				return pdfLayout{}
			}
			return parseBBox(b)
		},
	}
}

func layoutPaper(t *testing.T, class, body string) string {
	t.Helper()
	dir := t.TempDir()
	src := `\documentclass{` + class + `}\begin{document}` + body + `\end{document}`
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

const twoWordPage = `  <page width="600" height="800">
    <word xMin="100" yMin="100" xMax="180" yMax="112">alpha</word>
    <word xMin="200" yMin="100" xMax="280" yMax="112">beta</word>
  </page>`

func TestAnalyzeLayoutOK(t *testing.T) {
	dir := layoutPaper(t, "article", "ignored")
	body := str(twoWordPage)
	res := analyzeLayout(dir, "p1", filepath.Join(t.TempDir(), "w"), layoutFakePipe(body, body))
	if res.status != statusOK {
		t.Fatalf("status=%s want ok", res.status)
	}
	if res.class != "article" {
		t.Fatalf("class=%s want article", res.class)
	}
	if res.score.matched != 2 || res.score.divergence != 0 {
		t.Fatalf("score off: matched=%d divergence=%v", res.score.matched, res.score.divergence)
	}
}

func TestAnalyzeLayoutRefUnavailable(t *testing.T) {
	dir := layoutPaper(t, "article", "body")
	res := analyzeLayout(dir, "p2", filepath.Join(t.TempDir(), "w"), layoutFakePipe(nil, str(twoWordPage)))
	if res.status != statusRefMissing {
		t.Fatalf("status=%s want ref-unavailable", res.status)
	}
}

func TestAnalyzeLayoutGotexFailed(t *testing.T) {
	dir := layoutPaper(t, "article", "body")
	res := analyzeLayout(dir, "p3", filepath.Join(t.TempDir(), "w"), layoutFakePipe(str(twoWordPage), nil))
	if res.status != statusGotexFailed {
		t.Fatalf("status=%s want gotex-failed", res.status)
	}
	if res.score.refPages != 1 {
		t.Fatalf("gotex-failed should record refPages, got %d", res.score.refPages)
	}
}

func TestAnalyzeLayoutNoToplevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frag.tex"), []byte("no document env"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := analyzeLayout(dir, "p4", filepath.Join(t.TempDir(), "w"), layoutFakePipe(str(twoWordPage), str(twoWordPage)))
	if res.status != statusNoToplevel {
		t.Fatalf("status=%s want no-toplevel", res.status)
	}
}

func TestAnalyzeLayoutMkdirError(t *testing.T) {
	dir := layoutPaper(t, "article", "body")
	// Make the workDir un-creatable by rooting it under a regular file.
	blocker := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	workDir := filepath.Join(blocker, "sub")
	res := analyzeLayout(dir, "p5", workDir, layoutFakePipe(str(twoWordPage), str(twoWordPage)))
	if res.status != statusNoToplevel {
		t.Fatalf("status=%s want no-toplevel (mkdir failure)", res.status)
	}
}

func TestReportLayoutRanksWorstFirst(t *testing.T) {
	results := []layoutResult{
		{id: "mild", class: "article", status: statusOK, score: layoutScore{divergence: 0.3, refPages: 5, gotPages: 5, matched: 100, lineAgree: 0.99}},
		{id: "severe", class: "revtex", status: statusOK, score: layoutScore{divergence: 8.0, refPages: 10, gotPages: 4, matched: 200, lineAgree: 0.7}},
		{id: "dead", class: "book", status: statusGotexFailed, score: layoutScore{divergence: 1, refPages: 3}},
		{id: "skip", class: "?", status: statusRefMissing},
	}
	var buf bytes.Buffer
	reportLayout(results, &buf)
	out := buf.String()
	iSevere := strings.Index(out, "severe")
	iDead := strings.Index(out, "dead")
	iMild := strings.Index(out, "mild")
	if !(iSevere < iDead && iDead < iMild) {
		t.Fatalf("not ranked worst-first (desc divergence):\n%s", out)
	}
	if !strings.Contains(out, "MISSING PAGES (-6)") {
		t.Fatalf("expected page-loss note:\n%s", out)
	}
	if !strings.Contains(out, "gotex-failed") {
		t.Fatalf("expected gotex-failed note:\n%s", out)
	}
	if !strings.Contains(out, "not scored") || !strings.Contains(out, "skip") {
		t.Fatalf("unscored section missing:\n%s", out)
	}
	if !strings.Contains(out, "mean divergence") {
		t.Fatalf("mean line missing:\n%s", out)
	}
}

func TestReportLayoutTieBreakByID(t *testing.T) {
	// Two papers with identical divergence are ordered by id (stable, so the
	// ranking is reproducible when scores coincide).
	results := []layoutResult{
		{id: "zebra", class: "article", status: statusOK, score: layoutScore{divergence: 2.0, refPages: 1, gotPages: 1, matched: 10}},
		{id: "alpha", class: "article", status: statusOK, score: layoutScore{divergence: 2.0, refPages: 1, gotPages: 1, matched: 10}},
	}
	var buf bytes.Buffer
	reportLayout(results, &buf)
	out := buf.String()
	if strings.Index(out, "alpha") > strings.Index(out, "zebra") {
		t.Fatalf("equal divergence not tie-broken by id:\n%s", out)
	}
}

func TestReportLayoutEmpty(t *testing.T) {
	var buf bytes.Buffer
	reportLayout(nil, &buf)
	if !strings.Contains(buf.String(), "0 paper(s) scored") {
		t.Fatalf("unexpected empty report: %s", buf.String())
	}
}

func TestLayoutNote(t *testing.T) {
	cases := []struct {
		r    layoutResult
		want string
	}{
		{layoutResult{status: statusGotexFailed}, "gotex-failed"},
		{layoutResult{status: statusOK, score: layoutScore{refPages: 3, gotPages: 5}}, "PAGE OVERFLOW (+2)"},
		{layoutResult{status: statusOK, score: layoutScore{refPages: 5, gotPages: 3}}, "MISSING PAGES (-2)"},
		{layoutResult{status: statusOK, score: layoutScore{refPages: 3, gotPages: 3, matched: 0}}, "NO SHARED LAYOUT"},
		{layoutResult{status: statusOK, score: layoutScore{refPages: 3, gotPages: 3, matched: 50}}, "ok"},
	}
	for _, c := range cases {
		if got := layoutNote(c.r); got != c.want {
			t.Errorf("layoutNote=%q want %q", got, c.want)
		}
	}
}

func TestExtractLayout(t *testing.T) {
	work := t.TempDir()
	// Missing file → empty.
	if pl := extractLayout(nil, time.Second, filepath.Join(work, "absent.pdf")); len(pl.words) != 0 {
		t.Fatalf("missing file: got %+v", pl)
	}
	// Empty file → empty.
	empty := filepath.Join(work, "empty.pdf")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if pl := extractLayout(nil, time.Second, empty); len(pl.words) != 0 {
		t.Fatalf("empty file: got %+v", pl)
	}
	// Non-empty file, runner returns bbox → parsed.
	full := filepath.Join(work, "full.pdf")
	if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := func(_ time.Duration, _ string) ([]byte, error) { return bboxDoc(twoWordPage), nil }
	if pl := extractLayout(ok, time.Second, full); len(pl.words) != 2 {
		t.Fatalf("ok runner: got %d words", len(pl.words))
	}
	// Runner error → empty.
	bad := func(_ time.Duration, _ string) ([]byte, error) { return nil, os.ErrClosed }
	if pl := extractLayout(bad, time.Second, full); len(pl.words) != 0 {
		t.Fatalf("error runner: got %+v", pl)
	}
}

func TestRealLayoutPipelineConstructs(t *testing.T) {
	p := realLayoutPipeline("/nonexistent/gotex", 100*time.Millisecond)
	if pl := p.extract(filepath.Join(t.TempDir(), "absent.pdf")); len(pl.words) != 0 {
		t.Fatalf("extract of missing pdf: %+v", pl)
	}
	if err := p.compileGot("/x/paper.tex", filepath.Join(t.TempDir(), "o.pdf")); err == nil {
		t.Fatal("expected compileGot with a missing gotex binary to error")
	}
	// Exercise the compileRef closure too: a nonexistent .tex with a very short
	// timeout errors quickly, covering the reference-compile wiring without a
	// successful (network-bound) tectonic run.
	if err := p.compileRef(filepath.Join(t.TempDir(), "absent.tex"), filepath.Join(t.TempDir(), "ref.pdf")); err == nil {
		t.Fatal("expected compileRef on an absent .tex to error")
	}
}

func TestBBoxRunner(t *testing.T) {
	// Fallback branch: a tool that does not exist selects the pkgx path; running
	// it returns an error (pkgx may be absent), which is all we assert.
	r := bboxRunner("definitely-not-a-real-binary-xyz")
	_, _ = r(10*time.Millisecond, "/tmp/x.pdf")
	// Direct branch: a tool on PATH ("go") is invoked directly; `go -bbox …`
	// errors, but the direct selection path is exercised without panicking.
	direct := bboxRunner("go")
	_, _ = direct(10*time.Second, filepath.Join(t.TempDir(), "x.pdf"))
}

func TestAttrMap(t *testing.T) {
	a := attrMap([]byte(` width="12.5" height="800" bad="nope"`))
	if a["width"] != 12.5 || a["height"] != 800 {
		t.Fatalf("numeric attrs = %v", a)
	}
	if _, ok := a["bad"]; ok {
		t.Fatalf("unparseable attr should be omitted: %v", a)
	}
	if _, ok := a["missing"]; ok {
		t.Fatalf("absent attr should be omitted: %v", a)
	}
}
