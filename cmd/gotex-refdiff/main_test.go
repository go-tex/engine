// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePipe builds a pipeline whose engines just write the given text as the
// "PDF" content, and whose extractor returns that content verbatim. A nil text
// means the engine fails (writes nothing) — the empty-PDF case.
func fakePipe(refText, gotText *string) pipeline {
	writeOrSkip := func(text *string) func(_, outPDF string) error {
		return func(_, outPDF string) error {
			if text == nil {
				return nil // simulate a failed compile: no file
			}
			return os.WriteFile(outPDF, []byte(*text), 0o644)
		}
	}
	return pipeline{
		compileRef: writeOrSkip(refText),
		compileGot: writeOrSkip(gotText),
		extract: func(pdf string) extraction {
			b, err := os.ReadFile(pdf)
			if err != nil {
				return extraction{} // absent PDF: a genuine empty result
			}
			return extraction{text: string(b), nonTrivial: len(b) >= nonTrivialPDFBytes}
		},
	}
}

func str(s string) *string { return &s }

func makePaper(t *testing.T, class, body string) string {
	t.Helper()
	dir := t.TempDir()
	src := `\documentclass{` + class + `}\begin{document}` + body + `\end{document}`
	if err := os.WriteFile(filepath.Join(dir, "main.tex"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAnalyzePaperOK(t *testing.T) {
	dir := makePaper(t, "article", "ignored")
	pipe := fakePipe(str("alpha beta gamma delta"), str("alpha beta zeta"))
	res := analyzePaper(dir, "p1", filepath.Join(t.TempDir(), "w"), pipe)
	if res.status != statusOK {
		t.Fatalf("status=%s want ok", res.status)
	}
	if res.class != "article" {
		t.Fatalf("class=%s want article", res.class)
	}
	if res.refWords != 4 || res.common != 2 || res.ratio != 0.5 {
		t.Fatalf("scoring off: ref=%d common=%d ratio=%v", res.refWords, res.common, res.ratio)
	}
}

func TestAnalyzePaperRefUnavailable(t *testing.T) {
	dir := makePaper(t, "article", "body")
	pipe := fakePipe(nil, str("gotex produced text")) // reference failed
	res := analyzePaper(dir, "p2", filepath.Join(t.TempDir(), "w"), pipe)
	if res.status != statusRefMissing {
		t.Fatalf("status=%s want ref-unavailable", res.status)
	}
}

func TestAnalyzePaperGotexFailed(t *testing.T) {
	dir := makePaper(t, "article", "body")
	pipe := fakePipe(str("real reference content here"), nil) // gotex failed
	res := analyzePaper(dir, "p3", filepath.Join(t.TempDir(), "w"), pipe)
	if res.status != statusGotexFailed {
		t.Fatalf("status=%s want gotex-failed", res.status)
	}
	if res.refWords == 0 {
		t.Fatal("gotex-failed should still record the reference word count")
	}
}

func TestAnalyzePaperNoToplevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "frag.tex"), []byte("no document env"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := analyzePaper(dir, "p4", filepath.Join(t.TempDir(), "w"), fakePipe(str("x"), str("y")))
	if res.status != statusNoToplevel {
		t.Fatalf("status=%s want no-toplevel", res.status)
	}
}

func TestReportRanksWorstFirst(t *testing.T) {
	results := []paperResult{
		{id: "good", class: "article", status: statusOK, ratio: 0.9, refWords: 100, common: 90},
		{id: "bad", class: "revtex", status: statusOK, ratio: 0.2, refWords: 100, common: 20},
		{id: "zero", class: "book", status: statusGotexFailed, ratio: 0, refWords: 50},
		{id: "skip", class: "?", status: statusRefMissing},
	}
	var buf bytes.Buffer
	report(results, &buf)
	out := buf.String()

	// Worst (ratio 0) must appear before the mid and the best in the table.
	iZero := strings.Index(out, "zero")
	iBad := strings.Index(out, "bad")
	iGood := strings.Index(out, "good")
	if !(iZero < iBad && iBad < iGood) {
		t.Fatalf("not ranked worst-first:\n%s", out)
	}
	// The unscored paper is listed in its own section, after the mean line.
	if !strings.Contains(out, "not scored") || !strings.Contains(out, "skip") {
		t.Fatalf("unscored section missing:\n%s", out)
	}
	if !strings.Contains(out, "mean recall") {
		t.Fatalf("mean line missing:\n%s", out)
	}
}

func TestReportEmpty(t *testing.T) {
	var buf bytes.Buffer
	report(nil, &buf)
	if !strings.Contains(buf.String(), "0 paper(s) scored") {
		t.Fatalf("unexpected empty report: %s", buf.String())
	}
}

func TestSamplePapers(t *testing.T) {
	dirs := []string{"a", "b", "c", "d", "e"}
	// Deterministic for a fixed seed.
	s1 := samplePapers(dirs, 3, 7)
	s2 := samplePapers(dirs, 3, 7)
	if len(s1) != 3 || !equal(s1, s2) {
		t.Fatalf("sample not deterministic: %v vs %v", s1, s2)
	}
	// Different seed generally reorders (not a hard guarantee, but these differ).
	if equal(s1, samplePapers(dirs, 3, 8)) {
		t.Fatalf("seed had no effect: %v", s1)
	}
	// n>=len returns all.
	if len(samplePapers(dirs, 99, 1)) != len(dirs) {
		t.Fatal("n>=len should return all papers")
	}
	// n<=0 returns all.
	if len(samplePapers(dirs, 0, 1)) != len(dirs) {
		t.Fatal("n<=0 should return all papers")
	}
}

func TestDiscoverPapers(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"2101.1", "2102.2"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := discoverPapers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 paper dirs, got %v", got)
	}
	if _, err := discoverPapers(filepath.Join(root, "missing")); err == nil {
		t.Fatal("expected error for missing corpus")
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in, want string
		n        int
	}{
		{"article", "article", 16},
		{"averylongclassname", "averylongclassn…", 16},
		{"ab", "a", 1},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q,%d)=%q want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestReplaceExt(t *testing.T) {
	if got := replaceExt("paper.tex", ".pdf"); got != "paper.pdf" {
		t.Fatalf("replaceExt=%q", got)
	}
}

func TestToolRunnerFallback(t *testing.T) {
	// A tool that certainly does not exist forces the pkgx branch to be chosen;
	// running it returns an error (pkgx may be absent in the test env), which is
	// all we assert — the point is the selection path is exercised.
	r := toolRunner("definitely-not-a-real-binary-xyz")
	_ = r(10*time.Millisecond, "arg")
	// A tool that does exist (go, always on PATH for the test) takes the direct
	// branch and runs successfully with a no-op invocation.
	direct := toolRunner("go")
	if err := direct(30*time.Second, "version"); err != nil {
		t.Fatalf("direct runner for 'go version': %v", err)
	}
}

func TestCaptureRunnerDirect(t *testing.T) {
	// captureRunner over "cat" (present on PATH) returns the file's bytes; cat
	// reads the path and ignores the trailing "-" argument-as-stdin-marker in a
	// way close enough to pdftotext's contract for this selection-path test.
	if _, err := os.Stat("/bin/cat"); err != nil {
		t.Skip("cat not available")
	}
	run := captureRunner("cat")
	f := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(10*time.Second, f)
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if !strings.Contains(out, "hello") {
		t.Fatalf("captured %q, want to contain hello", out)
	}
}

func TestRunFlagErrors(t *testing.T) {
	var out, errb bytes.Buffer
	// Missing -corpus.
	if code := run(nil, &out, &errb); code != 2 {
		t.Fatalf("missing corpus: code=%d want 2", code)
	}
	// Bad flag.
	if code := run([]string{"-nope"}, &out, &errb); code != 2 {
		t.Fatalf("bad flag: code=%d want 2", code)
	}
	// Missing corpus directory.
	if code := run([]string{"-corpus", filepath.Join(t.TempDir(), "absent")}, &out, &errb); code != 1 {
		t.Fatalf("absent corpus: code=%d want 1", code)
	}
	// Empty corpus directory.
	if code := run([]string{"-corpus", t.TempDir()}, &out, &errb); code != 1 {
		t.Fatalf("empty corpus: code=%d want 1", code)
	}
}

func TestRunTectonicRenames(t *testing.T) {
	work := t.TempDir()
	outPDF := filepath.Join(work, "ref.pdf")
	// Fake runner: honour tectonic's contract by writing <base>.pdf into -o dir.
	run := func(_ time.Duration, args ...string) error {
		var outDir, tex string
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "-o" {
				outDir = args[i+1]
			}
		}
		tex = args[len(args)-1]
		produced := filepath.Join(outDir, replaceExt(filepath.Base(tex), ".pdf"))
		return os.WriteFile(produced, []byte("PDF"), 0o644)
	}
	if err := runTectonic(run, time.Second, filepath.Join(work, "paper.tex"), outPDF); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outPDF); err != nil {
		t.Fatalf("expected renamed %s: %v", outPDF, err)
	}
}

func TestRunTectonicErrorPropagates(t *testing.T) {
	run := func(_ time.Duration, _ ...string) error { return os.ErrPermission }
	if err := runTectonic(run, time.Second, "/x/paper.tex", "/x/ref.pdf"); err != os.ErrPermission {
		t.Fatalf("expected the runner error to propagate, got %v", err)
	}
}

func TestExtractText(t *testing.T) {
	work := t.TempDir()
	// Missing file → zero extraction (genuine empty, not a tool error).
	if got := extractText(nil, time.Second, filepath.Join(work, "absent.pdf")); got != (extraction{}) {
		t.Fatalf("missing file: got %+v", got)
	}
	// Empty file → zero extraction.
	empty := filepath.Join(work, "empty.pdf")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := extractText(nil, time.Second, empty); got != (extraction{}) {
		t.Fatalf("empty file: got %+v", got)
	}
	// Small (sub-threshold) file, runner returns text: text kept, nonTrivial false.
	small := filepath.Join(work, "small.pdf")
	if err := os.WriteFile(small, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	ok := func(_ time.Duration, pdf string) (string, error) { return "the words", nil }
	if got := extractText(ok, time.Second, small); got.text != "the words" || got.nonTrivial || got.toolErr {
		t.Fatalf("small ok runner: got %+v", got)
	}
	// Non-trivial file, runner returns text: nonTrivial true.
	big := filepath.Join(work, "big.pdf")
	if err := os.WriteFile(big, make([]byte, nonTrivialPDFBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := extractText(ok, time.Second, big); !got.nonTrivial || got.toolErr || got.text != "the words" {
		t.Fatalf("big ok runner: got %+v", got)
	}
	// Runner error on a present, non-trivial file → toolErr, no text.
	bad := func(_ time.Duration, pdf string) (string, error) { return "", os.ErrClosed }
	if got := extractText(bad, time.Second, big); !got.toolErr || got.text != "" || !got.nonTrivial {
		t.Fatalf("error runner: got %+v", got)
	}
}

func TestRealPipelineConstructs(t *testing.T) {
	// Construct the real pipeline and exercise the extract closure on a missing
	// file (no external tool needed). This covers the wiring in realPipeline.
	p := realPipeline("/nonexistent/gotex", 100*time.Millisecond)
	if got := p.extract(filepath.Join(t.TempDir(), "absent.pdf")); got != (extraction{}) {
		t.Fatalf("extract of missing pdf: %+v", got)
	}
	// compileGot over a missing binary must error (not panic).
	if err := p.compileGot("/x/paper.tex", filepath.Join(t.TempDir(), "o.pdf")); err == nil {
		t.Fatal("expected compileGot with a missing gotex binary to error")
	}
}

func TestBuildGotex(t *testing.T) {
	// Build the real gotex from the module root so the ./cmd/gotex path resolves.
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	bin := filepath.Join(t.TempDir(), "gotex")
	if out, err := buildGotex(bin); err != nil {
		t.Fatalf("buildGotex: %v\n%s", err, out)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("gotex binary not produced: %v", err)
	}
}

// guardPipe builds a pipeline whose compiles always "succeed" (a dummy file is
// written so a real workDir exists) and whose extractor returns exactly the given
// extraction for the reference and gotex PDFs in turn — so the empty-extraction
// guard can be driven directly, without any external tool.
func guardPipe(ref, got extraction) pipeline {
	write := func(_, outPDF string) error { return os.WriteFile(outPDF, []byte("PDF"), 0o644) }
	first := true
	return pipeline{
		compileRef: write,
		compileGot: write,
		extract: func(string) extraction {
			if first {
				first = false
				return ref
			}
			return got
		},
	}
}

func TestAnalyzePaperExtractErrorReferenceToolErr(t *testing.T) {
	// A real reference PDF the extractor could not read (a tool error) must NOT be
	// reported as a missing ground truth — it is an extract-error, excluded, with
	// a diagnostic naming the reference.
	dir := makePaper(t, "article", "body")
	res := analyzePaper(dir, "p", filepath.Join(t.TempDir(), "w"), guardPipe(extraction{toolErr: true, nonTrivial: true}, extraction{}))
	if res.status != statusExtractError {
		t.Fatalf("status=%s want extract-error", res.status)
	}
	if !strings.Contains(res.diag, "reference") || res.diag == "" {
		t.Fatalf("diag=%q want a reference tool-error message", res.diag)
	}
}

func TestAnalyzePaperExtractErrorReferenceEmptyNonTrivial(t *testing.T) {
	// A non-trivial reference PDF that extracted to nothing (no tool error, but a
	// big PDF and zero words) is a silent extractor failure, not a missing paper.
	dir := makePaper(t, "article", "body")
	res := analyzePaper(dir, "p", filepath.Join(t.TempDir(), "w"), guardPipe(extraction{nonTrivial: true}, extraction{}))
	if res.status != statusExtractError {
		t.Fatalf("status=%s want extract-error", res.status)
	}
	if !strings.Contains(res.diag, "non-trivial") {
		t.Fatalf("diag=%q want a non-trivial-PDF message", res.diag)
	}
}

func TestAnalyzePaperExtractErrorGotex(t *testing.T) {
	// Reference extracts fine; a non-trivial gotex PDF that extracts to nothing is
	// an extract-error (not a bogus recall-0), and the reference word count is kept.
	dir := makePaper(t, "article", "body")
	ref := extraction{text: "alpha beta gamma delta"}
	res := analyzePaper(dir, "p", filepath.Join(t.TempDir(), "w"), guardPipe(ref, extraction{toolErr: true, nonTrivial: true}))
	if res.status != statusExtractError {
		t.Fatalf("status=%s want extract-error", res.status)
	}
	if !strings.Contains(res.diag, "gotex") {
		t.Fatalf("diag=%q want a gotex message", res.diag)
	}
	if res.refWords != 4 {
		t.Fatalf("refWords=%d want 4 (kept even on a gotex extract-error)", res.refWords)
	}
}

func TestClassifyExtraction(t *testing.T) {
	// Usable text: no status.
	if s, d := classifyExtraction(false, false, 5, "reference", statusRefMissing); s != "" || d != "" {
		t.Fatalf("usable: status=%q diag=%q want empty", s, d)
	}
	// Tool error → extract-error with a tool diagnostic.
	if s, d := classifyExtraction(true, false, 0, "reference", statusRefMissing); s != statusExtractError || !strings.Contains(d, "tool error") {
		t.Fatalf("toolErr: status=%q diag=%q", s, d)
	}
	// Non-trivial empty PDF → extract-error with a non-trivial diagnostic.
	if s, d := classifyExtraction(false, true, 0, "gotex", statusGotexFailed); s != statusExtractError || !strings.Contains(d, "non-trivial") {
		t.Fatalf("nonTrivial: status=%q diag=%q", s, d)
	}
	// Genuinely empty tiny PDF → the caller's genuineEmpty status, no diagnostic.
	if s, d := classifyExtraction(false, false, 0, "gotex", statusGotexFailed); s != statusGotexFailed || d != "" {
		t.Fatalf("genuine: status=%q diag=%q", s, d)
	}
}

func TestPkgxArgs(t *testing.T) {
	// A poppler tool is pinned to its project so pkgx does not hit MultipleProjects.
	got := pkgxArgs("pdftotext", "file.pdf", "-")
	want := []string{"+" + popplerProject, "pdftotext", "file.pdf", "-"}
	if !equal(got, want) {
		t.Fatalf("pkgxArgs(pdftotext)=%v want %v", got, want)
	}
	// pdftoppm is pinned the same way.
	if got := pkgxArgs("pdftoppm", "-png", "f.pdf"); got[0] != "+"+popplerProject || got[1] != "pdftoppm" {
		t.Fatalf("pkgxArgs(pdftoppm)=%v not pinned", got)
	}
	// A non-poppler tool is passed through unpinned.
	if got := pkgxArgs("tectonic", "-o", "d"); !equal(got, []string{"tectonic", "-o", "d"}) {
		t.Fatalf("pkgxArgs(tectonic)=%v want unpinned", got)
	}
}

func TestReportShowsExtractErrorDiag(t *testing.T) {
	results := []paperResult{
		{id: "good", class: "article", status: statusOK, ratio: 0.9, refWords: 100, common: 90},
		{id: "toolbroke", class: "revtex", status: statusExtractError, diag: "reference: a non-trivial PDF extracted to zero content words"},
	}
	var buf bytes.Buffer
	report(results, &buf)
	out := buf.String()
	if !strings.Contains(out, "extract-error") || !strings.Contains(out, "zero content words") {
		t.Fatalf("extract-error diag missing from report:\n%s", out)
	}
	// An extract-error paper is excluded from the score (mean over the one ok paper).
	if !strings.Contains(out, "1 paper(s) scored") {
		t.Fatalf("extract-error should be excluded from the score:\n%s", out)
	}
}

func TestDiagSuffix(t *testing.T) {
	if diagSuffix("") != "" {
		t.Fatalf("empty diag should render nothing")
	}
	if got := diagSuffix("boom"); got != "  — boom" {
		t.Fatalf("diagSuffix=%q", got)
	}
}

func TestWarnExtractErrorsAndDiagnostics(t *testing.T) {
	// No extract-errors → nothing printed.
	var buf bytes.Buffer
	warnExtractErrors(&buf, paperDiagnostics([]paperResult{{id: "a", status: statusOK}}))
	if buf.String() != "" {
		t.Fatalf("no extract-error should print nothing, got %q", buf.String())
	}
	// Recall-mode diagnostics gather every extract-error, and the warning names them.
	results := []paperResult{
		{id: "a", status: statusOK},
		{id: "bad1", status: statusExtractError, diag: "reference: tool error"},
		{id: "bad2", status: statusExtractError, diag: "gotex: non-trivial"},
	}
	diags := paperDiagnostics(results)
	if len(diags) != 2 {
		t.Fatalf("paperDiagnostics=%v want 2", diags)
	}
	buf.Reset()
	warnExtractErrors(&buf, diags)
	out := buf.String()
	if !strings.Contains(out, "WARNING") || !strings.Contains(out, "2 paper(s)") ||
		!strings.Contains(out, "bad1") || !strings.Contains(out, "bad2") {
		t.Fatalf("warning missing content:\n%s", out)
	}
	// Layout-mode diagnostics behave identically.
	ld := layoutDiagnostics([]layoutResult{
		{id: "lok", status: statusOK},
		{id: "lbad", status: statusExtractError, diag: "reference: tool error"},
	})
	if len(ld) != 1 || !strings.Contains(ld[0], "lbad") {
		t.Fatalf("layoutDiagnostics=%v", ld)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
