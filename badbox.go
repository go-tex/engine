// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// Overfull and underfull lines, counted and reported.
//
// ⛔ THE ENGINE ALREADY KNEW AND NEVER SAID. hasBadLine (paragraph.go) tests
// exactly this and has been used to DECIDE WHICH PASS TO RUN since line
// breaking was written; nothing ever reached a caller. A document whose lines
// run into the margin compiled silently.
//
// That is the same shape as the \hangindent defect closed in #760: a condition
// the engine detects, acts on, and keeps to itself. It is worse here, because
// an overfull line is the single most common thing a real TeX complains about
// and the reason a reader opens the log at all.
//
// ⭐ AND IT WOULD HAVE MADE #768 VISIBLE WITHOUT AN INVESTIGATION. The witness
// there is one paragraph at \hsize=45pt where the reference sets 5 lines and we
// set 8. Its log says why, in four lines:
//
//	Overfull \hbox (18.1pt too wide) in paragraph at lines 12--13
//	Overfull \hbox (13.92pt too wide) in paragraph at lines 12--13
//	Overfull \hbox (14.3pt too wide) in paragraph at lines 12--13
//	Overfull \hbox (18.16pt too wide) in paragraph at lines 12--13
//
// The reference overhangs the margin by up to 18pt rather than hyphenate again;
// we hyphenate again and stay inside. Ours said nothing at all, so the first
// reading of that difference — "our breaking is looser" — was the opposite of
// what was happening, and it took reading tectonic's log to see it.
//
// # What is counted
//
// tex.web reports an overfull box when the line needs more shrink than it has
// (§660, badness of a shrinking box past its limit) and an underfull one when
// its badness exceeds \hbadness (§663). This engine's line breaker returns a
// RATIO per line, which carries the same information in the form it already
// has: below -1 the line is overfull, and a large positive ratio is a line
// stretched far past comfort.
//
// ⚠ THE THRESHOLDS ARE THE BREAKER'S, NOT \hbadness AND \hfuzz. Those two are
// accept-and-ignore parameters here (texparams.go), so consulting them would be
// reading half-implemented state — the mistake #767 refused to make with
// \defaulthyphenchar. The counts therefore answer "how many lines did the
// breaker itself consider bad", which is a true statement about this engine,
// rather than pretending to be TeX's exact warning criteria.

// badBoxes is the running tally of lines the breaker judged bad.
type badBoxes struct {
	overfull  int
	underfull int
	// worst is the largest overflow seen, IN POINTS past the margin — the number
	// tectonic prints — because that is what tells a reader whether this is a
	// hairline or a line halfway across the page.
	worst float64
}

// lineOverflowSP returns how far a line's own content runs past the measure, in
// scaled points, and 0 when it fits. It is the ONE place that answers "is this
// line overfull".
//
// ⛔ ASK THE WIDTH, BECAUSE THE RATIO BELONGS TO WHOEVER BUILT THE Line. That
// is this file's lesson, and the precise shape of it is worth keeping: the
// ratio is right when the breaker computed it and MEANINGLESS when something
// else synthesised the Line. greedyLines (lastresort.go) never sets Ratio at
// all, and neither did the one-line fallback it replaced, so every line they
// produce carries 0 — which is why `Ratio < -1` counted nothing while a line
// hung 133pt into the margin. The width does not care who built the Line.
func lineOverflowSP(list []node, ln Line, targetSP int) int {
	if ln.Start < 0 || ln.End > len(list) || ln.Start >= ln.End {
		return 0
	}
	if natural := hpackSP(list[ln.Start:ln.End], packNatural, 0).width; natural > targetSP {
		return natural - targetSP
	}
	return 0
}

// anyLineOverflows reports whether any line runs past the measure. It is the
// precondition breakSegment's ragged first pass keeps: §852 makes every line
// of a ragged paragraph GOOD, but a line that does not fit is not good at any
// \rightskip, and TeX's first pass does not accept one.
//
// ⚠ IT HAS NO WITNESS. Removing it entirely leaves the whole suite green, and
// on every ragged document traced it either agreed with `Ratio < -1` or was
// never reached, because a measure narrow enough to overfull a line makes the
// first pass fail outright. It is kept because it is the condition tex.web
// states, not because a document was found that needs it — and the comment says
// so rather than implying a defect that was never observed.
func anyLineOverflows(list []node, lines []Line, lineWidthPt float64) bool {
	target := ptToSP(lineWidthPt)
	for _, ln := range lines {
		if lineOverflowSP(list, ln, target) > 0 {
			return true
		}
	}
	return false
}

// noteBadLines records what a finished paragraph's lines look like.
//
// ⚠ IT IS CALLED WITH THE LINES THAT WERE ADOPTED, not with every pass's
// attempt. The earlier passes exist precisely to find something better; their
// rejected answers are not what the document got and counting them would
// report badness the reader never sees.
//
// ⛔⛔ IT MEASURES THE LINE, IT DOES NOT ASK FOR ITS RATIO — and the first
// version of this did, which made it miss the worst case there is. Measured on
// a \texttt token at \hsize=40pt: the line came out 173.25pt wide, which is
// 133.25pt past the margin and is exactly the number tectonic prints for the
// same document ("Overfull \hbox (133.25pt too wide)"). The reference reported
// it three times and this counted zero. lineOverflowSP above holds the
// measurement and the reason.
func (e *Engine) noteBadLines(list []node, lines []Line, lineWidthPt float64) {
	target := ptToSP(lineWidthPt)
	for _, ln := range lines {
		if ln.Start < 0 || ln.End > len(list) || ln.Start >= ln.End {
			continue
		}
		switch over := lineOverflowSP(list, ln, target); {
		case over > 0:
			e.badBox.overfull++
			if pt := spToPt(over); pt > e.badBox.worst {
				e.badBox.worst = pt
			}
		case ln.Ratio > 100:
			// A line stretched far past comfort still has glue, so the ratio is
			// the right question for THIS half and the width is not: an
			// underfull line is narrower than its measure by construction.
			e.badBox.underfull++
		}
	}
}
