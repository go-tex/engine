// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// authblk's \affil[n]{text} typesets NOTHING where it stands: it accumulates, and
// the class's \@maketitle sets the list through \@author (authblk.sty:148-171).
// Declared as a stub taking a mandatory #1, it grabbed the "[" and the rest leaked
// into the running text.
func TestAffilReachesTheTitleBlock(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\title{TITRE}\author{Nom}\affil[1,5]{LABOUN}\affil[2]{LABODEUX}\maketitle\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"TITRE", "Nom", "LABOUN", "LABODEUX"} {
		if !strings.Contains(txt, want) {
			t.Errorf("%q missing from the title block: %q", want, txt)
		}
	}
	// The bracket is the note number, not text.
	if strings.Contains(txt, "1,5]") || strings.Contains(txt, "2]") {
		t.Errorf("the optional argument leaked: %q", txt)
	}
	// Order: the affiliation follows the author.
	if i, j := strings.Index(txt, "Nom"), strings.Index(txt, "LABOUN"); i < 0 || j < 0 || i > j {
		t.Errorf("the affiliation is not after the author: %q", txt)
	}
}

// The braced form carries no note and must work the same.
func TestAffilWithoutABracket(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\title{T}\author{Nom}\affil{LABO}\maketitle\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "LABO") {
		t.Errorf("the braced form produced nothing: %q", txt)
	}
}

// \affil with no \author at all must not leave a dangling row separator.
func TestAffilWithoutAnAuthor(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\title{T}\affil[1]{LABO}\maketitle\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "LABO") {
		t.Errorf("the affiliation was lost: %q", txt)
	}
}
