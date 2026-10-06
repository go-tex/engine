// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/go-pdfkit/pdfkit"
)

// writePDF makes a document of n pages, either in the classic cross-reference
// form or packed into object streams, and returns its bytes.
func writePDF(t *testing.T, n int, packed bool) []byte {
	t.Helper()
	// Packed = PDF 1.5 object streams, FLATE-COMPRESSED as a whole, which is the
	// form a real LaTeX engine writes and the form the byte-grep cannot see.
	doc := pdfkit.New(pdfkit.Options{ObjectStreams: packed, Compress: packed})
	for i := 0; i < n; i++ {
		doc.AddPage(pdfkit.A4)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// ⛔ THE REASON pageCount PARSES. The measurements this tool replaces counted
// "/Type /Page" with a regular expression over the raw bytes. This test puts the
// two forms side by side: the regex reads the classic file and is BLIND to the
// object-stream file, which is the form the reference engine writes — so a
// perfectly good document came back as zero pages, indistinguishable from a
// compile that produced nothing.
func TestPageCountReadsBothPDFForms(t *testing.T) {
	grep := regexp.MustCompile(`/Type\s*/Page[^s]`)
	dir := t.TempDir()
	for _, c := range []struct {
		name          string
		objectStreams bool
		wantGrep      int // what the OLD instrument would have said
	}{
		{"classic cross-reference table", false, 3},
		{"packed into COMPRESSED object streams", true, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := writePDF(t, 3, c.objectStreams)
			p := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+".pdf")
			if err := os.WriteFile(p, b, 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := pageCount(p)
			if err != nil {
				t.Fatalf("pageCount: %v", err)
			}
			if got != 3 {
				t.Errorf("pageCount = %d, want 3", got)
			}
			if n := len(grep.FindAll(b, -1)); n != c.wantGrep {
				t.Errorf("the byte-grep found %d, and this test exists to pin that it finds %d here", n, c.wantGrep)
			}
		})
	}
}

// ⛔ An unreadable file must be an ERROR, never a count. "No output", "an empty
// document" and "I cannot parse this" all arrived as 0 from the old instrument,
// and a zero mixes into a sum without a trace.
func TestPageCountRefusesRatherThanReturningZero(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.pdf")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	junk := filepath.Join(dir, "junk.pdf")
	if err := os.WriteFile(junk, []byte("this is not a PDF at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{empty, junk, filepath.Join(dir, "absent.pdf")} {
		if n, err := pageCount(p); err == nil {
			t.Errorf("%s: pageCount returned %d and no error", filepath.Base(p), n)
		}
	}
	// The control: a good file still reads, so the three above fail for their own
	// reason and not because pageCount refuses everything.
	good := filepath.Join(dir, "good.pdf")
	if err := os.WriteFile(good, writePDF(t, 2, true), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := pageCount(good); err != nil || n != 2 {
		t.Fatalf("control: pageCount = %d, %v; want 2, nil", n, err)
	}
}

// ⛔ The reference page counts carry no identifier: line n belongs to line n of
// the corpus list. Read as a map keyed by paper id they match nothing, and the
// measurement reports "0 exact, Σ 0" — which reads like a perfect score.
func TestLoadCorpusPairsByPositionAndRefusesAMismatch(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, id := range []string{"1234.5678", "2345.6789", "3456.7890"} {
		d := filepath.Join(dir, id)
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, d+"//main.tex")
	}
	list := filepath.Join(dir, "list.txt")
	if err := os.WriteFile(list, []byte(strings.Join(paths, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	refs := filepath.Join(dir, "refs.txt")
	if err := os.WriteFile(refs, []byte("11\n22\n33\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := loadCorpus(list, refs)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{11, 22, 33}
	wantID := []string{"1234.5678", "2345.6789", "3456.7890"}
	for i, p := range got {
		if p.RefPage != want[i] || p.ID != wantID[i] {
			t.Errorf("entry %d is %s/%d, want %s/%d", i, p.ID, p.RefPage, wantID[i], want[i])
		}
	}
	// Short reference file: the pairing is positional, so this MUST refuse rather
	// than silently measure the first two papers.
	short := filepath.Join(dir, "short.txt")
	if err := os.WriteFile(short, []byte("11\n22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCorpus(list, short); err == nil {
		t.Error("a reference file with 2 entries against a list of 3 was accepted")
	}
	// A path that no longer exists must refuse too: a stale list shrinks the
	// population, and a smaller population flatters every total.
	gone := filepath.Join(dir, "gone.txt")
	if err := os.WriteFile(gone, []byte(strings.Join(append(paths[:2:2], filepath.Join(dir, "nope")+"//main.tex"), "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCorpus(gone, refs); err == nil {
		t.Error("a list naming a directory that does not exist was accepted")
	}
}

// ⛔ A paper that failed on one side must stop the verdict, not quietly leave the
// population. Scoring the rest is available, but only on purpose.
func TestReportRefusesToScoreALostPaper(t *testing.T) {
	rs := []result{
		{paper: Paper{ID: "ok", RefPage: 10}, base: 10, head: 10},
		{paper: Paper{ID: "broken", RefPage: 10}, baseErr: os.ErrNotExist},
	}
	if err := report(rs, 5, false); err == nil {
		t.Error("report scored a population that lost a paper")
	}
	if err := report(rs, 5, true); err != nil {
		t.Errorf("report(-keep-going) refused anyway: %v", err)
	}
}
