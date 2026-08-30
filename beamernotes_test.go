// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"testing"
)

// This engine renders no speaker notes, and beamer's note machinery emits PAGES.
// \setbeameroption{show notes on second screen} sets \beamer@notes and makes every
// frame produce a note page beside the slide, which LaTeX hands to pgfpages to merge
// onto ONE physical page. With no merging, each frame came out three pages instead of
// one — and the worst talk in the reference set rendered 21 pages against tectonic's 7.
//
// The switch is turned off at \begin{document}, after the preamble's \setbeameroption
// has had its say.
func TestNotesOnASecondScreenDoNotMultiplyPages(t *testing.T) {
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("needs a texmf tree with the real beamer.cls")
	}
	for _, c := range []struct {
		nom, option string
		want        int
	}{
		{"sans option", "", 2},
		{"show notes", `\setbeameroption{show notes}`, 2},
		{"second écran", `\setbeameroption{show notes on second screen}`, 2},
	} {
		e, err := compile([]byte(`\documentclass{beamer}
`+c.option+`
\begin{document}
\begin{frame}{A}un\end{frame}
\begin{frame}{B}deux\end{frame}
\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if got := len(e.Pages()); got != c.want {
			t.Errorf("%s: %d pages, want %d (two frames are two pages)", c.nom, got, c.want)
		}
	}
}
