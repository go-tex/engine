// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The lrbox environment is the environment form of \sbox: it stores its body in a
// save-box register instead of setting it on the page, so the content appears where
// the later \usebox places it — NOT inline where the \begin{lrbox} sits. Without the
// handler the body leaked into the running text and the register stayed void.
func TestLrboxSavesBodyForUsebox(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// X and Y bracket the lrbox; BODY must NOT appear between them (no leak), but must
	// appear at the \usebox after Y.
	src := `\newsavebox{\bx}X\begin{lrbox}{\bx}BODY\end{lrbox}Y\usebox{\bx}Z`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if strings.Contains(txt, "XBODY") {
		t.Errorf("lrbox body leaked inline (before \\usebox): %q", txt)
	}
	if !strings.Contains(txt, "XY") {
		t.Errorf("prose around lrbox broken (expected X immediately before Y): %q", txt)
	}
	if !strings.Contains(txt, "BODY") {
		t.Errorf("lrbox body not recovered by \\usebox: %q", txt)
	}
	// The whole order: X, then Y, then the used box BODY, then Z.
	if txt != "XYBODYZ" {
		t.Errorf("order = %q, want XYBODYZ (body placed at \\usebox)", txt)
	}
}

// An lrbox whose register operand is not a valid save-box handle still consumes its
// body (no leak) and simply stores nothing.
func TestLrboxInvalidHandleNoLeak(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `A\begin{lrbox}{\notabox}HIDDEN\end{lrbox}B`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if strings.Contains(txt, "HIDDEN") {
		t.Errorf("lrbox with an invalid handle leaked its body: %q", txt)
	}
	if txt != "AB" {
		t.Errorf("order = %q, want AB", txt)
	}
}
