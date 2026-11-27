// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

func tblrRows(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString("r & x\\\\\n")
	}
	return b.String()
}

func pagesOf(t *testing.T, src string) int {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	return len(e.Pages())
}

// A longtable taller than a page runs over to the next one: it was assembled as
// ONE box and a box is never split, so 60 rows kept a page of them and lost the
// rest. tectonic sets the witness on 2 pages (2407.18384's Notation table is the
// corpus case: 3 pages here against 5 there).
func TestLongtableBreaksAcrossPages(t *testing.T) {
	src := `\documentclass{article}
\usepackage{longtable}
\begin{document}
Before.
\begin{longtable}{|l|l|}
\hline
` + tblrRows(60) + `\end{longtable}
After.
\end{document}`
	if got := pagesOf(t, src); got < 2 {
		t.Errorf("pages = %d, want at least 2", got)
	}
}

// tabularray's longtblr was unknown: its body was set as running text and its
// \DefTblrTemplate calls leaked "captiondefault contheaddefault" onto the page.
func TestLongtblrIsATable(t *testing.T) {
	src := `\documentclass{article}
\usepackage{tabularray}
\begin{document}
\DefTblrTemplate{caption}{default}{}
\begin{longtblr}[caption = {Notation}]{colspec = {|t{0.15\linewidth}|m{0.5\linewidth}|}, hlines}
` + tblrRows(60) + `\end{longtblr}
\end{document}`
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if n := len(e.Pages()); n < 2 {
		t.Errorf("pages = %d, want at least 2", n)
	}
	if txt := pageText(e); strings.Contains(txt, "captiondefault") {
		t.Errorf("template name leaked onto the page: %q", txt)
	}
}
