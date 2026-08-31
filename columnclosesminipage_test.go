// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
	"testing"
)

// beamer's \column always begins by running \beamer@colclose, which holds the
// PREVIOUS column's closer (beamerbaseframecomponents.sty:281-283):
//
//	\newcommand<>\beamer@columncom[2][\beamer@colmode]{%
//	  \beamer@colclose
//	  \def\beamer@colclose{\end{minipage}\hfill\end{actionenv}\ignorespaces}%
//
// so a \column met while collecting a minipage's body IS that minipage's \end —
// deferred, and invisible to a raw scan because \column takes arguments while the
// narrow expandsToEnd rule only sees parameterless macros. Read raw, the first
// column swallowed its sibling and everything after the frame: a talk with two
// columns in a [fragile] frame rendered ONE page instead of its seven.
func TestTwoColumnsInAFragileFrame(t *testing.T) {
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("needs a texmf tree with the real beamer.cls")
	}
	e, err := compile([]byte(`\documentclass{beamer}
\begin{document}
\begin{frame}{Un}A\end{frame}
\begin{frame}[fragile]{Deux}
\begin{columns}\column{0.5\textwidth}X\column{0.5\textwidth}Y\end{columns}
\end{frame}
\begin{frame}{Trois}C\end{frame}
\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := len(e.Pages()); got != 3 {
		t.Errorf("%d page(s), want 3: le cadre à deux colonnes a avalé la suite", got)
	}
	if got := pageChars(e); !strings.Contains(got, "C") {
		t.Errorf("la page porte %q — le troisième cadre manque", got)
	}
}
