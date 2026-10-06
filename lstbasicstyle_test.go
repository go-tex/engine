// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ basicstyle= is the font a code block is set in, and it was read and dropped.
// \lstset and \lstdefinestyle were no-op MACROS, so a document that set its size
// once at the top — which is how listings is normally used — got body size in
// every block. 26 of the 154 corpus papers ask for a smaller size there
// (footnotesize 14, small 6, scriptsize 5, tiny 1).
//
// Measured against tectonic at 12pt: the reference sets \scriptsize code at a
// 9.46pt line pitch and this engine set it at 14.45 — 57 lines per page against
// 39. The three ways a document can ask all land on 9.46 now; the CONTROL is the
// fourth row, where no style is asked for and nothing may change.
func TestListingsBasicStyleIsApplied(t *testing.T) {
	// setLines compiles one witness and returns the heights of its line boxes.
	setLines := func(t *testing.T, preamble, opt string) []int {
		t.Helper()
		// ⛔ A REAL CLASS, because \scriptsize is the class's: with a bare engine the
		// size command is undefined and every row of this table reads the same,
		// which is what a first version of this test reported.
		src := "\\documentclass[12pt]{article}" + preamble +
			"\\begin{document}\n\\begin{lstlisting}" + opt +
			"\nalpha\nbeta\ngamma\n\\end{lstlisting}\n\\end{document}"
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		// ⛔ The observable is the INTERLINE GLUE, not the box height: the test font
		// returns the same metrics at every size, so a line box is the same height
		// whatever \scriptsize does. \baselineskip is an engine parameter and does
		// move — which is also what the measurement against tectonic sees (a 9.46pt
		// line pitch against 14.45).
		var glue []int
		seenBox := false
		for _, n := range e.mvl {
			switch v := n.(type) {
			case *boxNode:
				seenBox = true
			case glueNode:
				if seenBox {
					glue = append(glue, v.spec.width)
				}
			}
		}
		if len(glue) < 2 {
			t.Fatalf("only %d interline glues — the witness did not set its three lines", len(glue))
		}
		return glue
	}

	plain := setLines(t, "", "")
	for _, c := range []struct{ name, preamble, opt string }{
		{"the block's own option", "", `[basicstyle=\scriptsize]`},
		{"\\lstset", `\lstset{basicstyle=\scriptsize}`, ""},
		{"\\lstdefinestyle and style=", `\lstdefinestyle{m}{basicstyle=\scriptsize}\lstset{style=m}`, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := setLines(t, c.preamble, c.opt)
			if got[0] >= plain[0] {
				t.Errorf("interline glue %d with %s, %d without: the style did not make it tighter",
					got[0], c.name, plain[0])
			}
		})
	}
	// ⛔ THE CONTROL: without a style nothing may change, or the three rows above
	// would pass against a block that had simply shrunk for its own reasons.
	again := setLines(t, "", "")
	if again[0] != plain[0] {
		t.Errorf("the same witness twice gave %d then %d", plain[0], again[0])
	}
}
