// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// beamer is emulated: \documentclass{beamer} turns each frame into a page, renders
// titles/blocks/itemize, shows overlays statically, and gobbles themes — so a real
// talk renders as a sequence of slides instead of collapsing to one page.
func TestBeamerFramesBecomePages(t *testing.T) {
	src := `\documentclass{beamer}
\usetheme{Metropolis}
\title{Talk}\author{Me}\date{2026}
\begin{document}
\frame{\titlepage}
\section{Intro}
\begin{frame}{First}
Hello.
\begin{itemize}\item<1-> One\item Two\end{itemize}
\end{frame}
\begin{frame}[fragile]{Second}
\begin{block}{A block}\alert{Important} content.\end{block}
\only<2>{Hidden-until-2 shown statically.}
\begin{columns}\column{0.5\textwidth}Left\column{0.5\textwidth}Right\end{columns}
\end{frame}
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	// title page + 2 frames ⇒ at least 3 pages.
	if n := len(e.Pages()); n < 3 {
		t.Fatalf("beamer produced %d pages, want ≥3 (frames must become pages)", n)
	}
	// content of the frames must actually render (glyph paths present).
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if !strings.Contains(svg, "<path") {
		t.Error("beamer frames rendered no glyph content")
	}
	// theme/overlay/column machinery must not leak as dropped commands.
	for _, k := range []string{"frame", "frametitle", "usetheme", "only", "alert", "column", "block", "pause"} {
		if e.SkippedCommands()[k] != 0 {
			t.Errorf("beamer command %q was dropped: %v", k, e.SkippedCommands())
		}
	}
}
