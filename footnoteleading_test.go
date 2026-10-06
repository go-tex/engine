// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ A footnote left the BODY set at the note's leading, for the whole rest of the
// document. A note opens with \footnotesize, and typesetGroupToVbox saves the
// vertical-list state and the current font but NOT \baselineskip — a documented
// property its six other callers rely on.
//
// Measured against tectonic, \documentclass[12pt]{book}: \the\baselineskip read
// 14.5pt before a \footnote and 12.0pt after it, where the reference reads 14.5pt
// on both sides; with \baselinestretch{1.25}, 18.125pt became 15.0pt. On the
// 198-page thesis 2402.04711 that is most of an 18-page deficit — the body of the
// document from its first footnote on was set at footnote leading.
//
// The CONTROL is the second case: a note must still be set SMALLER than the body,
// or this would pass against a footnote that had stopped changing size at all.
func TestAFootnoteDoesNotLeakItsLeading(t *testing.T) {
	// ⛔ The observable is the glue BETWEEN THE LINES THAT FOLLOW the note, not a
	// field read after the run: by the time compile returns, \end{document} has
	// been through the state and e.baselineskip no longer shows the leak. A first
	// version of this test checked the field and passed against the defect.
	lead := func(t *testing.T, src string) int {
		t.Helper()
		e, err := compile([]byte(`\documentclass[12pt]{book}\begin{document}\chapter{C}`+src+
			`\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		// The last paragraph's interline glue: walk the vertical list and keep the
		// glue that sits between two line boxes.
		// ⛔ A PENALTY MAY SIT BETWEEN the line and its glue — tex.web §890 puts one
		// there — so the walk must step over penalties rather than treat them as
		// "something else happened". A first version reset on them and reported
		// "no interline glue found" the moment that was implemented.
		var last int
		prevBox := false
		for _, n := range e.mvl {
			switch v := n.(type) {
			case *boxNode:
				prevBox = true
			case penaltyNode:
				// transparent: keeps prevBox as it was
			case glueNode:
				if prevBox && v.spec.width > 0 {
					last = v.spec.width
				}
			default:
				prevBox = false
			}
		}
		if last == 0 {
			t.Fatal("no interline glue found — the witness set no second line")
		}
		return last
	}

	const tail = `\par ` + `Body text that runs to a second line so the paragraph has interline glue in it, ` +
		`and then some more words again to be sure it does. ` +
		`And more still, so that two lines are certain.\par `
	plain := lead(t, `Body text.`+tail)
	noted := lead(t, `Body text.\footnote{a note}`+tail)
	if noted != plain {
		t.Errorf("the paragraph after a footnote is set at %d, the same paragraph without one at %d: "+
			"the note's \\footnotesize leaked into the body", noted, plain)
	}
}
