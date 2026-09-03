// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// A figure this engine cannot draw still states how big it is. Reading that is what
// makes the reserved box the right SHAPE: with only [width=…] given — which is how
// nearly every paper includes a figure — the height is the file's own aspect ratio
// instead of a fixed default, and the text around it breaks where it should.
func TestFigureDeclaredSize(t *testing.T) {
	dir := t.TempDir()
	for _, c := range []struct {
		nom, fichier, contenu string
		w, h                  int
	}{
		{"EPS BoundingBox", "a.eps", "%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 300 150\n", 300, 150},
		{"EPS HiRes gagne", "b.eps", "%!PS-Adobe-3.0\n%%BoundingBox: 0 0 300 150\n%%HiResBoundingBox: 0 0 200.4 100.2\n", 200, 100},
		{"EPS décalée", "c.eps", "%!PS-Adobe-3.0\n%%BoundingBox: 10 20 110 70\n", 100, 50},
		{"PDF MediaBox", "d.pdf", "%PDF-1.5\n1 0 obj<</Type/Page/MediaBox [0 0 432 288]>>endobj\n", 432, 288},
		{"boîte dégénérée", "e.eps", "%!PS-Adobe-3.0\n%%BoundingBox: 0 0 0 0\n", 0, 0},
		{"boîte différée", "f.eps", "%!PS-Adobe-3.0\n%%BoundingBox: (atend)\n", 0, 0},
	} {
		p := filepath.Join(dir, c.fichier)
		if err := os.WriteFile(p, []byte(c.contenu), 0o644); err != nil {
			t.Fatal(err)
		}
		if w, h := figureDeclaredSize(p); w != c.w || h != c.h {
			t.Errorf("%s: (%d, %d), want (%d, %d)", c.nom, w, h, c.w, c.h)
		}
	}
	if w, h := figureDeclaredSize(filepath.Join(dir, "absent.eps")); w != 0 || h != 0 {
		t.Errorf("fichier absent: (%d, %d), want (0, 0)", w, h)
	}
}

// The placeholder for an EPS included with [width=…] must be as tall as the file's
// aspect ratio says, not the engine's default square. The document names the figure
// the way a paper does — a bare file name, resolved against the working directory —
// rather than an absolute path, which would carry the temporary directory's own
// characters (a backslash on Windows, a ~ in a runner's short path) into TeX's
// tokeniser and be read as something else.
func TestEPSPlaceholderKeepsItsAspect(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fig.eps"),
		[]byte("%!PS-Adobe-3.0 EPSF-3.0\n%%BoundingBox: 0 0 400 100\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd) //nolint:errcheck // restoring the test's own directory

	e, err := compile([]byte(`\documentclass{article}\usepackage{graphicx}\begin{document}`+
		`\includegraphics[width=200pt]{fig.eps}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	var got *frameNode
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case frameNode:
				if got == nil {
					f := v
					got = &f
				}
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	walk(e.parList)
	if got == nil {
		t.Fatal("aucun cadre de remplacement sur la page")
	}
	// 400 × 100 bp asked for at 200pt wide is 50pt tall (the bp→pt correction is
	// under a percent and cancels in the ratio).
	if w, h := got.inner.width, got.inner.height; w != 200*unity || h < 49*unity || h > 51*unity {
		t.Errorf("cadre %.1fpt × %.1fpt, want 200.0pt × 50.0pt", spToPt(w), spToPt(h))
	}
}
