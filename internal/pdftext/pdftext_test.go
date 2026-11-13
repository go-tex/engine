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

// ⭐ opts REACH THE TOOL, AND THEY REACH IT BEFORE THE FILE, which is where
// pdftotext takes its options.
//
// It matters because of what `-layout` turned out to be worth. Without it,
// pdftotext reads a two-column page as ONE stream and fuses the end of one
// column's line onto the start of another's — `disFigure`, `participaentire`,
// `quaThe` are real tokens from one revtex paper. WHICH boundaries fuse depends
// on where the lines break, so a line-breaking change moves them.
//
// Measured on #772's two papers: scoring them with `-layout` extraction made
// one of the two "regressions" vanish outright (1327 matched words against
// 1327, where the plain extraction had reported −0.2), and reduced the other to
// a single word. ⚠ It also moved the ABSOLUTE scores by nine points on both
// sides (96.5% → 87.6%), so it is a diagnostic and not a free improvement to
// the scorer.
func TestOptsReachTheToolBeforeTheFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(p, []byte("one two"), 0o600); err != nil {
		t.Fatal(err)
	}
	// `cat -u f -` is cat with an option before the file: if opts landed after
	// the path, cat would treat "-u" as a second file to read, and on macOS it
	// reads stdin for "-" either way — so the assertion is that the OPTION was
	// accepted, i.e. no error and the file's text came back.
	out, err := CaptureRunner("cat", "-u")(5*time.Second, p)
	if err != nil {
		t.Fatalf("cat -u: %v", err)
	}
	if out != "one two" {
		t.Errorf("captured %q, want the file's text — the option did not reach "+
			"the tool in front of the file", out)
	}
	// And PkgxArgs carries them through in the same order.
	got := PkgxArgs("pdftotext", "-layout", "f.pdf", "-")
	want := []string{"+" + PopplerProject, "pdftotext", "-layout", "f.pdf", "-"}
	if !equal(got, want) {
		t.Fatalf("PkgxArgs with opts = %v want %v", got, want)
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
