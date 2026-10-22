// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \maketitle defers to the CLASS's \@maketitle when it has one, which is what
// article.cls does (classes.dtx, and texmf/article.cls:203-219 here):
//
//	\if@twocolumn
//	  \ifnum \col@number=\@ne \@maketitle \else \twocolumn[\@maketitle]\fi
//	\else \newpage \global\@topnum\z@ \@maketitle \fi
//
// Without it the class's own title layout never ran, and the generic fallback set
// \@author inside a \centerline — an \hbox that nothing line-breaks. On 2310.11517
// (paper.cls, PNAS) that hbox came out 1787.88pt wide on a 612pt sheet, carrying one
// author and four affiliations (go-tex/engine#629). 5104 papers of the arXiv pool
// define their own \@maketitle, 58 of the 154 measured.
//
// ⛔ The witness must use a class the engine has NO definitions for. With
// \documentclass{article} the embedded texmf/article.cls installs its own \maketitle,
// which already calls \@maketitle — so the test passes before the fix as well as
// after, and asserts nothing. That is how this test first "passed"; it was run
// against the unfixed file and only this spelling fails there.
func TestMaketitleUsesTheClassOwnMaketitle(t *testing.T) {
	got := unknownClassPage(t, `\makeatletter\def\@maketitle{GOTEXCLASSTITLE}\makeatother`+
		`\title{GOTEXTITRE}\author{GOTEXAUTEUR}\maketitle`)
	if !strings.Contains(got, "GOTEXCLASSTITLE") {
		t.Errorf("le \\@maketitle de la classe n'a pas tourné: %q", got)
	}
	// ⛔ And it REPLACES the generic block rather than running beside it: a title set
	// twice is the other way this can look like it works.
	if strings.Contains(got, "GOTEXTITRE") {
		t.Errorf("le bloc générique a tourné aussi: %q", got)
	}
}

// ⚠ The fallback is KEPT. A document whose class defines no \@maketitle must still
// get its title, author and date — that is most of this test suite.
func TestMaketitleWithoutAClassMaketitleStillSetsTheTitle(t *testing.T) {
	got := unknownClassPage(t, `\title{GOTEXTITRE}\author{GOTEXAUTEUR}\date{GOTEXDATE}\maketitle`)
	for _, want := range []string{"GOTEXTITRE", "GOTEXAUTEUR", "GOTEXDATE"} {
		if !strings.Contains(got, want) {
			t.Errorf("le bloc générique a perdu %q: %q", want, got)
		}
	}
}

// unknownClassPage renders a document whose class the engine has no definitions for,
// so the generic \maketitle of latex.go is the one in play — as it is for every paper
// that ships a class of its own, which is what this change is about.
func unknownClassPage(t *testing.T, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{gotexnosuchclass}\begin{document}`+body+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return mvlText(e.mvl)
}
