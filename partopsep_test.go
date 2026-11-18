// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// glueBefore sums the natural width of the vertical glue standing between the
// (n-1)th and nth top-level box of a vertical list: the room a list opened
// with, read from the list rather than from the page.
func glueBefore(list []node, nth int) int {
	seen, sum := 0, 0
	for _, n := range list {
		switch c := n.(type) {
		case glueNode:
			sum += c.spec.width
		case *boxNode:
			seen++
			if seen == nth {
				return sum
			}
			sum = 0
		}
	}
	return -1
}

// ⭐ \partopsep JOINS \@topsepadd ONLY IN VERTICAL MODE.
//
// ltlists.dtx:596-603 — \@trivlist sets \@topsepadd to \topsep and then adds
// \partopsep only \ifvmode; after running text it takes the \else branch and
// adds nothing. \itemize, \enumerate and \description each opened with \par
// FIRST, which forces vertical mode, and then added \partopsep unconditionally.
//
// Measured against tectonic on two lists differing ONLY in their starting mode:
// the horizontal-mode one was 1.99pt out at EACH end — \@topsepadd is set once
// and spent at both — and the vertical-mode one already agreed.
// go-tex/engine#685.
//
// Each row asserts the DIFFERENCE between the two modes, never either absolute
// value: an absolute assertion passes just as well when both are wrong by the
// same amount, which is precisely the state before the fix.
//
// ⛔⛔ AND THE ZERO GUARD IS NOT CEREMONY. \partopsep is 0pt until a class sets
// it, so without \documentclass the difference is 0 and "0 == 0" passes against
// the defect. A first version of this file ran one case without a class: the
// \itemize row reported the zero and the trivlist row PASSED on it, green
// against a bug. Every row loads the class and every row refuses a zero
// \partopsep.
func TestPartopsepJoinsOnlyAListThatStartsAParagraph(t *testing.T) {
	// \description takes a term: a bare \item in one is an error, not a defect.
	for _, tc := range []struct{ env, item string }{
		{"itemize", `\item Bravo.`},
		{"enumerate", `\item Bravo.`},
		{"description", `\item[Term] Bravo.`},
		{"trivlist", `\item Bravo.`},
	} {
		body := `\begin{` + tc.env + `}` + tc.item + `\end{` + tc.env + `}Charlie.`
		doc := func(lead string) string {
			return `\documentclass{article}\begin{document}` + lead + body + `\end{document}`
		}
		horiz := runEI(t, doc(`Alfa runs straight on.`))
		vert := runEI(t, doc(`Alfa ends its paragraph.\par `))

		want := horiz.namedSkip("partopsep").width
		if want == 0 {
			t.Errorf("%s: \\partopsep is 0pt, so this row cannot tell the two modes "+
				"apart and would pass against the defect", tc.env)
			continue
		}
		// Box 1 is "Alfa…", box 2 is the item's line, so the glue before box 2 is
		// what the list opened with.
		gh, gv := glueBefore(horiz.mvl, 2), glueBefore(vert.mvl, 2)
		if gh < 0 || gv < 0 {
			t.Errorf("%s: could not find the item's line (horizontal %d, vertical %d)",
				tc.env, gh, gv)
			continue
		}
		if got := gv - gh; got != want {
			t.Errorf("%s: a list opening in vertical mode is %.2fpt taller than one "+
				"opening in horizontal mode; want exactly \\partopsep = %.2fpt "+
				"(horizontal %.2fpt, vertical %.2fpt).\n"+
				"A difference of 0 means \\partopsep was added in BOTH modes — "+
				"\\@trivlist (ltlists.dtx:596-603) adds it only \\ifvmode.",
				tc.env, spToPt(got), spToPt(want), spToPt(gh), spToPt(gv))
		}
	}
}
