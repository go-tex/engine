// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// An isolated expansion (\edef, \message, and the maths retry path) pushes its list
// with a sentinel behind it and reads until that sentinel. An unbalanced conditional
// inside the list skips forward looking for its \fi and swallows the sentinel with
// it — and from there the loop reads the CALLER's pending lists, consuming the rest
// of the document into the expansion.
//
// Measured on a paper whose formulas carry class internals: 78% of its glyphs left
// the page. The list's own depth on the input stack is what tells: falling below it
// means our list is gone, and the expansion stops rather than eating what follows.
func TestExpandListDoesNotEatWhatFollowsIt(t *testing.T) {
	e, err := NewDocument(Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// The caller's pending input, behind the expansion.
	suite := []tok{chTok('A', catLetter), chTok('P', catLetter), chTok('R', catLetter)}
	e.push(suite)
	// An \iffalse with no \fi: its skip runs past the sentinel.
	e.expandList([]tok{csTok("iffalse")})
	var got []rune
	for {
		tk, ok := e.getNext()
		if !ok {
			break
		}
		if !tk.cs_ {
			got = append(got, tk.ch)
		}
	}
	if string(got) != "APR" {
		t.Errorf("après l'expansion il reste %q, want %q: la liste du dessous a été mangée",
			string(got), "APR")
	}
}
