// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"testing"
)

// ⛔ THE WHOLE SIZE LADDER AGAINST THE REFERENCE'S OWN NUMBERS.
//
// Latin Modern is optically sized and this engine used to scale one file to
// every size. Only \normalsize was right. The widths below are \the\wd of
// \hbox{MM} at each step of an article's ladder, read out of tectonic 0.17.0:
//
//	            reference   before      after
//	\tiny       11.890      9.170       11.890      -22.9 % -> exact
//	\scriptsize 14.392      12.838      14.392      -10.8 %
//	\footnotesize 15.568    14.672      15.568       -5.8 %
//	\small      16.956      16.506      16.956       -2.7 %
//	\normalsize 18.340      18.340      18.340        the one that was right
//	\large      21.528      22.008      21.528       +2.2 %
//	\Large      25.834      26.410      25.834       +2.2 %
//	\LARGE      29.376      31.692      29.376       +7.9 %
//	\huge       35.258      38.037      35.258       +7.9 %
//
// This pins the FILE CHOICE rather than those widths, because a width is also a
// function of the glyph and of rounding, and a test that pins nine numbers goes
// red for nine different reasons. What is asserted is the table: the size→file
// map tulmr.fd declares. The widths above are why it matters and are recorded
// here rather than asserted.
func TestTheOpticalTableIsTheReferencesOwn(t *testing.T) {
	for _, tc := range []struct {
		fam  opticalFamily
		at   float64
		want string
	}{
		// Regular — tulmr.fd TU/lmr/m/n, the full eight-entry table.
		{lmRomanRegularSizes, 5, "lmroman5-regular"},
		{lmRomanRegularSizes, 5.49, "lmroman5-regular"},
		{lmRomanRegularSizes, 5.5, "lmroman6-regular"},
		{lmRomanRegularSizes, 6.5, "lmroman7-regular"},
		{lmRomanRegularSizes, 7, "lmroman7-regular"},
		{lmRomanRegularSizes, 7.5, "lmroman8-regular"},
		{lmRomanRegularSizes, 8.5, "lmroman9-regular"},
		{lmRomanRegularSizes, 9, "lmroman9-regular"},
		{lmRomanRegularSizes, 9.5, "lmroman10-regular"},
		{lmRomanRegularSizes, 10, "lmroman10-regular"},
		// ⛔ 10.95 IS THE ONE THAT MATTERS MOST: an 11pt class sets its body at
		// \@xipt = 10.95, and the range is HALF-OPEN (latex.ltx:8642-8659), so it
		// takes lmroman10 and not lmroman12. An inclusive upper bound would get
		// every 11pt document in the corpus wrong.
		{lmRomanRegularSizes, 10.95, "lmroman10-regular"},
		{lmRomanRegularSizes, 11, "lmroman12-regular"},
		{lmRomanRegularSizes, 14.4, "lmroman12-regular"},
		{lmRomanRegularSizes, 15, "lmroman17-regular"},
		{lmRomanRegularSizes, 20.74, "lmroman17-regular"},

		// Bold — the same through 11, then 12 for ever: upstream ships no
		// lmroman17-bold, so a \Huge\bfseries heading scales lmroman12-bold in
		// BOTH engines. That is their table, not a gap here.
		{lmRomanBoldSizes, 5, "lmroman5-bold"},
		{lmRomanBoldSizes, 9, "lmroman9-bold"},
		{lmRomanBoldSizes, 10.95, "lmroman10-bold"},
		{lmRomanBoldSizes, 11, "lmroman12-bold"},
		{lmRomanBoldSizes, 24.88, "lmroman12-bold"},

		// Italic — starts at 7: there is no lmroman5-italic or 6-italic.
		{lmRomanItalicSizes, 5, "lmroman7-italic"},
		{lmRomanItalicSizes, 7.49, "lmroman7-italic"},
		{lmRomanItalicSizes, 8.5, "lmroman9-italic"},
		{lmRomanItalicSizes, 10.95, "lmroman10-italic"},
		{lmRomanItalicSizes, 11, "lmroman12-italic"},

		// Typewriter — tulmtt.fd TU/lmtt/m/n.
		{lmMonoSizes, 5, "lmmono8-regular"},
		{lmMonoSizes, 9, "lmmono9-regular"},
		{lmMonoSizes, 10.95, "lmmono10-regular"},
		{lmMonoSizes, 12, "lmmono12-regular"},
	} {
		_, got := tc.fam.pick(tc.at)
		if got != tc.want {
			t.Errorf("at %gpt: %s, want %s", tc.at, got, tc.want)
		}
	}
}

// ⭐ AND THE LADDER ITSELF, end to end, because a correct table wired up wrongly
// would pass the test above and change nothing on the page.
//
// The expected widths are tectonic's, to a hundredth of a point. A hundredth
// and not exact: the two engines round a sum of advances at different moments,
// and pinning the last digit would make this a test of rounding rather than of
// which file was opened. Before the optical table, \tiny was out by 2.7pt —
// three hundred times this tolerance.
func TestTheSizeLadderMatchesTheReference(t *testing.T) {
	for _, tc := range []struct {
		cmd  string
		want float64
	}{
		{`\tiny`, 11.890},
		{`\scriptsize`, 14.392},
		{`\footnotesize`, 15.568},
		{`\small`, 16.956},
		{`\normalsize`, 18.340},
		{`\large`, 21.528},
		{`\Large`, 25.834},
		{`\LARGE`, 29.376},
		{`\huge`, 35.258},
	} {
		// \makeatletter AFTER \begin{document}: @ is catcode 12 in the body, and
		// \begin{document} itself runs \makeatother, so a preamble \makeatletter
		// is undone before the probe is read.
		src := `\documentclass{article}\begin{document}\makeatletter{` + tc.cmd +
			`\setbox0\hbox{MM}\gotex@probewd}\end{document}`
		var got float64
		probed := false
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("%s: buildEngine: %v", tc.cmd, err)
		}
		e.prim("gotex@probewd", func(e *Engine) {
			got, probed = float64(e.box[0].width)/float64(unity), true
		})
		if _, err := e.Run(src); err != nil {
			t.Fatalf("%s: run: %v", tc.cmd, err)
		}
		if !probed {
			t.Fatalf("%s: the probe never ran — this case measured NOTHING", tc.cmd)
		}
		if math.Abs(got-tc.want) > 0.01 {
			t.Errorf("%s: \\wd\\hbox{MM} = %.3fpt, tectonic gives %.3fpt (off by %+.3f)",
				tc.cmd, got, tc.want, got-tc.want)
		}
	}
}

// A font the HOST supplied must never be re-faced into Latin Modern at another
// size. Only the files this package embeds carry a table.
func TestAHostsOwnFontIsNeverGivenAnOpticalTable(t *testing.T) {
	if opticalOf(nil) != nil {
		t.Error("nil bytes were given an optical table")
	}
	own := append([]byte(nil), lmRomanRegular...) // a byte-identical COPY
	if opticalOf(own) != nil {
		t.Error("a host's own copy of Latin Modern was given our table: identity, not " +
			"content, is what says a file is one of ours")
	}
	if opticalOf(lmRomanRegular) == nil {
		t.Error("our own embedded regular has no table, so this test proves nothing " +
			"about the negative cases above")
	}
}
