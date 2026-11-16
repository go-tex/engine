// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// penaltiesBetweenLines returns the penalties a paragraph contributed, in order,
// together with the number of line boxes, so an assertion can say WHICH gap each
// one sits in.
func penaltiesBetweenLines(t *testing.T, preamble, body string) (pens []int, boxes int) {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}` + preamble + body +
		`\end{document}`); err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, n := range e.mvl {
		switch v := n.(type) {
		case *boxNode:
			boxes++
		case penaltyNode:
			pens = append(pens, v.penalty)
		}
	}
	return pens, boxes
}

// ⛔⛔ THE ENGINE INSERTED NO PENALTY BETWEEN THE LINES OF A PARAGRAPH.
//
// tex.web:17445-17455 appends one after every line but the last:
//
//	if cur_line+1<>best_line then
//	  begin pen:=inter_line_penalty;
//	  if cur_line=prev_graf+1 then pen:=pen+club_penalty;
//	  if cur_line+2=best_line then pen:=pen+final_widow_penalty;
//	  if disc_break then pen:=pen+broken_penalty;
//
// \clubpenalty, \widowpenalty, \interlinepenalty and \brokenpenalty were entries
// in texparams.go that a document could set and read back and that reached no
// page: a widow line cost nothing, a club line cost nothing, and \samepage
// (\interlinepenalty\@M) was a command with no effect at all.
//
// ⛔ THE VALUES ARE CHOSEN TO BE DISTINGUISHABLE, which is the whole design of
// this test: with the defaults (\clubpenalty and \widowpenalty both 150) a sum
// cannot be told from either term. Here club=7, widow=11, interline=3, so every
// combination is a different number and a wrong clause cannot hide behind a
// right total.
//
// ⭐ AND \brokenpenalty IS ZEROED HERE, after my own hand-computed expectation
// was WRONG and the code was right: the first run reported the middle gap as
// 103, not 3, because that line ends at a hyphen and 103 is 3 + the default
// \brokenpenalty of 100 — tex.web's fourth clause, `if disc_break then
// pen:=pen+broken_penalty`, which my table had forgotten. Zeroing it isolates
// the three clauses this test is about; the fourth has its own test below, with
// its own distinguishable value, rather than being left to whichever line the
// breaker happens to hyphenate.
func TestPenaltiesBetweenTheLinesOfAParagraph(t *testing.T) {
	// \hsize is small enough that the body breaks into four lines, so there is a
	// first line, a penultimate line and a middle line that is neither.
	const pre = `\hsize=120pt \parindent=0pt \clubpenalty=7 \widowpenalty=11 ` +
		`\interlinepenalty=3 \brokenpenalty=0 `
	const body = `Typography affects every line of every paper and a measure that is wrong is wrong on every line.\par`

	pens, boxes := penaltiesBetweenLines(t, pre, body)
	t.Logf("%d line box(es), penalties %v", boxes, pens)

	if boxes < 4 {
		t.Fatalf("the body came out in %d line(s); this test needs at least 4 so that a "+
			"middle line exists which is neither first nor penultimate", boxes)
	}

	// One penalty per gap, and none after the final line.
	wantGaps := boxes - 1
	var got []int
	for _, p := range pens {
		if p != -10000 { // the \par's own forced break at the end of the document
			got = append(got, p)
		}
	}
	if len(got) != wantGaps {
		t.Errorf("%d penalt(ies) for %d line(s): tex.web appends one after every line "+
			"but the LAST, so %d are expected. Got %v", len(got), boxes, wantGaps, got)
	}

	// ⭐ The arithmetic, clause by clause: first gap carries club, the one before
	// the last carries widow, the middle ones carry the interline penalty alone.
	if len(got) == wantGaps && wantGaps >= 3 {
		if want := 3 + 7; got[0] != want {
			t.Errorf("first gap = %d, want %d (interlinepenalty 3 + clubpenalty 7): "+
				"the club clause is `cur_line=prev_graf+1`", got[0], want)
		}
		if want := 3 + 11; got[wantGaps-1] != want {
			t.Errorf("last gap = %d, want %d (interlinepenalty 3 + widowpenalty 11): "+
				"the widow clause is `cur_line+2=best_line`", got[wantGaps-1], want)
		}
		if want := 3; got[1] != want {
			t.Errorf("second gap = %d, want %d (interlinepenalty alone): a middle line "+
				"is neither the first nor the penultimate", got[1], want)
		}
	}
}

// ⭐ "Breaking between lines of a two-line paragraph gets BOTH club-line and
// widow-line penalties" — tex.web:17440-17441, said in the prose and easy to
// lose when porting the four clauses as alternatives. With one break the single
// gap is both the first and the penultimate, so the node carries the sum.
func TestATwoLineParagraphGetsBothClubAndWidow(t *testing.T) {
	const pre = `\hsize=120pt \parindent=0pt \clubpenalty=7 \widowpenalty=11 ` +
		`\interlinepenalty=3 \brokenpenalty=0 `
	const body = `Typography affects every line of it.\par`

	pens, boxes := penaltiesBetweenLines(t, pre, body)
	t.Logf("%d line box(es), penalties %v", boxes, pens)
	if boxes != 2 {
		t.Fatalf("the body came out in %d line(s), not 2: this test is about the "+
			"two-line case and cannot measure it otherwise", boxes)
	}
	var got []int
	for _, p := range pens {
		if p != -10000 {
			got = append(got, p)
		}
	}
	if len(got) != 1 {
		t.Fatalf("a two-line paragraph has ONE gap, got %d penalt(ies): %v", len(got), got)
	}
	if want := 3 + 7 + 11; got[0] != want {
		t.Errorf("the single gap = %d, want %s = %d: both the club clause and the widow "+
			"clause fire on it, and the four terms ADD rather than choose",
			got[0], "3+7+11", want)
	}
}

// ⛔ THE FOURTH CLAUSE: a line that ends AT a discretionary carries
// \brokenpenalty — tex.web:17450, `if disc_break then pen:=pen+broken_penalty`.
//
// ⭐ This test exists because that clause caught ME: the four-line test above
// first reported a middle gap of 103 where my hand-computed table said 3, and
// 103 is 3 + the default \brokenpenalty of 100. The code was right and my
// arithmetic was wrong, which is the better way round but only if the clause
// then gets a witness of its own instead of staying an accident.
//
// ⚠ It carries a CONTROL, because the whole test rests on the breaker actually
// hyphenating something: with \brokenpenalty zeroed, the same paragraph must
// produce a STRICTLY SMALLER total. A run where nothing was hyphenated would
// give two equal totals and the test would pass while measuring nothing.
func TestALineBrokenAtADiscretionaryCarriesBrokenPenalty(t *testing.T) {
	const body = `Typography affects every line of every paper and a measure that is wrong is wrong on every line.\par`
	const common = `\hsize=120pt \parindent=0pt \clubpenalty=0 \widowpenalty=0 \interlinepenalty=0 `

	// ⚠ The comparable quantity is the number of LINE BOXES, not the number of
	// penalty nodes: tex.web appends a node only `if pen<>0`, so the zeroed run
	// legitimately emits none. My first version compared emitted nodes and
	// refused its own measurement for that reason.
	sum := func(pre string) (total, boxes int) {
		pens, boxes := penaltiesBetweenLines(t, pre, body)
		for _, p := range pens {
			if p != -10000 {
				total += p
			}
		}
		return total, boxes
	}

	withBroken, boxesA := sum(common + `\brokenpenalty=13 `)
	without, boxesB := sum(common + `\brokenpenalty=0 `)
	t.Logf("broken=13: total %d over %d line(s); broken=0: total %d over %d line(s)",
		withBroken, boxesA, without, boxesB)

	if boxesA != boxesB {
		t.Fatalf("the two runs broke the paragraph into different numbers of lines "+
			"(%d against %d), so their totals are not comparable", boxesA, boxesB)
	}
	// The control: with every other penalty at 0 and \brokenpenalty at 0 too,
	// there is nothing to add, so the total must be 0.
	if without != 0 {
		t.Fatalf("with all four penalties at 0 the total is %d, not 0: something else is "+
			"contributing and this test cannot attribute a difference", without)
	}
	if withBroken == 0 {
		t.Errorf("\\brokenpenalty=13 added nothing: no line was charged for ending at a " +
			"discretionary (tex.web:17450). Either the clause is missing or the breaker " +
			"hyphenated nothing — the control above rules out the second only for the " +
			"zero case")
	}
	if withBroken%13 != 0 {
		t.Errorf("the total with \\brokenpenalty=13 is %d, which is not a multiple of 13: "+
			"something other than the discretionary clause contributed", withBroken)
	}
}
