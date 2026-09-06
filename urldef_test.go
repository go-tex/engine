// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// url.sty's \urldef{\name}\url{text} DEFINES \name and typesets nothing. Undefined,
// \urldef was skipped and the \url after it ran on the spot: the URL appeared where
// the definition stood and the later \name produced nothing — both halves wrong.
func TestUrldefDefinesAndDoesNotTypeset(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `X\urldef{\lien}\url{http://exemple.org/a}Y[\lien]\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	// glyphString descends through the link node the URL is set inside.
	txt := glyphString(e.mvl)
	i, j := strings.Index(txt, "X"), strings.Index(txt, "Y")
	if i < 0 || j < 0 || strings.Contains(txt[i:j], "exemple.org") {
		t.Errorf("the URL was typeset at the definition (between X and Y): %q", txt)
	}
	if !strings.Contains(txt, "exemple.org") {
		t.Errorf("the defined macro produced nothing: %q", txt)
	}
}

// Every real use writes the BARE form — \urldef\tempurl\url{…} is what a
// bibliography style emits, and \urldef is \def\urldef#1{…}, so a single token is
// the argument. Reading only the braced form defined nothing at all.
func TestUrldefAcceptsABareName(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`X\urldef\tempurl\url{http://exemple.org/c}Y[\tempurl]\par`); err != nil {
		t.Fatal(err)
	}
	txt := glyphString(e.mvl)
	i, j := strings.Index(txt, "X"), strings.Index(txt, "Y")
	if i < 0 || j < 0 || strings.Contains(txt[i:j], "exemple.org") {
		t.Errorf("the URL was typeset at the definition: %q", txt)
	}
	if !strings.Contains(txt, "exemple.org") {
		t.Errorf("the bare form defined nothing: %q", txt)
	}
}

// A bare \url is untouched: it still typesets where it stands.
func TestPlainUrlStillTypesets(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`X\url{http://exemple.org/b}Y\par`); err != nil {
		t.Fatal(err)
	}
	if txt := glyphString(e.mvl); !strings.Contains(txt, "exemple.org") {
		t.Errorf("a plain \\url stopped typesetting: %q", txt)
	}
}
