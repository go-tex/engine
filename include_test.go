// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// \include{FILE} must read FILE.tex into the document — LaTeX's
// \clearpage\input{FILE}\clearpage. Left undefined, \include was an unknown control
// sequence: in lenient mode it was dropped and its {FILE} argument typeset as stray
// text, so a manuscript whose whole body lives in \include'd files (the standard
// LaTeX convention for splitting a paper) rendered zero pages. Regression for arXiv
// 2601.11013 (revtex4-2), whose top-level file only \includes its main text and
// supplement.
func TestIncludeLoadsFileBody(t *testing.T) {
	const marker = "INCLUDEDBODYMARK"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chap.tex"), []byte(marker+" reached the page.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	src := "\\documentclass{article}\n\\begin{document}\nBEFOREMARK\n\\include{chap}\n\\end{document}\n"
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("text before \\include is missing; got %q", txt)
	}
	if !strings.Contains(txt, marker) {
		t.Fatalf("\\include did not load the file body; got %q", txt)
	}
}

// \includeonly must not break \include: the engine does not honour the file-restriction
// list (over-including only adds content, never drops the document), but it must accept
// and discard the argument so a document that calls \includeonly still typesets.
func TestIncludeOnlyIsAccepted(t *testing.T) {
	const marker = "STILLINCLUDED"
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "part.tex"), []byte(marker+" here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)

	src := "\\documentclass{article}\n\\includeonly{part}\n\\begin{document}\n\\include{part}\n\\end{document}\n"
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, marker) {
		t.Fatalf("\\includeonly broke \\include; body missing, got %q", txt)
	}
}
