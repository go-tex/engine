// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ⛔ THE CLAIM "EMBEDDED UNCHANGED" IS THE WHOLE LICENCE ARGUMENT, so it is
// checked rather than written down. Every file in embeddedFontProvenance must
// hash to the value recorded beside it.
//
// This is not a test of the fonts. It is a test of the RECORD: a font file
// swapped, re-optimised, subsetted or "fixed" without the record changing is
// exactly what would void the GUST Font License's one request, and exactly what
// nobody would notice, because a font that still renders looks fine.
func TestEmbeddedFontsMatchTheirRecordedProvenance(t *testing.T) {
	if len(embeddedFontProvenance) == 0 {
		t.Fatal("no provenance recorded: this test would pass over an empty list, " +
			"which is the way a check like this goes vacuous")
	}
	for _, p := range embeddedFontProvenance {
		b, err := os.ReadFile(filepath.Join("fonts", p.file))
		if err != nil {
			t.Errorf("%s: %v", p.file, err)
			continue
		}
		sum := sha256.Sum256(b)
		got := hex.EncodeToString(sum[:])
		if got != p.sha256 {
			t.Errorf("%s: sha256 = %s, recorded %s\n"+
				"The file and the record disagree. Either the file was changed — in which "+
				"case 'embedded unchanged' is no longer true and the licence argument needs "+
				"redoing — or it was replaced from a different upstream version, in which "+
				"case say which in %s.", p.file, got, p.sha256, p.pkg)
		}
	}
}

// Every file under fonts/ must be accounted for, or the record is a list of
// what somebody remembered rather than of what ships.
//
// ⛔ THE RECORD BEING RIGHT ABOUT WHAT IT MENTIONS IS NOT ENOUGH. A sixth font
// dropped into fonts/lm/ and embedded would pass the checksum test above
// without appearing in it at all, because that test iterates the record and not
// the directory. This one iterates the directory.
func TestEveryVendoredFontIsRecorded(t *testing.T) {
	recorded := map[string]bool{}
	for _, p := range embeddedFontProvenance {
		recorded[filepath.ToSlash(p.file)] = true
	}
	var unrecorded []string
	err := filepath.WalkDir("fonts", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.EqualFold(filepath.Ext(path), ".otf") &&
			!strings.EqualFold(filepath.Ext(path), ".ttf") {
			return nil // licences, manifests and readmes are not fonts
		}
		rel, err := filepath.Rel("fonts", path)
		if err != nil {
			return err
		}
		if !recorded[filepath.ToSlash(rel)] {
			unrecorded = append(unrecorded, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking fonts/: %v", err)
	}
	for _, f := range unrecorded {
		t.Errorf("%s ships in this binary and is in no provenance record: say which CTAN "+
			"package it came from, which version it declares, and its sha256", f)
	}
}

// The licence text and both upstream manifests must travel with the fonts. The
// GUST Font License names the manifest as the document that defines the Work,
// so shipping the fonts without it ships an argument with its evidence removed.
func TestTheLicenceAndManifestsTravelWithTheFonts(t *testing.T) {
	for _, f := range []string{
		"lm/GUST-FONT-LICENSE.txt",
		"lm/MANIFEST-Latin-Modern.txt",
		"lm/README-Latin-Modern.txt",
		"lm/MANIFEST-Latin-Modern-Math.txt",
		"lm/README-Latin-Modern-Math.txt",
	} {
		st, err := os.Stat(filepath.Join("fonts", f))
		if err != nil {
			t.Errorf("%s is missing: %v", f, err)
			continue
		}
		if st.Size() == 0 {
			t.Errorf("%s is empty, which ships the filename and not the document", f)
		}
	}
}
