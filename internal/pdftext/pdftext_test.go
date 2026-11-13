// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package pdftext

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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

// ⛔ POPPLER'S AMBIGUOUS TOOLS ARE PINNED, OR pkgx FAILS WITH MultipleProjects
// and the fallback silently produces no text — which reads as "this page is
// empty" rather than "the extractor never ran".
func TestPkgxArgsPinsPopplerAndLeavesOthersAlone(t *testing.T) {
	got := PkgxArgs("pdftotext", "file.pdf", "-")
	want := []string{"+" + PopplerProject, "pdftotext", "file.pdf", "-"}
	if !equal(got, want) {
		t.Fatalf("PkgxArgs(pdftotext)=%v want %v", got, want)
	}
	if got := PkgxArgs("pdftoppm", "-png", "f.pdf"); got[0] != "+"+PopplerProject || got[1] != "pdftoppm" {
		t.Fatalf("PkgxArgs(pdftoppm)=%v not pinned", got)
	}
	if got := PkgxArgs("tectonic", "-o", "d"); !equal(got, []string{"tectonic", "-o", "d"}) {
		t.Fatalf("PkgxArgs(tectonic)=%v want unpinned", got)
	}
}

// CaptureRunner over a tool that IS on PATH returns what it writes to stdout.
// `cat f -` prints the file and then an empty stdin, which is close enough to
// pdftotext's contract to exercise the PATH branch.
func TestCaptureRunnerReturnsStdout(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte("the measure is wrong"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := CaptureRunner("cat")(5*time.Second, p)
	if err != nil {
		t.Fatalf("cat: %v", err)
	}
	if out != "the measure is wrong" {
		t.Errorf("captured %q", out)
	}
}

// ⛔ A TOOL THAT IS NOT THERE MUST ERROR, not return an empty string: an empty
// extraction and a missing extractor are different facts and the caller cannot
// tell them apart afterwards.
func TestCaptureRunnerErrorsWhenTheToolIsMissing(t *testing.T) {
	out, err := CaptureRunner("definitely-not-a-real-binary-xyz")(5*time.Second, "f.pdf")
	if err == nil {
		t.Fatalf("a missing tool returned no error, and %q of text", out)
	}
}
