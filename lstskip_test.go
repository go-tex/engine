// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// firstGlueWidth returns the width of the first glue on the main vertical list
// wider than a hair, which for these documents is the gap opened above the block.
func firstGlueWidth(t *testing.T, src string) int {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range e.mvl {
		if g, ok := n.(glueNode); ok && g.spec.width > unity {
			return g.spec.width
		}
	}
	t.Fatal("no vertical glue found")
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
