// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package refaudit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// testdata/leaky.pdf is tectonic's output for a 6-line article that sets
// \pdfoutput=1 and loads hyperref; clean.pdf is tectonic's output for a witness
// that does not; junkfirst.pdf has preamble material shipped out as page 1.
func TestProbeLeakyJunkAndClean(t *testing.T) {
	leaky, err := Probe("testdata/leaky.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if !leaky.Leaky() || leaky.FirstLeak != 1 || !strings.Contains(leaky.Unusable(), "leak") {
		t.Errorf("leaky.pdf: %+v unusable=%q", leaky, leaky.Unusable())
	}
	junk, err := Probe("testdata/junkfirst.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if junk.Leaky() || !junk.JunkFirst() || !strings.Contains(junk.Unusable(), "junk first page") {
		t.Errorf("junkfirst.pdf: %+v unusable=%q", junk, junk.Unusable())
	}
	clean, err := Probe("testdata/clean.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if clean.Pages == 0 {
		t.Fatal("clean.pdf read as zero pages: the probe saw nothing")
	}
	if clean.Leaky() || clean.JunkFirst() || clean.Unusable() != "" {
		t.Errorf("clean.pdf judged unusable: %+v %q", clean, clean.Unusable())
	}
}

func TestMatchLeaksIgnoresSpacing(t *testing.T) {
	if got := MatchLeaks("name page.1 XYZ"); len(got) != 1 || got[0] != "namepage." {
		t.Fatalf("got %v", got)
	}
	if got := MatchLeaks("name page.1"); len(got) != 1 {
		t.Fatalf("a no-break space hid the pattern: %v", got)
	}
	if got := MatchLeaks("a page name, and a border attribute"); len(got) != 0 {
		t.Fatalf("prose matched: %v", got)
	}
}

func TestEveryPatternIsFound(t *testing.T) {
	for _, p := range leakPatterns {
		if got := MatchLeaks("xx " + p + " yy"); len(got) == 0 {
			t.Errorf("pattern %q does not match itself", p)
		}
	}
}

func TestJunkFirstNeedsSeveralPages(t *testing.T) {
	if (Verdict{Pages: 1, Page1Chars: 3}).JunkFirst() {
		t.Error("a one-page document is not junk-first")
	}
	if !(Verdict{Pages: 2, Page1Chars: SparseLimit - 1}).JunkFirst() {
		t.Error("a sparse first page of several was not reported")
	}
	if (Verdict{Pages: 2, Page1Chars: SparseLimit}).JunkFirst() {
		t.Error("exactly the limit is not under it")
	}
}

func TestUnreadIsNotClean(t *testing.T) {
	if _, err := Probe("testdata/absent.pdf"); err == nil {
		t.Fatal("a missing file read as a verdict")
	}
}

// minimalPDF writes a hand-built PDF whose page tree holds kids pages, each with
// the given content stream.
func writePDF(t *testing.T, name, content string, pages int, streamDict ...string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	off := map[int]int{}
	obj := func(n int, body string) {
		off[n] = b.Len()
		b.WriteString(strconv.Itoa(n) + " 0 obj\n" + body + "\nendobj\n")
	}
	obj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	kids := ""
	for i := 0; i < pages; i++ {
		kids += strconv.Itoa(4+i) + " 0 R "
	}
	obj(2, "<< /Type /Pages /Kids ["+kids+"] /Count "+strconv.Itoa(pages)+" >>")
	obj(3, "<< /Length "+strconv.Itoa(len(content))+" "+strings.Join(streamDict, " ")+" >>\nstream\n"+content+"\nendstream")
	for i := 0; i < pages; i++ {
		obj(4+i, "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 3 0 R >>")
	}
	n := 4 + pages
	xref := b.Len()
	b.WriteString("xref\n0 " + strconv.Itoa(n) + "\n0000000000 65535 f \n")
	for i := 1; i < n; i++ {
		b.WriteString(fmt.Sprintf("%010d 00000 n \n", off[i]))
	}
	b.WriteString("trailer\n<< /Size " + strconv.Itoa(n) + " /Root 1 0 R >>\nstartxref\n" + strconv.Itoa(xref) + "\n%%EOF\n")
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A file that is not a PDF, and one that names a missing path, are each an ERROR
// carrying the path, never a verdict.
func TestUnreadableNamesTheFile(t *testing.T) {
	junk := filepath.Join(t.TempDir(), "x.pdf")
	if err := os.WriteFile(junk, []byte("not a pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	// With the check gone, Probe reads a nil document: recover, so the assertion
	// is what fails and not a panic that ends the run.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("a garbage file made Probe panic: %v", r)
			}
		}()
		if _, err := Probe(junk); err == nil || !strings.Contains(err.Error(), junk) {
			t.Errorf("a garbage file: %v", err)
		}
	}()
	if _, err := Probe(filepath.Join(t.TempDir(), "gone.pdf")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a missing file: %v", err)
	}
}

func TestZeroPagesIsNotAVerdict(t *testing.T) {
	p := writePDF(t, "empty.pdf", "", 0)
	_, err := Probe(p)
	if err == nil || !strings.Contains(err.Error(), "no pages") {
		t.Errorf("a document with no pages was given a verdict: %v", err)
	}
}

// A page whose content stream cannot be decoded is an ERROR naming the page: its
// text is unknown, so a leak on it could not have been seen.
func TestUndecodablePageIsAnErrorNotACleanPage(t *testing.T) {
	p := writePDF(t, "bad.pdf", "this is not flate data", 1, "/Filter /FlateDecode")
	_, err := Probe(p)
	if err == nil || !strings.Contains(err.Error(), "page 1") {
		t.Errorf("an undecodable page was read as clean: %v", err)
	}
}

func TestAHandBuiltPDFIsRead(t *testing.T) {
	p := writePDF(t, "two.pdf", "BT /F1 12 Tf 72 700 Td (hello) Tj ET", 2)
	v, err := Probe(p)
	if err != nil {
		t.Fatal(err)
	}
	if v.Pages != 2 {
		t.Errorf("pages = %d, want 2", v.Pages)
	}
}
