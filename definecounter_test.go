// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \@definecounter{name} is the counter-creating half of \newcounter (latex.ltx).
// A class that builds its own \newtheorem calls it directly — llncs.cls does — and
// undefined it read no argument, so the counter was never allocated: every theorem
// numbered 0 and the class's own \c@proposition leaked into the prose as text.
func TestDefineCounterAllocatesTheCounter(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\makeatletter\@definecounter{prop}\makeatother` +
		`\stepcounter{prop}\stepcounter{prop}[\theprop]\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "[2]") {
		t.Errorf("counter did not step: %q, want [2]", txt)
	}
}

// It takes NO optional argument, so a [ that follows belongs to the document and
// must survive — binding it to \newcounter's scanner would swallow it.
func TestDefineCounterLeavesAFollowingBracket(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\makeatletter\@definecounter{gam}\makeatother[SUITE]\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "[SUITE]") {
		t.Errorf("the bracket after \\@definecounter was eaten: %q", txt)
	}
}
