// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The class lead defines the space-suffixed robust forms \[<space> and \]<space> so
// that class code introspecting \[ as a LaTeX robust command (\csname[ \endcsname)
// finds a $$-bearing macro instead of \relax. The display-math primitive \[ itself is
// untouched.
func TestRobustDisplayMathDecoyDefined(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\documentclass{article}`); err != nil {
		t.Fatalf("load class: %v", err)
	}
	for _, name := range []string{"[ ", "] "} {
		m := e.eq[name]
		if m == nil {
			t.Fatalf("robust form \\%q is not defined", name)
		}
		if m.kind != mMacro {
			t.Errorf("robust form \\%q should be a macro, got kind %v", name, m.kind)
		}
		if !strings.Contains(e.toksToString(m.body), "$") {
			t.Errorf("robust form \\%q must carry a $ for the introspecting split, got %q", name, e.toksToString(m.body))
		}
	}
	// The display-math primitive \[ is left in place (kind mPrim, name "[").
	if m := e.eq["["]; m == nil || m.kind != mPrim || m.name != "[" {
		t.Errorf("display-math primitive \\[ was disturbed: %+v", m)
	}
}

// Regression: amsart's amsthm QED patch splices \def\@currenvir{displaymath} into \['s
// body by splitting it at a $. With \[ a primitive and the robust form undefined, that
// patch used to leak a stray $…\def\@currenvir{displaymath} into the stream, opening
// math that swallowed a lone \input body typeset after \maketitle. The robust decoy
// routes the patch to the (inert) space-suffixed form, so the input body survives.
func TestInputBodyAfterMaketitleSurvivesAmsart(t *testing.T) {
	delete(emulatedClasses, "amsart")
	defer func() { emulatedClasses["amsart"] = true }()

	const marker = "INPUTAFTERMAKETITLE"
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "gotex_maketitle_input.tex")
	if err := os.WriteFile(inputPath, []byte(marker+" renders now\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// TeX \input uses forward slashes and no extension; ToSlash keeps the path
	// valid on Windows (t.TempDir is under an OS temp dir, not /tmp).
	texPath := filepath.ToSlash(strings.TrimSuffix(inputPath, ".tex"))

	src := "\\documentclass{amsart}\n\\title{A Title}\\author{An Author}\n" +
		"\\begin{document}\n\\maketitle\n\\input{" + texPath + "}\nMore body after input.\n\\end{document}\n"

	// Strict: the stray-$ math error is gone.
	if _, err := compile([]byte(src), Options{Lenient: false}); err != nil {
		t.Fatalf("strict compile still errors (stray $ not fixed): %v", err)
	}
	// Lenient: the \input body actually reaches the page.
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient compile: %v", err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, marker) {
		t.Fatalf("\\input body after \\maketitle was swallowed; got %q", txt)
	}
}

// Some amsart-derived classes (e.g. cip-v3-cls-submit) carry the \[ display-math
// patch UNGUARDED — no \ifx\csname[ \endcsname\relax around it — as
//
//	\def\@tempa#1$#2#3\@nil{\def\[{#1$#2\def\@currenvir{displaymath}#3}}%
//	\expandafter\@tempa\[\@nil
//
// With \[ a primitive rather than a $$-bearing macro, \@tempa's "#1 up to $" scan
// finds no $ and, before the fix, ran off the end of the file the patch lives in,
// past the file-end sentinel and into the main document, swallowing the entire body
// (the paper rendered 0 pages). The delimited-argument scanner must stop at the
// sentinel (TeX's "Runaway argument"), abandon the call, and let the rest process.
// Regression for arXiv 2607.05870 (cip-v3-cls-submit), which rendered 0 pages of 13.
func TestUnguardedDisplayMathPatchDoesNotSwallowBody(t *testing.T) {
	const marker = "BODYAFTERPATCH"
	dir := t.TempDir()
	incPath := filepath.Join(dir, "gotex_dmpatch.tex")
	inc := "\\makeatletter\n" +
		"\\def\\@tempa#1$#2#3\\@nil{\\def\\[{#1$#2\\def\\@currenvir{displaymath}#3}}%\n" +
		"\\expandafter\\@tempa\\[\\@nil\n" +
		"\\makeatother\n"
	if err := os.WriteFile(incPath, []byte(inc), 0o644); err != nil {
		t.Fatal(err)
	}
	// TeX \input uses forward slashes and no extension; ToSlash keeps t.TempDir
	// (an OS temp dir, not /tmp) valid on Windows.
	texPath := filepath.ToSlash(strings.TrimSuffix(incPath, ".tex"))
	src := "\\documentclass{article}\n\\begin{document}\nBEFOREMARK\n\\input{" +
		texPath + "}\n" + marker + " renders now.\n\\end{document}\n"
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("text before the \\input is missing; got %q", txt)
	}
	if !strings.Contains(txt, marker) {
		t.Fatalf("body after the unguarded \\[ patch was swallowed; got %q", txt)
	}
}

// The decoy must not disturb ordinary display math: \[ … \] still typesets through the
// primitive and leaves nothing dropped.
func TestOrdinaryDisplayMathStillWorks(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}Before \[ x^2 + y^2 = z^2 \] after\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("display math errored: %v", err)
	}
	// No math command (a \-keyed drop) may be recorded — the equation must render.
	// (A pre-existing text-mode "input" tally from the class-load pipeline is unrelated.)
	for k := range e.SkippedCommands() {
		if strings.HasPrefix(k, "\\") {
			t.Errorf("display math dropped a command: %v", e.SkippedCommands())
			break
		}
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "Before") || !strings.Contains(txt, "after") {
		t.Errorf("text around display math missing; got %q", txt)
	}
}
