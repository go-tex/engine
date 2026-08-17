package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// Every way of reading a file in must mark where that file ends. \endinput means
// "stop reading THIS file", and it stops at the nearest end marker; a file spliced
// without one lets an \endinput inside it run on to the enclosing file's marker
// and swallow the rest of THAT file. pgf loads its libraries with
// \InputIfFileExists, and each library ends with \endinput — so a package that
// loaded a library kept only what came before it, losing every definition after.
func TestInputIfFileExistsEndsAtItsOwnFile(t *testing.T) {
	dir := t.TempDir()
	// The library stops itself halfway, as pgf's libraries do.
	if err := os.WriteFile(filepath.Join(dir, "biblio.tex"),
		[]byte("\\message{[bib-debut]}\\endinput\n\\message{[bib-jamais]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The file that loads it must keep running afterwards.
	if err := os.WriteFile(filepath.Join(dir, "englobant.tex"),
		[]byte("\\message{[avant]}\\InputIfFileExists{biblio.tex}{}{}\\message{[apres]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\input{englobant.tex}\message{[fin]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[avant] [bib-debut] [apres] [fin]" {
		t.Errorf("= %q, want the loading file to continue after the library", got)
	}
}

// The same holds for \input, and for a file loaded through \InputIfFileExists
// inside a package: the lines of each file are discounted from the document's
// own numbering, whichever way it was read in.
func TestEveryFileSpliceIsAccounted(t *testing.T) {
	dir := t.TempDir()
	body := "% commentaire\n% commentaire\n% commentaire\n\\endinput\n"
	for _, name := range []string{"un.tex", "deux.tex"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run("\\input{un.tex}\\InputIfFileExists{deux.tex}{}{}\nTrouve"); err != nil {
		t.Fatal(err)
	}
	line := 0
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case charNode:
				if v.ch == 'T' && line == 0 {
					line = v.srcLine
				}
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	if line != 2 {
		t.Errorf("glyph after two 4-line files reports line %d, want 2", line)
	}
}

// A library that does NOT stop itself still hands control back at its end.
func TestInputIfFileExistsWithoutEndinput(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "suite.tex"), []byte(`\message{[dedans]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\InputIfFileExists{suite.tex}{\message{[oui]}}{\message{[non]}}\message{[apres]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[oui] [dedans] [apres]" {
		t.Errorf("= %q", got)
	}
}

// The engine's colour bridge and the fully-loaded key machinery together let a
// drawing package's colour options through: the explicit forms name the colour
// on the stroke and the fill.
func TestTikzColourOptionsReachTheDriver(t *testing.T) {
	// Exercised end to end by the pgf integration only when the real sources are
	// present; here we check the piece the engine owns — that a colour named in a
	// package's own form resolves to the right rgb values.
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\makeatletter\def\read#1#2#3#4#5{\message{[#4:#5]}}` +
		`\expandafter\expandafter\expandafter\read\csname\string\color@green\endcsname`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[rgb:0,1,0]" {
		t.Errorf("green = %q, want [rgb:0,1,0]", got)
	}
}
