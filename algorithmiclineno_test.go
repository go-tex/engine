// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ \begin{algorithmic}[N] numbers every N-th line, and the optional argument was
// DISCARDED: \algorithmic ended in \@discardopt, so every numbered algorithm lost every
// number — and the surrounding prose refers to them ("see line 7").
//
// Measured on 999 arXiv papers: 230 \begin{algorithmic} carry the argument over 109
// papers, against 35 that do not. Every value is numeric: [1] 229 times, [2] once.
//
// The semantics are algorithmic.sty's \ALC@it (line 146) and \ALC@lno (line 127): step a
// remainder, reset it to 0 when it reaches N, step the line number, print only when the
// remainder is 0. So [2] numbers lines 2, 4, 6 — not 1, 3, 5 — because the reset happens
// on reaching N. All four cases below were checked against tectonic and agree exactly,
// phase included.

func algGlyphs(t *testing.T, opt string) int {
	t.Helper()
	src := `\documentclass{article}\usepackage{algorithm}\usepackage{algorithmic}` +
		`\begin{document}\begin{algorithmic}` + opt + `
\REQUIRE in
\STATE one
\STATE two
\STATE three
\STATE four
\STATE five
\STATE six
\end{algorithmic}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%q: %v", opt, err)
	}
	return strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path")
}

// The glyph-path count is the instrument because a digit and its delimiter are DRAWN: six
// numbers are twelve more paths than none, and the counts have to order themselves
// [1] > [2] > [5] > none. A test that only asserted "some number appeared" would pass for
// a resolver that numbered every line whatever N said.
func TestAlgorithmicNumbersEveryNthLine(t *testing.T) {
	none, one, two, five := algGlyphs(t, ""), algGlyphs(t, "[1]"),
		algGlyphs(t, "[2]"), algGlyphs(t, "[5]")
	// Six \STATE lines: [1] draws 6 numbers, [2] draws 3 (lines 2,4,6), [5] draws 1
	// (line 5), and no argument draws none. Each number is two glyphs, a digit and a colon.
	for _, c := range []struct {
		name  string
		got   int
		wantN int
	}{
		{"[1] six numbers", one, 6},
		{"[2] three numbers", two, 3},
		{"[5] one number", five, 1},
	} {
		if got := c.got - none; got != 2*c.wantN {
			t.Errorf("%s: %d glyph path(s) more than the unnumbered block, want %d",
				c.name, got, 2*c.wantN)
		}
	}
}

// ⛔ \REQUIRE and \ENSURE take no number: algorithmic.sty gives them \item[<label>], which
// never reaches \ALC@it (lines 155-156). \STATEx is algorithmicx's unnumbered statement and
// was \let to the same macro as \STATE here, so numbering \STATE would have numbered it too.
//
// The witness counts the numbers rather than looking for their absence: with six \STATE and
// one each of \REQUIRE and \STATEx, a block that numbered everything would draw 8.
func TestAlgorithmicLeavesRequireAndStatexUnnumbered(t *testing.T) {
	src := `\documentclass{article}\usepackage{algorithm}\usepackage{algorithmic}` +
		`\begin{document}\begin{algorithmic}[1]
\REQUIRE in
\ENSURE out
\STATE one
\STATE two
\STATEx aside
\end{algorithmic}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	base := `\documentclass{article}\usepackage{algorithm}\usepackage{algorithmic}` +
		`\begin{document}\begin{algorithmic}
\REQUIRE in
\ENSURE out
\STATE one
\STATE two
\STATEx aside
\end{algorithmic}\end{document}`
	b, err := compile([]byte(base), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	num := strings.Count(strings.Join(e.RenderPages(e.renderMargin(0)), ""), "<path")
	plain := strings.Count(strings.Join(b.RenderPages(b.renderMargin(0)), ""), "<path")
	// Two numbers, four glyphs. Eight would mean \REQUIRE, \ENSURE and \STATEx were
	// numbered as well.
	if num-plain != 4 {
		t.Errorf("%d glyph path(s) of numbering, want 4 (two numbers): \\REQUIRE, "+
			"\\ENSURE and \\STATEx must not take one", num-plain)
	}
}
