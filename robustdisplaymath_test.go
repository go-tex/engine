// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
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
	if err := os.WriteFile("/tmp/gotex_maketitle_input.tex", []byte(marker+" renders now\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("/tmp/gotex_maketitle_input.tex")

	src := "\\documentclass{amsart}\n\\title{A Title}\\author{An Author}\n" +
		"\\begin{document}\n\\maketitle\n\\input{/tmp/gotex_maketitle_input}\nMore body after input.\n\\end{document}\n"

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
