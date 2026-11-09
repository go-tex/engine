// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
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

// ligatures spells out what the engine sets as one glyph and pdftotext reads back as
// letters (it expands them in the reference's text; the engine's page text keeps the
// presentation form).
var ligatures = strings.NewReplacer("ﬀ", "ff", "ﬁ", "fi", "ﬂ", "fl", "ﬃ", "ffi", "ﬄ", "ffl")

func stripSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, ligatures.Replace(s))
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

// url.sty's \path with TikZ loaded FIRST. TikZ installs its \path only inside a
// {tikzpicture} (\tikz@installcommands), so at the top level the name is still free
// when url.sty tests it — \@ifundefined{path}{\DeclareUrlCommand\path…} — and url's
// holds there. The characters _ % and \. can only compile in text mode if url.sty's
// catcodes read the argument; tectonic 0.17.0 typesets both with 0 errors.
func TestParityUrlPathAfterTikz(t *testing.T) {
	_, got, want := parityPair(t, "urlpath")
	if got != want {
		t.Errorf("page text differs from tectonic's\n got: %q\nwant: %q", got, want)
	}
}

// The default face is Latin Modern, as tectonic's: a plain article sets the space of
// lmroman10 (\lastskip prints 3.33pt plus 1.665pt minus 1.11pt on both engines) and
// embeds LMRoman10-Regular, -Bold, -Italic and LMMono10-Regular. Before this the
// engine set STIX Two Math, a maths face whose space is 2.00pt. The decision is the
// owner's, 2026-11-09: conform to tectonic.
func TestParityDefaultFaceIsLatinModern(t *testing.T) {
	_, got, want := parityPair(t, "lmface")
	if got != want {
		t.Errorf("page text differs from tectonic's\n got: %q\nwant: %q", got, want)
	}
}

// BaseFont names, as `pdffonts` reports them for tectonic's PDF with the subset tag
// and the Identity-H suffix removed.
func TestParityDefaultFaceBaseFonts(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "lmface.tex"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := os.ReadFile(filepath.Join("testdata", "parity", "lmface.tectonic.fonts.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := CompileToPDF(src, Options{}, &buf); err != nil {
		t.Fatal(err)
	}
	for _, name := range strings.Fields(string(names)) {
		if !bytes.Contains(buf.Bytes(), []byte("+"+name)) {
			t.Errorf("the PDF embeds no %s, tectonic's does", name)
		}
	}
}

// A fontspec article with no \setmainfont: the SET of font families in the PDF, which
// is exact where a page count is not. tectonic 0.17.0 names (testdata) lmroman 10/12/
// 8/7/6, lmmono 10/8, lmmonolt10-bold and CMR/CMMI/CMEX for the maths. The engine
// named only STIXTwoMath-Regular before the default face became Latin Modern; it now
// names the four regular-size text faces, every one of them a member of tectonic's
// set. Still missing, and said so: the optical sizes (lmroman12/8/7/6, lmmono8,
// lmmonolt10-bold) and the Computer Modern maths faces (the maths face is a later
// step of the same issue).
func TestParityFontspecFamilySet(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "fontspecfamilies.tex"))
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile(filepath.Join("testdata", "parity", "fontspecfamilies.tectonic.fonts.txt"))
	if err != nil {
		t.Fatal(err)
	}
	refSet := map[string]bool{}
	for _, n := range strings.Fields(string(ref)) {
		refSet[n] = true
	}
	var buf bytes.Buffer
	if _, err := CompileToPDF(src, Options{}, &buf); err != nil {
		t.Fatalf("strict compile: %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("STIXTwoMath")) {
		t.Error("the PDF still embeds STIXTwoMath, which tectonic's never does")
	}
	var found int
	for n := range refSet {
		if bytes.Contains(buf.Bytes(), []byte("+"+n)) {
			found++
		}
	}
	if found < 4 {
		t.Errorf("only %d of tectonic's %d families are embedded, want at least the four text faces", found, len(refSet))
	}
}
