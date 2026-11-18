// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ A HEADING STARTS AT THE MARGIN, AND ONE SPACE TOKEN MOVED EVERY ONE OF THEM.
//
// \@gxhead's body read
//
//	\noindent{#1 #2}\hskip-\@tempskipa
//
// and the space between #1 and #2 is a real space TOKEN: #1 is a parameter, not a
// control word, so nothing absorbs it. Expanded in horizontal mode at the very
// start of the heading it became interword glue, and every sectioning heading in
// every document began that far right of the margin.
//
// LaTeX has no such space — ltsect.dtx:418 is \@hangfrom{\hskip #3\relax\@svsec},
// where #3 is \@startsection's indent argument (\z@ in the standard classes).
//
// Measured against tectonic, leftmost glyph in points from the page edge:
//
//	heading           font    reference   before   after
//	\section          14.35   133.77      140.95   133.77
//	\subsection       11.96   133.77      139.75   133.77
//	\subsubsection     9.96   133.77      138.86   133.77
//	body lines         9.96   133.77      133.77   133.77
//
// The right edges land too (185.66/185.67, 190.40/190.41, 206.51/206.51), so the
// whole line is in place and not merely its start.
//
// ⚠ WHY A SHAPE ASSERTION AND NOT A RENDERED ONE. The defect is a horizontal
// displacement of 5-7pt, and this repository's fidelity metric is not positioned
// to see that (#716). The rendered witness that does see it is a PDF comparison
// against tectonic, which is not available inside the suite. So this asserts the
// token that caused it, and the figures above record what the rendered comparison
// said — the same bargain TestGxheadEndsTheHeadingParagraphInsideTheGroup makes,
// and for the same reason.
func TestAHeadingIsNotPrecededByAStraySpace(t *testing.T) {
	// Both branches of \@gxhead: display headings (positive afterskip) and run-in
	// ones (negative). The defect was in both, so the guard is on both.
	for _, bad := range []string{`{#1 #2}`, `{#1 #2\par}`} {
		if strings.Contains(LaTeX2eClassLead, bad) {
			t.Errorf("\\@gxhead contains %q: the space between #1 and #2 is a space "+
				"TOKEN — #1 is a parameter, so nothing absorbs it — and it becomes "+
				"interword glue at the head of every heading (ltsect.dtx:418 has none)",
				bad)
		}
	}
	// ⛔ AND THE CONTROL: the style group must still be there. A fix that deleted
	// the braces instead of the space would pass the two rows above and set every
	// heading in body type.
	for _, want := range []string{`{#1#2}`, `{#1#2\par}`} {
		if !strings.Contains(LaTeX2eClassLead, want) {
			t.Errorf("\\@gxhead no longer contains %q: the style group is what puts "+
				"the heading in its own font", want)
		}
	}
}
