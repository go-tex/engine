// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// \import{dir}{file} reads dir/file. Undefined, it was skipped — and a skipped
// \import does not lose a command, it loses a FILE: one corpus paper imports seven
// and rendered 5 pages against a reference of 28.
func TestImportReadsTheFileItNames(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sections"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The imported file itself \subimports a neighbour, which must resolve relative
	// to the directory the importing file lives in.
	write("sections/intro.tex", `INTRO \subimport{sub/}{deep}`)
	if err := os.MkdirAll(filepath.Join(dir, "sections", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	write("sections/sub/deep.tex", `PROFOND`)
	write("main.tex", `\documentclass{article}\begin{document}\import{sections/}{intro}APRES\par\end{document}`)

	src, err := os.ReadFile(filepath.Join(dir, "main.tex"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir) // \import resolves against the directory the document is read from
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"INTRO", "PROFOND", "APRES"} {
		if !strings.Contains(txt, want) {
			t.Errorf("%q missing — the import did not read its file: %q", want, txt)
		}
	}
	if e.importPath != "" {
		t.Errorf("import path %q left behind: the pop must run at the end of the file", e.importPath)
	}
}

// A file the paper does not ship is recorded like any other skipped input rather
// than failing the render.
func TestImportOfAMissingFileIsRecorded(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\import{nulle-part/}{absent}x\par`); err != nil {
		t.Fatal(err)
	}
	if e.skippedCS["import"] != 1 {
		t.Errorf("a missing import was not recorded: %v", e.skippedCS)
	}
}
