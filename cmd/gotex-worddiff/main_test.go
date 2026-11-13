// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakePDF writes a non-empty file to stand in for a PDF; the extractor is
// injected, so the bytes are never parsed.
func fakePDF(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("%PDF-1.7 not really"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func textOf(m map[string]string) extractor {
	return func(_ time.Duration, pdf string) (string, error) {
		return m[filepath.Base(pdf)], nil
	}
}

// ⭐ THE CASE THIS TOOL WAS BUILT FOR, from #772: a paper whose word-recall
// score moved by −0.1 without saying which words. These are real pairs from
// that paper's two builds — a word split across a hyphen on one side and whole
// on the other, and one fused token — the shapes a line-breaking change makes.
//
// ⚠ THE FUSED TOKEN HERE IS A FIXTURE, NOT A FINDING. A scratch probe once
// reported fused affiliation words in that paper and it was its extractor's
// doing, not the engine's (see the file comment). The tool must report such a
// token when it IS there; whether it is there is the extractor's business, and
// this package has exactly one.
func TestItNamesTheWordsThatRanTogether(t *testing.T) {
	a, b := fakePDF(t, "base.pdf"), fakePDF(t, "head.pdf")
	ex := textOf(map[string]string{
		"base.pdf": "QiluNormalUniversity Jinan250200 char- acterized",
		"head.pdf": "Qilu Normal University Jinan 250200 characterized",
	})
	var out, errb bytes.Buffer
	if rc := compare(ex, time.Second, a, b, false, &out, &errb); rc != 1 {
		t.Fatalf("exit %d, want 1 (they differ)", rc)
	}
	got := out.String()
	for _, w := range []string{"QiluNormalUniversity", "Jinan250200", "char-", "acterized"} {
		if !strings.Contains(got, w) {
			t.Errorf("%q is missing from the report:\n%s", w, got)
		}
	}
	for _, w := range []string{"Qilu", "Normal", "University", "characterized"} {
		if !strings.Contains(got, w) {
			t.Errorf("%q is missing from the report:\n%s", w, got)
		}
	}
	// ⛔ AND A WORD BOTH SIDES HOLD IS NOT REPORTED. `acterized` is base's alone,
	// but `characterized` must appear only under head — if the diff listed words
	// both files share it would bury the four that matter.
	if n := strings.Count(got, "characterized"); n != 1 {
		t.Errorf("`characterized` appears %d times, want 1 (head's side only):\n%s", n, got)
	}
}

// ⛔ A WORD BOTH PDFS HOLD IS NOT A DIFFERENCE, and two identical files exit 0.
func TestIdenticalTextIsNoDifference(t *testing.T) {
	a, b := fakePDF(t, "a.pdf"), fakePDF(t, "b.pdf")
	ex := textOf(map[string]string{
		"a.pdf": "the measure is wrong on every line",
		"b.pdf": "every line is wrong on the measure",
	})
	var out, errb bytes.Buffer
	if rc := compare(ex, time.Second, a, b, false, &out, &errb); rc != 0 {
		t.Fatalf("exit %d, want 0 — the same words in a different order are the "+
			"same words:\n%s", rc, out.String())
	}
	if !strings.Contains(out.String(), "(0)") {
		t.Errorf("want both sides reported as 0 differences:\n%s", out.String())
	}
}

// ⛔⛔ IT IS A MULTISET, NOT A SET. A word the page holds three times and the
// other twice is a difference; collapsing duplicates would hide exactly what a
// line-breaking change does to repeated words.
func TestARepeatedWordCountsAsManyTimesAsItAppears(t *testing.T) {
	a, b := fakePDF(t, "a.pdf"), fakePDF(t, "b.pdf")
	ex := textOf(map[string]string{
		"a.pdf": "line line line",
		"b.pdf": "line line",
	})
	var out, errb bytes.Buffer
	if rc := compare(ex, time.Second, a, b, false, &out, &errb); rc != 1 {
		t.Fatalf("exit %d, want 1 — three occurrences are not two", rc)
	}
	if got := out.String(); !strings.Contains(got, "line") {
		t.Errorf("the surplus occurrence is not reported:\n%s", got)
	}
}

// ⛔⛔ AN EXTRACTION THAT FAILED IS AN ERROR, NEVER AN EMPTY PAGE. Reporting
// every word of the other file as a difference would be a maximal, confident
// and meaningless answer — the shape of defect this repository keeps finding.
func TestAFailedExtractionIsNotAnEmptyPage(t *testing.T) {
	a, b := fakePDF(t, "a.pdf"), fakePDF(t, "b.pdf")
	ex := func(_ time.Duration, pdf string) (string, error) {
		if filepath.Base(pdf) == "b.pdf" {
			return "", errors.New("pdftotext: not found")
		}
		return "one two three four", nil
	}
	var out, errb bytes.Buffer
	rc := compare(ex, time.Second, a, b, false, &out, &errb)
	if rc != 2 {
		t.Fatalf("exit %d, want 2 — a broken extractor is an error", rc)
	}
	if strings.Contains(out.String(), "one") {
		t.Errorf("it reported words as differences although one side never read:\n%s",
			out.String())
	}
	if !strings.Contains(errb.String(), "extracting") {
		t.Errorf("the error does not say what failed: %q", errb.String())
	}
}

// ⛔ AN ABSENT OR EMPTY FILE IS AN ERROR TOO, for the same reason.
func TestAnAbsentFileIsAnError(t *testing.T) {
	a := fakePDF(t, "a.pdf")
	ex := textOf(map[string]string{"a.pdf": "words"})
	var out, errb bytes.Buffer
	if rc := compare(ex, time.Second, a, filepath.Join(t.TempDir(), "gone.pdf"), false, &out, &errb); rc != 2 {
		t.Fatalf("exit %d, want 2 for a missing file", rc)
	}

	empty := filepath.Join(t.TempDir(), "empty.pdf")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if rc := compare(ex, time.Second, a, empty, false, &out, &errb); rc != 2 {
		t.Fatalf("exit %d, want 2 for a zero-byte file", rc)
	}
}

// -q prints the counts and not the words.
func TestQuietPrintsCountsOnly(t *testing.T) {
	a, b := fakePDF(t, "a.pdf"), fakePDF(t, "b.pdf")
	ex := textOf(map[string]string{"a.pdf": "alpha", "b.pdf": "beta"})
	var out, errb bytes.Buffer
	compare(ex, time.Second, a, b, true, &out, &errb)
	got := out.String()
	if strings.Contains(got, "alpha") || strings.Contains(got, "beta") {
		t.Errorf("-q printed the words:\n%s", got)
	}
	if !strings.Contains(got, "(1)") {
		t.Errorf("-q did not print the counts:\n%s", got)
	}
}

// Usage: two operands are required, and anything else exits 2.
func TestUsageRequiresTwoPDFs(t *testing.T) {
	var out, errb bytes.Buffer
	if rc := run([]string{"only-one.pdf"}, &out, &errb); rc != 2 {
		t.Fatalf("exit %d, want 2", rc)
	}
	if !strings.Contains(errb.String(), "usage:") {
		t.Errorf("no usage shown: %q", errb.String())
	}
}
