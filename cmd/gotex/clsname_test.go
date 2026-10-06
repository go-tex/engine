package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ⛔ A class name is a NAME, not a path. bundlesFor runs BEFORE the engine and
// reads the class file itself, so it sits outside the engine's read policy
// (readpolicy.go, go-tex/engine#551): \documentclass{../outside/evil} made it
// read a .cls the document had not shipped and act on what was in it — measured,
// the bundle list came back carrying pgf from that file.
//
// The disclosure is small: the file's CONTENT never reaches the page, and only
// names the texmf registry knows become bundles, so a document cannot aim the
// fetch at a URL of its own. But it is a read the engine itself would refuse.
//
// ⛔ The CONTROL is the second half: the same class sitting BESIDE the document
// must still be read, or this test would pass against a scan that had simply
// stopped reading classes.
func TestAClassNameIsNotAPath(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	doc := filepath.Join(root, "doc")
	for _, d := range []string{outside, doc} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cls := []byte("% a class the document did not ship\n\\usepackage{pgf}\n")
	if err := os.WriteFile(filepath.Join(outside, "evil.cls"), cls, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(doc, "local.cls"), cls, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(doc)

	for _, c := range []struct {
		name, class string
		want        bool // a bundle from the class file is expected
	}{
		{"a path out of the tree is not read", `../outside/evil`, false},
		{"an absolute path is not read", filepath.Join(outside, "evil"), false},
		{"CONTROL: a class beside the document is read", `local`, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := bundlesFor([]byte(`\documentclass{` + c.class + `}`))
			found := false
			for _, b := range got {
				if b.Name == "pgf" {
					found = true
				}
			}
			if found != c.want {
				t.Errorf("\\documentclass{%s}: pgf bundle present = %v, want %v (bundles: %v)",
					c.class, found, c.want, got)
			}
		})
	}
}
