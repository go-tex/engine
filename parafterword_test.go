// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A control word absorbs the spaces after it AND the line's own end — but only
// that one end. tex.web §354 leaves the tokenizer in state S (skip blanks) after a
// control word, and §348 has an end-of-line in state S finish the line and start
// the next one in state N, where a blank line is \par.
//
// Absorbing EVERY end-of-line instead swallowed the paragraph break. Ending a
// paragraph with a macro is ordinary — \noindent, \ldots, a document's own \ack,
// an \end{...} reached through a macro — so this reached real text: beside
// tectonic on
//
//	X\relax
//
//	\ifvmode…
//
// the reference is in VERTICAL mode at the test and this engine was still in
// horizontal mode, with the paragraph never closed.
func TestBlankLineAfterAControlWordIsAParagraphBreak(t *testing.T) {
	for _, c := range []struct {
		nom, src string
		wantPar  bool
	}{
		{"control word then blank line", "X\\relax\n\n", true},
		{"plain text then blank line", "X\n\n", true},
		{"control word then ONE line end", "X\\relax\nY", false},
		{"control word, spaces, blank line", "X\\relax   \n\n", true},
		{"comment then blank line", "X%\n\n", true},
	} {
		e := New()
		e.SetFont(spMock{})
		var got []string
		e.LoadFormat(`\def\relax{}`)
		e.push(nil)
		// Read the source as tokens and record whether a \par surfaces.
		e.base, e.bpos = []rune(c.src), 0
		for {
			tk, ok := e.getNext()
			if !ok {
				break
			}
			if tk.cs_ {
				got = append(got, tk.cs)
			}
		}
		sawPar := false
		for _, n := range got {
			if n == "par" {
				sawPar = true
			}
		}
		if sawPar != c.wantPar {
			t.Errorf("%s: \\par present = %v, want %v (tokens %v)", c.nom, sawPar, c.wantPar, got)
		}
	}
}

// …and the effect a reader sees: the paragraph actually ends.
func TestParagraphEndsAfterAMacroAtLineEnd(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run("X\\relax\n\n\\ifvmode [VMODE]\\else [HMODE]\\fi\\message{done}")
	if err != nil {
		t.Fatal(err)
	}
	_ = out
	if e.inPar {
		t.Error("still in horizontal mode after a blank line following a control word")
	}
	if txt := mvlText(e.mvl); strings.Contains(txt, "HMODE") {
		t.Errorf("set %q, want the VMODE branch", txt)
	}
}
