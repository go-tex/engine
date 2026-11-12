// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// firstGlueWidth returns the width of the glue that opens the gap ABOVE the
// block, for a document shaped "AVANT\n\n<block>\n\nAPRES".
//
// ⛔ IT SKIPS PAST THE FIRST BOX, AND THAT IS NOT A DETAIL. It used to return
// the first glue on the list wider than a hair, which was the block's surround
// only for as long as NOTHING preceded the first box. Once \prevdepth is 0 at a
// document body's start — which is what the reference does, see #699 — the
// first line of AVANT takes interline glue, and that glue is wider than a hair.
// The helper then answered 4.84pt for both of the tests below, which is neither
// \medskipamount nor \topsep+\partopsep and is not about listings at all.
//
// A helper whose heuristic silently starts measuring a different quantity is
// worse than a wrong expectation: both tests would have been "fixed" by writing
// 4.84 into them, and the thing they exist to pin would have stopped being
// pinned. So the rule is now the one the name claims: the gap above the block
// is the first glue AFTER the paragraph that precedes it.
func firstGlueWidth(t *testing.T, src string) int {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	seenBox := false
	for _, n := range e.mvl {
		if _, ok := n.(*boxNode); ok {
			seenBox = true
			continue
		}
		if g, ok := n.(glueNode); ok && seenBox && g.spec.width > unity {
			return g.spec.width
		}
	}
	t.Fatal("no vertical glue found after the first box: the document did not take the " +
		"shape this helper assumes, so it is measuring nothing")
	return 0
}

// lstlisting does NOT build on \trivlist. listings.sty:1694 defaults its aboveskip
// and belowskip to \medskipamount and applies them as plain \vspace (:1724, :1777),
// so the surround is 6pt — not the list separation \topsep+\partopsep, which is
// 10pt at the 10pt class. Measured against tectonic on one block of six lines, the
// baseline-to-baseline gap into the block was 22.0pt against the reference's 17.93;
// with \medskipamount it is 18.0.
func TestLstlistingSurroundIsMedskipNotTrivlistSep(t *testing.T) {
	const doc = `\documentclass{article}\usepackage{listings}\begin{document}` +
		"AVANT\n\n" + `\begin{lstlisting}` + "\ncode();\n" + `\end{lstlisting}` +
		"\n\nAPRES" + `\end{document}`
	got := firstGlueWidth(t, doc)
	if want := 6 * unity; got != want {
		t.Errorf("lstlisting surround = %d sp (%.2fpt), want \\medskipamount %d sp (6pt)",
			got, float64(got)/float64(unity), want)
	}
}

// fancyvrb — which minted and the Code/CodeInput environments sit on — DOES use the
// list separation: fancyvrb.sty:665 sets \@topsepadd=\FancyVerbVspace (default
// \topsep) and :666 adds \partopsep. So verbatim keeps 10pt, and this pins that the
// listings change did not leak onto it.
func TestVerbatimSurroundStaysTheListSeparation(t *testing.T) {
	const doc = `\documentclass{article}\begin{document}` +
		"AVANT\n\n" + `\begin{verbatim}` + "\ncode();\n" + `\end{verbatim}` +
		"\n\nAPRES" + `\end{document}`
	got := firstGlueWidth(t, doc)
	if want := 10 * unity; got != want {
		t.Errorf("verbatim surround = %d sp (%.2fpt), want \\topsep+\\partopsep %d sp (10pt)",
			got, float64(got)/float64(unity), want)
	}
}
