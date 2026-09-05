// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// mu is the math unit: 18mu = 1em (TeXbook ch. 18). Unrecognised, its two letters
// went back into the input and were TYPESET — a class opening with
// \thinmuskip = 3mu / \medmuskip = 4mu / \thickmuskip = 5mu printed "mu mu mu" on
// its title page (oupau.cls:101-103).
func TestMuUnitIsScannedNotTypeset(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\thinmuskip = 3mu \medmuskip = 4mu AVANT APRES\par`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if strings.Contains(txt, "mu") {
		t.Errorf("the unit was typeset instead of scanned: %q", txt)
	}
	if !strings.Contains(txt, "AVANT") || !strings.Contains(txt, "APRES") {
		t.Errorf("the surrounding text was disturbed: %q", txt)
	}
}

// \usebox takes an undelimited argument (\long\def\usebox#1{\leavevmode\copy#1}),
// so a bare handle is as valid as a braced one. Only the braced form was read, and
// the bare form silently produced nothing.
func TestUseboxAcceptsABareHandle(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\newsavebox{\bx}\begin{lrbox}{\bx}CONTENU\end{lrbox}[\usebox\bx]\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "CONTENU") {
		t.Errorf("\\usebox with a bare handle produced nothing: %q", txt)
	}
}

// \lrbox need not be reached through \begin{lrbox}: a class may post it as another
// environment's opening. oupau.cls does — \def\abstract{\lrbox\absbox …} with
// \def\endabstract{\endlrbox} — and scanning for a literal \end{lrbox} then found
// none, ran to the end of the file, and set the abstract on the page instead of
// storing it.
func TestLrboxEndsAtTheEnvironmentThatOpenedIt(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\newsavebox{\ab}\def\abstract{\lrbox\ab}\def\endabstract{\endlrbox}` +
		`X\begin{abstract}RESUME\end{abstract}Y[\usebox\ab]\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if i, j := strings.Index(txt, "X"), strings.Index(txt, "Y"); i < 0 || j < 0 || strings.Contains(txt[i:j], "RESUME") {
		t.Errorf("the body leaked where \\begin{abstract} stood: %q", txt)
	}
	if !strings.Contains(txt, "RESUME") {
		t.Errorf("the body was not stored in the box: %q", txt)
	}
}
