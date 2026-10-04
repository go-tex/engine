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

// ⛔ A nested \import must still find its file, and \subimport is not the only nesting
// idiom in the wild: a paper whose chapters each live in their own directory writes
//
//	main.tex                     \import{chapters/01-intro}{main.tex}
//	chapters/01-intro/main.tex   \import{./}{intro.tex}
//
// — \import, not \subimport, with a path relative to the importing file. import.sty
// serves that because the reset and the search are two different things:
//
//	\newcommand{\import}{\global\let\import@path\@empty …}   % the PATH is reset
//	\protected@edef\input@path{{\import@path}#2}             % the SEARCH accumulates
//
// Honouring only the reset sent ./intro.tex to the working directory. Measured on
// 2601.22691, which imports five chapters that each import their own sections: 9 of its
// 14 \import calls were recorded as a skipped command and the paper came out at 2 pages
// of 132KB; with the enclosing directories retained it is 27 pages and none is skipped.
func TestANestedImportResolvesAgainstTheImportingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "chapters", "01-intro"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, s string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("chapters/01-intro/intro.tex", `DEDANS`)
	write("chapters/01-intro/main.tex", `CHAPITRE \import{./}{intro.tex}`)
	write("main.tex", `\documentclass{article}\begin{document}`+
		`\import{chapters/01-intro}{main.tex}APRES\par\end{document}`)

	src, err := os.ReadFile(filepath.Join(dir, "main.tex"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	// CHAPITRE proves the outer import worked and was never in doubt; DEDANS is the
	// witness for this change.
	for _, want := range []string{"CHAPITRE", "DEDANS", "APRES"} {
		if !strings.Contains(txt, want) {
			t.Errorf("%q missing — a nested \\import did not resolve against its own file: %q",
				want, txt)
		}
	}
	if n := e.Diagnostics().Skipped["import"]; n != 0 {
		t.Errorf("\\import recorded as skipped %d time(s): the lookup exhausted its candidates", n)
	}
	if e.importPath != "" {
		t.Errorf("import path %q left behind", e.importPath)
	}
}
