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

// ⛔ THE REASON measure PARSES. The measurements this tool replaces counted
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
			got, err := measure(p)
			if err != nil {
				t.Fatalf("measure: %v", err)
			}
			if got.Pages != 3 {
				t.Errorf("measure = %d pages, want 3", got.Pages)
			}
			// ⛔ THE CONTROL THAT KEEPS INK FROM BEING PAGES. These pages are
			// blank, so the glyph total must be ZERO — a glyph count that came
			// back 3 here would be the page count under another name, and it
			// would answer every "did the ink move" question with "exactly as
			// much as the pagination".
			if got.Glyphs != 0 {
				t.Errorf("three BLANK pages reported %d glyphs: the ink count is reading pages, "+
					"not ink", got.Glyphs)
			}
			// ⛔⛔ AND THE ZERO ABOVE MUST BE A MEASURED ZERO. Without this line
			// the control passes just as well when NO page decoded at all — the
			// blind-scan-reports-zero shape this fleet has paid for before. A
			// blank page is readable and holds no glyphs; an unreadable page is a
			// different fact and is counted in its own field.
			if got.BadPages != 0 {
				t.Errorf("%d of 3 pages did not decode, so the 0 glyphs above is an ABSENCE OF "+
					"MEASUREMENT and not an absence of ink", got.BadPages)
			}
			if n := len(grep.FindAll(b, -1)); n != c.wantGrep {
				t.Errorf("the byte-grep found %d, and this test exists to pin that it finds %d here", n, c.wantGrep)
			}
		})
	}
}

// ⛔ An unreadable file must be an ERROR, never a count. "No output", "an empty
// document" and "I cannot parse this" all arrived as 0 from the old instrument,
// and a zero mixes into a sum without a trace. That is now true of TWO sums — the
// pages and the ink — so the refusal matters twice.
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
		if m, err := measure(p); err == nil {
			t.Errorf("%s: measure returned %d pages / %d glyphs and no error",
				filepath.Base(p), m.Pages, m.Glyphs)
		}
	}
	// The control: a good file still reads, so the three above fail for their own
	// reason and not because measure refuses everything.
	good := filepath.Join(dir, "good.pdf")
	if err := os.WriteFile(good, writePDF(t, 2, true), 0o644); err != nil {
		t.Fatal(err)
	}
	if m, err := measure(good); err != nil || m.Pages != 2 {
		t.Fatalf("control: measure = %d pages, %v; want 2, nil", m.Pages, err)
	}
}

// ⛔ THE INK VERDICT IS A THREE-WAY CLASSIFICATION, and it is the only part of
// this report that says what a number MEANS rather than what it is. "Pages moved
// and ink did not" is a breaking change; "ink moved and pages did not" is a
// content change; they are opposite findings and the two inputs that distinguish
// them differ by nothing but which is zero. A branch chosen backwards would label
// every finding as its opposite, in a sentence a reader has no reason to doubt.
//
// So each branch gets a row, and the row names the word the reader will act on.
func TestTheInkVerdictNamesTheRightKindOfChange(t *testing.T) {
	for _, c := range []struct {
		name                            string
		base, head, inkMoved, pageMoved int
		want, mustNotSay                string
	}{
		{
			name: "pagination moved, ink held still",
			base: 50_000, head: 50_000, inkMoved: 0, pageMoved: 7,
			want: "BREAKING", mustNotSay: "CONTENT",
		},
		{
			name: "ink moved, pagination held still",
			base: 50_000, head: 51_200, inkMoved: 3, pageMoved: 0,
			want: "CONTENT", mustNotSay: "BREAKING",
		},
		{
			name: "neither moved",
			base: 50_000, head: 50_000, inkMoved: 0, pageMoved: 0,
			want: "does not reach this corpus", mustNotSay: "BREAKING",
		},
		{
			name: "both moved",
			base: 50_000, head: 51_200, inkMoved: 3, pageMoved: 7,
			want: "both a content and a breaking change", mustNotSay: "does not reach",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			reportInk(&buf, c.base, c.head, 0, 0, c.inkMoved, c.pageMoved)
			out := buf.String()
			if !strings.Contains(out, c.want) {
				t.Errorf("the verdict does not say %q:\n%s", c.want, out)
			}
			if strings.Contains(out, c.mustNotSay) {
				t.Errorf("the verdict says %q, which is the OPPOSITE finding:\n%s", c.mustNotSay, out)
			}
			// ⛔ And in every case the caveat travels with the number. A Σ with a
			// signed delta beside two columns that ARE scored against a reference
			// will be read as a third score unless the line says it is not one.
			if !strings.Contains(out, "NOT A SCORE") {
				t.Errorf("Σ glyphs was printed without saying there is no reference ink:\n%s", out)
			}
		})
	}
}

// ⛔ A page that parsed but whose text did not decode makes BOTH Σ glyphs short,
// by different amounts, and their delta a difference between two incomplete sums.
// It must be said out loud, because the delta itself looks exactly like ink that
// moved.
func TestUndecodablePagesDisownTheirOwnDelta(t *testing.T) {
	var buf bytes.Buffer
	reportInk(&buf, 50_000, 51_200, 0, 2, 3, 0)
	out := buf.String()
	if !strings.Contains(out, "incomplete sums") {
		t.Errorf("2 undecodable pages on the head did not disown the glyph delta:\n%s", out)
	}
	// The control: with nothing undecodable the warning must NOT appear, or it
	// would be furniture a reader learns to skip — including on the run where it
	// is true.
	buf.Reset()
	reportInk(&buf, 50_000, 51_200, 0, 0, 3, 0)
	if strings.Contains(buf.String(), "incomplete sums") {
		t.Errorf("the warning appeared with no undecodable page:\n%s", buf.String())
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
		{paper: Paper{ID: "ok", RefPage: 10}, base: measurement{Pages: 10}, head: measurement{Pages: 10}},
		{paper: Paper{ID: "broken", RefPage: 10}, baseErr: os.ErrNotExist},
	}
	if err := report(rs, 5, false); err == nil {
		t.Error("report scored a population that lost a paper")
	}
	if err := report(rs, 5, true); err != nil {
		t.Errorf("report(-keep-going) refused anyway: %v", err)
	}
}
