// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A paragraph tells the page builder where it would rather not be cut, with the
// penalties TeX appends between its lines (tex.web §890). Nothing was appending
// any, so every gap between two lines was a free break: a paragraph's FIRST line
// could be left alone at the foot of a page and its LAST line alone at the head of
// the next, which is exactly what \clubpenalty and \widowpenalty price.
//
// The engine already read penalty nodes on the vertical list (pagebuilder.go);
// this asserts that the paragraph now puts them there, and WHERE.
func TestAParagraphPricesItsOwnLineBreaks(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	// Five lines at this measure, so club (after line 1) and widow (before the
	// last) land on different lines and can be told apart.
	if _, err := e.Run(`\hsize=120pt ` + rep("word ", 60) + `\par`); err != nil {
		t.Fatal(err)
	}
	// Walk the vertical list and record, for each penalty, how many lines precede
	// it — the position is the whole claim, not the count.
	after := map[int]int{} // lines before the penalty -> its value
	lines := 0
	for _, n := range e.mvl {
		switch v := n.(type) {
		case *boxNode:
			lines++
		case penaltyNode:
			after[lines] = v.penalty
		}
	}
	if lines < 4 {
		t.Fatalf("only %d lines — the witness is too short to tell club from widow", lines)
	}
	club, widow := texIntParams["clubpenalty"], texIntParams["widowpenalty"]
	if got := after[1]; got != club {
		t.Errorf("penalty after the FIRST line is %d, want \\clubpenalty = %d", got, club)
	}
	if got := after[lines-1]; got != widow {
		t.Errorf("penalty before the LAST line is %d, want \\widowpenalty = %d", got, widow)
	}
	// ⛔ And nothing anywhere else: \interlinepenalty is 0, so an interior break
	// costs nothing — which is what makes the two above mean something. A blanket
	// penalty on every line would satisfy them both and be wrong. TeX appends a
	// node only when the sum is non-zero (tex.web §890), so TWO is the whole list.
	if len(after) != 2 {
		t.Errorf("%d penalties on the list (%v), want exactly 2: after line 1 and before the last", len(after), after)
	}
	// ⛔ And none after the last line: what follows a paragraph is its own glue.
	if _, ok := after[lines]; ok {
		t.Errorf("a penalty was appended after the LAST line: %v", after)
	}
}

func rep(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
