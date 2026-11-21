// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"

	"github.com/go-tex/texmf"
)

// A rounded block in a [fragile] frame must not take the next frame with it
// (go-tex/engine#866).
//
// The rounded inner theme (here through Warsaw) draws a block with
// beamerboxesrounded, which opens \begin{minipage} in its begin code; the
// matching \end{minipage} comes only from \endbeamerboxesrounded, run by
// \end{block}. In a [fragile] frame the body is read back from a file, so the
// minipage's body scan reaches the end of that file having stored \end{block},
// an \end the rewind cannot follow. It used to read on into the document, to
// EOF: the frames after it were gathered raw, handed back with three groups
// unbalanced, and the next frame's body collection never found its \end{frame}.
// That frame, and everything after it, was lost.
//
// The two controls differ from the witness by one line each, and both rendered
// two pages before the fix: that is what pins the loss on the fragile file
// boundary rather than on the theme or the block.
func TestARoundedBlockInAFragileFrameKeepsTheNextFrame(t *testing.T) {
	withTeXMF(t, texmf.Beamer)
	const frames = `\begin{document}
\begin{frame}%s
\frametitle{FRAGTITLE}
\begin{block}{BLOCKTITLE}
FRAGBODY
\end{block}
\end{frame}
\begin{frame}
\frametitle{LASTTITLE}
LASTBODY
\end{frame}
\end{document}`
	for _, c := range []struct{ name, theme, opt string }{
		{"rounded block, fragile frame", `\usetheme{Warsaw}`, `[fragile]`},
		{"control: no [fragile]", `\usetheme{Warsaw}`, ``},
		{"control: no theme", ``, `[fragile]`},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := `\documentclass{beamer}` + "\n" + c.theme + "\n" +
				strings.Replace(frames, "%s", c.opt, 1)
			e, err := compile([]byte(src), Options{Lenient: true})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := len(e.Pages()); got != 2 {
				t.Errorf("%d page(s), want 2: one per frame", got)
			}
			got := pageChars(e)
			for _, want := range []string{"FRAGBODY", "LASTTITLE", "LASTBODY"} {
				if !strings.Contains(got, want) {
					t.Errorf("%q is missing from the output", want)
				}
			}
		})
	}
}
