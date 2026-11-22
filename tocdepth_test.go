// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// tocDepthCounts compiles a document with the given preamble line and returns how many
// times the section and the subsection titles are typeset: once in the body, and once more
// in the contents list when the entry is listed.
func tocDepthCounts(t *testing.T, pre string) (sec, sub int) {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}
`+pre+`
\begin{document}
\tableofcontents
\section{SECTITLEQ}
\subsection{SUBTITLEQ}
\end{document}`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	return strings.Count(text, "SECTITLEQ"), strings.Count(text, "SUBTITLEQ")
}

// ⭐ \tableofcontents ignored tocdepth: toc.go had no depth at all, so \setcounter{tocdepth}{1}
// still listed every subsection. article.cls's \l@section drops the lines deeper than the
// counter (article.cls:529), at the value in force when the list is read. A paper that sets
// the depth to 1 got a longer contents list than the reference — +1 page in 2402.04711.
func TestTOCDepthDropsDeeperEntries(t *testing.T) {
	if sec, sub := tocDepthCounts(t, ``); sec != 2 || sub != 2 {
		t.Errorf("default depth 3: section %d, subsection %d times, want 2 and 2", sec, sub)
	}
	if sec, sub := tocDepthCounts(t, `\setcounter{tocdepth}{1}`); sec != 2 || sub != 1 {
		t.Errorf("tocdepth 1: section %d, subsection %d times, want 2 (listed) and 1 (body only)", sec, sub)
	}
	if sec, sub := tocDepthCounts(t, `\setcounter{tocdepth}{0}`); sec != 1 || sub != 1 {
		t.Errorf("tocdepth 0: section %d, subsection %d times, want 1 and 1", sec, sub)
	}
}
