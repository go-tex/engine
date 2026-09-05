// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// An unclosed $ must not swallow a group boundary. It did: one arXiv paper has an
// unpaired $ inside an algorithmic block, the scan ate the block's \endgroup's, the
// groups desynchronised, and every later \item took the generic definition instead
// of the one \enumerate posts in its own group (#229).
func TestUnterminatedMathStopsAtEndgroup(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\begingroup $x+1 \endgroup APRES\par`); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "APRES") {
		t.Errorf("the text after the group was swallowed by the unclosed $: %q", txt)
	}
}

// The same at a paragraph end: \par inside maths is "Missing $ inserted" in TeX.
func TestUnterminatedMathStopsAtPar(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run("$x+1\n\nAPRES\\par"); err != nil {
		t.Fatal(err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "APRES") {
		t.Errorf("the next paragraph was swallowed: %q", txt)
	}
}

// A CLOSED formula is untouched, including one holding a group of its own.
func TestClosedMathIsUnaffected(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`avant $x_{1}+\mbox{texte}$ apres\par`); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "avant") || !strings.Contains(txt, "apres") {
		t.Errorf("a well-formed formula lost its surroundings: %q", txt)
	}
}
