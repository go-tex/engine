// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// The two faces bound only with the default family are parsed when it is built,
// and one that does not parse refuses the build. The embedded bytes cannot fail to
// parse, so each is truncated in turn.
func TestATruncatedBoldItalicOrBoldSlantedFaceRefusesTheDefaultFamily(t *testing.T) {
	for name, face := range map[string]*[]byte{
		"bold italic":  &lmRomanBoldItalic,
		"bold slanted": &lmRomanBoldSlant,
	} {
		saved := *face
		*face = saved[:len(saved)/2]
		_, err := buildEngine(Options{}, true)
		*face = saved
		if err == nil {
			t.Errorf("buildEngine with a truncated %s face: want an error, got nil", name)
		}
	}
	if _, err := buildEngine(Options{}, true); err != nil {
		t.Errorf("buildEngine with the embedded faces: %v", err)
	}
}

// faceKey names no face for what is not an OpenType font, a nil one, or a font
// with no bytes.
func TestAFaceThatCannotBeIdentifiedHasNoKey(t *testing.T) {
	// A guard that is gone dereferences nil: report that as a failure, not a crash.
	named := func(f fontFace) (k *byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("faceKey(%T) panicked: %v", f, r)
			}
		}()
		return faceKey(f)
	}
	if named(nil) != nil {
		t.Error("faceKey(nil) named a face")
	}
	if named((*OpenTypeFont)(nil)) != nil {
		t.Error("faceKey of a nil font named a face")
	}
	if k := faceKey(&OpenTypeFont{}); k != nil {
		t.Error("faceKey of a font with no bytes named a face")
	}
}

// A series switch and a shape switch combine (nfss.go). Against tectonic on
// testdata/parity/bolditalic.tex, the face of each line:
//
//	line                          tectonic          before
//	\textbf{\textit{…}}           LMRoman10-BoldItalic   LMRoman10-Italic
//	{\itshape\bfseries …}         LMRoman10-BoldItalic   LMRoman10-Bold
//	{\bfseries\slshape …}         LMRomanSlant10-Bold    LMRomanSlant10-Regular
//	{\bfseries\itshape\upshape …} LMRoman10-Bold         LMRoman10-Regular
//	{\bfseries\itshape\mdseries …} LMRoman10-Italic      LMRoman10-Regular
//	{\it\bf …}                    LMRoman10-Bold         LMRoman10-Bold
//
// The last row is the control: the old one-letter switches do not combine in
// LaTeX either (\bf is \normalfont\bfseries). Every line now ends where
// tectonic's does, 281.18 / 280.89 / 260.49 bp.
func TestParitySeriesAndShapeCombine(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "parity", "bolditalic.tex"))
	if err != nil {
		t.Fatal(err)
	}
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var faces []*byte
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && directChar(b) {
				if ch, ok := firstGlyph(b); ok {
					faces = append(faces, faceKey(ch.face))
				}
				continue
			}
			walk(b.list)
		}
	}
	for _, p := range e.Pages() {
		walk([]node{p})
	}
	key := func(b []byte) *byte {
		if fam := opticalOf(b); len(fam) > 0 {
			return &fam[0].data[0]
		}
		return &b[0]
	}
	want := []struct {
		name string
		key  *byte
	}{
		{`\textbf{\textit{…}}: bold italic`, key(lmRomanBoldItalic)},
		{`{\itshape\bfseries …}: bold italic`, key(lmRomanBoldItalic)},
		{`{\bfseries\slshape …}: bold slanted`, key(lmRomanBoldSlant)},
		{`{\bfseries\itshape\upshape …}: bold`, key(lmRomanBold)},
		{`{\bfseries\itshape\mdseries …}: italic`, key(lmRomanItalic)},
		{`{\it\bf …}: bold, the old switches do not combine`, key(lmRomanBold)},
	}
	// Six lines and the folio.
	if len(faces) != len(want)+1 {
		t.Fatalf("%d lines, want %d plus the folio", len(faces), len(want))
	}
	for i, w := range want {
		if faces[i] != w.key {
			t.Errorf("line %d is not set in the face tectonic uses: %s", i+1, w.name)
		}
	}
}

// firstGlyph is the first glyph of an hbox, descending into the hboxes it holds.
func firstGlyph(b *boxNode) (charNode, bool) {
	for _, c := range b.list {
		switch v := c.(type) {
		case charNode:
			return v, true
		case *boxNode:
			if v.kind == hbox {
				if ch, ok := firstGlyph(v); ok {
					return ch, true
				}
			}
		}
	}
	return charNode{}, false
}
