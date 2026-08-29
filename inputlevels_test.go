// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tex.web §537: start_input does begin_file_reading, "set up cur_file and new level
// of input". The file is a LEVEL of its own — it is read to its end before whatever
// was being read when it was opened resumes.
//
// This mouth reads pending token lists before the character buffer, so a file
// merged into that buffer while a macro was still expanding arrived AFTER the rest
// of the macro's body. beamer's [fragile] frame ends by expanding
// \frame<*>[…]{\begingroup\input{\jobname.vrb}\endgroup}: the frame's body came back
// after the frame had closed.

func TestInputFromAMacroIsReadBeforeTheRestOfIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "vu.tex"), []byte(`\gdef\vu{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\def\x{\input{vu.tex}\ifdefined\vu\message{[dans l'ordre]}\else\message{[trop tard]}\fi}\x`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[dans l'ordre]" {
		t.Errorf("= %q, want the file read before the rest of the macro's body", got)
	}
}

// What the file leaves pending is read before the level it interrupted: the two
// belong to different levels, and the outer one only resumes when the file is done.
func TestWhatAFileLeavesPendingIsReadBeforeTheLevelUnderIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "suite.tex"), []byte(`\message{[a]}\suite`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\def\suite{\message{[b]}}\def\x{\input{suite.tex}\message{[c]}}\x`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[a] [b] [c]" {
		t.Errorf("= %q, want [a] [b] [c]", got)
	}
}

// A file read from inside a file returns to the RIGHT level: the inner one finishes,
// then the outer one, then the document.
func TestNestedInputReturnsThroughEachLevel(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"interne.tex": `\message{[interne]}`,
		"externe.tex": `\message{[externe-debut]}\input{interne.tex}\message{[externe-fin]}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\input{externe.tex}\message{[document]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[externe-debut] [interne] [externe-fin] [document]" {
		t.Errorf("= %q", got)
	}
}

// The no-progress guard measures forward progress by watching the mouth's position
// in the buffer it is reading. A new level starts that position again at 0, so its
// baseline travels with the level: without that, every file read after the first
// looked like an expansion loop that was getting nowhere, and long documents were
// cut off mid-way.
func TestALongFileAfterALongOneIsNotMistakenForALoop(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("mot mot mot mot mot\n", 400)
	for _, n := range []string{"un.tex", "deux.tex"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(long+`\message{[`+n+`]}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\input{un.tex}\input{deux.tex}\message{[fin]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[un.tex] [deux.tex] [fin]" {
		t.Errorf("= %q, want both files read to their end", got)
	}
}

// \InputIfFileExists runs its then-code BEFORE the file, which is what ltfiles.dtx
// does: \IfFileExists{#1}{#2\@addtofilelist{#1}\@@input\@filef@und}. A package
// announces itself, or sets what the file it is about to read expects to find.
func TestInputIfFileExistsRunsItsThenCodeFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "apres.tex"), []byte(`\message{[fichier]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\InputIfFileExists{apres.tex}{\message{[alors]}}{\message{[sinon]}}\message{[suite]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[alors] [fichier] [suite]" {
		t.Errorf("= %q", got)
	}
}
