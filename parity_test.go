// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
)

// The witnesses in testdata/parity are minimal documents compiled by BOTH engines.
// The engine's own tests may not run a process (capabilities_test.go), so the
// reference's side is a golden file recorded from `tectonic` 0.17.0:
//
//	tectonic -X compile --outdir out testdata/parity/<name>.tex
//	pdftotext -layout out/<name>.pdf testdata/parity/<name>.tectonic.txt
//
// The comparison drops every white-space rune: pageText holds characters, not the
// glue between them, so spacing is asserted separately where a witness is about it.

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// parityPair compiles testdata/parity/<name>.tex strictly and returns the engine,
// its page text and the recorded reference text.
func parityPair(t *testing.T, name string) (*Engine, string, string) {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "parity", name+".tex"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile(filepath.Join("testdata", "parity", name+".tectonic.txt"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatalf("strict compile of %s: %v", name, err)
	}
	return e, stripSpace(pageText(e)), stripSpace(string(ref))
}

func TestParityAllowbreakInText(t *testing.T) {
	_, got, want := parityPair(t, "allowbreak")
	if got != want {
		t.Errorf("page text differs from tectonic's\n got: %q\nwant: %q", got, want)
	}
}
