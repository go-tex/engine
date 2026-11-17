package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ AN EXCEPTION SAYS WHERE A BREAK MAY GO, NOT THAT ONE MUST.
//
// tex.web:18145-18160 is a single block. The exception table is consulted first
// and a hit does `goto found` — to the SAME label the pattern scan falls through
// to — and `found` is where the limits are imposed:
//
//	found: for j:=0 to l_hyf-1 do hyf[j]:=0;
//	       for j:=0 to r_hyf-1 do hyf[hn-j]:=0
//
// So \lefthyphenmin and \righthyphenmin refuse an exception's breaks exactly as
// they refuse a pattern's.
//
// ⭐ hyphexception.go's header has said this since it was written. The code below
// it did not do it. The comment was RIGHT and the code was WRONG — the harder way
// round to notice, because a reader checking the rule meets it stated correctly
// and moves on.
func TestAnExceptionsBreaksObeyLeftAndRightHyphenMin(t *testing.T) {
	// A 9-letter word whose entry offers breaks after 1, 4 and 8 letters.
	const word = "abcdefghi"
	x := hyphExceptions{}
	x.add("a-bcd-efgh-i")
	pts, ok := x.lookup(word)
	if !ok {
		t.Fatal("the entry was not recorded at all; this test is about which of its breaks survive")
	}
	if len(pts) != 3 {
		t.Fatalf("the entry recorded %v, want three breaks (1, 4, 8) — the fixture is wrong, not the rule", pts)
	}

	for _, c := range []struct {
		name       string
		lmin, rmin int
		want       []int
	}{
		{
			// \lefthyphenmin=2 refuses the break after 1 letter;
			// \righthyphenmin=3 refuses the one after 8 of 9.
			//
			// ⚠ The surviving middle break is the CONTROL, and it is what makes
			// this row mean anything: a "fix" that dropped every exception break
			// would pass a test that only checked 1 and 8 were gone.
			name: "the TeX defaults refuse the first and the last",
			lmin: 2, rmin: 3,
			want: []int{4},
		},
		{
			// ⛔ The boundary from the other side. With the minima opened up the
			// SAME entry must offer all three — otherwise the rule is not
			// "\lefthyphenmin and \righthyphenmin apply" but "the ends are never
			// offered", which is a different bug wearing this fix's name.
			name: "opened up, every break the entry wrote is offered",
			lmin: 1, rmin: 1,
			want: []int{1, 4, 8},
		},
		{
			// A minimum larger than the word leaves nothing, and must not panic
			// or produce a negative bound.
			name: "minima wider than the word leave no break",
			lmin: 6, rmin: 6,
			want: nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := clipToMins(pts, len([]rune(word)), c.lmin, c.rmin)
			if len(got) != len(c.want) {
				t.Fatalf("clipToMins(%v, 9, %d, %d) = %v, want %v (tex.web:18159-18160)",
					pts, c.lmin, c.rmin, got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("break %d = %d, want %d", i, got[i], c.want[i])
				}
			}
		})
	}

	// ⛔ AND THE STORED ENTRY IS NOT TOUCHED. The same exception is looked up once
	// per occurrence of the word; a clip that wrote back into the slice the map
	// holds would narrow it permanently, so the second paragraph of a document
	// would hyphenate differently from the first for no reason a reader could see.
	if again, _ := x.lookup(word); len(again) != 3 {
		t.Errorf("the stored entry is now %v: clipping aliased the map's slice, and "+
			"the next occurrence of this word gets the narrowed list", again)
	}
}

// ⛔⛔ THE MINIMA ARE APPLIED WHEN THE WORD IS HYPHENATED, NOT WHEN THE ENTRY IS
// RECORDED, and that is observable: a document may write \hyphenation{…} and set
// \lefthyphenmin afterwards — a package declaring its exceptions before the class
// settles the language is the ordinary case.
//
// Clipping at `add` time would freeze whichever value happened to be in force
// when the entry was read, and this test is what refuses that implementation.
func TestTheMinimaAreReadWhenHyphenatingNotWhenTheEntryIsRecorded(t *testing.T) {
	// `photo-graphy` is the existing fixture's own entry: a break after 5 letters.
	// Read with \lefthyphenmin=1 it is legal; set with \lefthyphenmin=6 it is not.
	src := `\documentclass{article}` +
		`\lefthyphenmin=1 \righthyphenmin=1 \hyphenation{photo-graphy}` +
		`\lefthyphenmin=6 \righthyphenmin=6` +
		`\begin{document}\hsize=40pt ` +
		`A photography and a manuscript follow here.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if strings.Contains(got, "photo-") {
		t.Errorf("`photography` broke after five letters although \\lefthyphenmin was 6 "+
			"when the word was SET: the minima were read at \\hyphenation time rather "+
			"than at hyphenation time, so an entry declared before the class settles "+
			"the language keeps whatever was in force then: %q", got)
	}

	// ⛔ THE CONTROL, and without it this test passes for an engine that stopped
	// hyphenating. The same document with the minima left at 1 must still break
	// where the entry says.
	open := `\documentclass{article}` +
		`\lefthyphenmin=1 \righthyphenmin=1 \hyphenation{photo-graphy}` +
		`\begin{document}\hsize=40pt ` +
		`A photography and a manuscript follow here.\end{document}`
	e2, err := compile([]byte(open), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e2); !strings.Contains(got, "photo-") {
		t.Errorf("control: with the minima at 1 the entry's break is gone too, so the "+
			"row above proves nothing about WHEN they are read: %q", got)
	}
}
