// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ THE GLUE IS POSED WITH \hskip, NOT \spaceskip, AND THAT IS NOT A STYLE
// CHOICE. \spaceskip is a \newskip register in this engine and not the base
// interword glue — spacefactor.go:104-109 says so — so a fixture that sets it
// gets the FONT's space and not the one it asked for. The first draft of this
// test did exactly that and produced eighteen rows in which \looseness changed
// nothing, which I nearly published as evidence that the parameter does not
// reach the breaker. It does; the fixture was measuring the wrong glue.
//
// With spMock every glyph is 5pt (charnode_test.go), so `ww` is 10pt on a 50pt
// measure: four to the line at 40+3x4 = 52pt shrinking 6, three at 38pt
// stretching 12 of the 16 they have. Both decent, which is what gives \looseness
// something to choose between.
func loosenessFixture() string {
	words := strings.TrimSuffix(
		strings.Repeat(`ww\hskip 4pt plus 8pt minus 2pt `, 12),
		`\hskip 4pt plus 8pt minus 2pt `)
	return `\parindent=0pt \hsize=50pt \tolerance=5000 \adjdemerits=10000 ` + words + `\par`
}

func loosenessLines(t *testing.T, src string) int {
	t.Helper()
	e := New()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, p := range e.Pages() {
		for _, nd := range pageTextList(p) {
			if b, ok := nd.(*boxNode); ok && !b.textBlock {
				n++
			}
		}
	}
	return n
}

// ⛔⛔ \looseness REACHES THE BREAKER, which it never did before.
//
// tex.web:17197-17237: when the last break has been tried, every feasible way of
// ending the paragraph is an active node; TeX takes the fewest-demerits one and
// then, only when \looseness is non-zero, looks for one whose line count differs
// from it in the direction asked for. It is a CHOICE among breakings that exist,
// not a second search.
//
// The register was declared in texparams.go and read by nothing — #754 item (C),
// go-typeset/linebreak#12 item (7) — so a document that set it was obeyed in
// silence. The library gained the choice, and the state it needs, in v1.12.0.
func TestLoosenessReachesTheBreaker(t *testing.T) {
	base := loosenessLines(t, loosenessFixture())
	loose := loosenessLines(t, `\looseness=1 `+loosenessFixture())
	t.Logf("looseness 0: %d line(s); looseness 1: %d", base, loose)
	if base != 3 {
		t.Fatalf("the fixture sets in %d lines, want 3 — the row below is about "+
			"which OTHER line count is reachable", base)
	}
	if loose != base+1 {
		t.Errorf("\\looseness=1 gave %d line(s), want %d: the longer ending is "+
			"feasible here and tex.web:17221-17237 must choose it", loose, base+1)
	}
}

// ⛔ THE CONTROL, and it is the one that matters for the corpus: \looseness=0 is
// what every document that never mentions the parameter gets, and it must be
// exactly what this engine did before. Without this row the change could have
// moved every paragraph and nothing would have said so.
func TestLoosenessZeroIsTheOldBehaviour(t *testing.T) {
	if a, b := loosenessLines(t, loosenessFixture()),
		loosenessLines(t, `\looseness=0 `+loosenessFixture()); a != b {
		t.Errorf("\\looseness=0 changed the paragraph: %d line(s) against %d", b, a)
	}
}

// ⛔ AND A LOOSENESS NOTHING CAN SATISFY IS NOT A FAILURE. tex.web:17203 keeps
// what the final pass reached; the paragraph must still come out, not vanish.
// \looseness=5 asks for five more lines than any feasible ending offers.
func TestAnUnreachableLoosenessStillSetsTheParagraph(t *testing.T) {
	if n := loosenessLines(t, `\looseness=5 `+loosenessFixture()); n == 0 {
		t.Error("\\looseness=5 produced no lines: an unmatched looseness must leave " +
			"the final pass's own breaking standing (tex.web:17203)")
	}
}
