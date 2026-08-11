// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCorpus is the document-level regression net: every .tex in testdata/corpus
// (real LaTeX documents, one per feature area, extracted from the browser demo) is
// compiled through the full public pipeline — CompileToSVGPages and CompileToPDF —
// and checked for structural validity. It complements the low-level TestConformance
// (mouth/gullet \message ratchet) and the per-feature unit tests by exercising whole
// documents end to end, so an interaction bug between features fails the build. It
// uses only the built-in font and no filesystem, so it runs unchanged in CI on every
// platform.
func TestCorpus(t *testing.T) {
	files, err := filepath.Glob("testdata/corpus/*.tex")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 30 {
		t.Fatalf("corpus has only %d files; expected the full feature set", len(files))
	}
	for _, f := range files {
		f := f
		t.Run(strings.TrimSuffix(filepath.Base(f), ".tex"), func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}

			// SVG pipeline: must produce at least one page of real, drawn material.
			pages, err := CompileToSVGPages(src, Options{})
			if err != nil {
				t.Fatalf("CompileToSVGPages: %v", err)
			}
			if len(pages) == 0 {
				t.Fatal("no SVG pages produced")
			}
			all := strings.Join(pages, "")
			if !strings.Contains(all, "<svg") {
				t.Errorf("output is not SVG: %.80s", all)
			}
			// A drawn document contains glyph paths, rules, images or links — an empty
			// (all-white) page would mean the document silently produced nothing.
			if !containsAny(all, "<path", "<rect", "<image", "<a ", "<g transform") {
				t.Errorf("SVG has no drawn content (glyphs/rules/images/links)")
			}

			// PDF pipeline: must produce a valid, non-trivial PDF without crashing.
			var buf bytes.Buffer
			n, err := CompileToPDF(src, Options{}, &buf)
			if err != nil {
				t.Fatalf("CompileToPDF: %v", err)
			}
			if n < 1 {
				t.Errorf("PDF reports %d pages", n)
			}
			if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF")) {
				t.Errorf("output is not a PDF (no %%PDF header)")
			}
			if buf.Len() < 500 {
				t.Errorf("PDF is suspiciously small: %d bytes", buf.Len())
			}
		})
	}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
