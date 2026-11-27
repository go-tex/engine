// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

func boxHeightOf(t *testing.T, body string) int {
	t.Helper()
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\documentclass{article}
\usepackage{tabularray}
\begin{document}
\global\setbox0\vbox{` + body + `}
\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	b := e.box[0]
	if b == nil {
		t.Fatal("box 0 is empty")
	}
	return b.height + b.depth
}

// Inside a box a table is packed whole and never paged: the rows go into the box's
// own list as one block, exactly as a tabular does.
func TestTblrInsideABoxStaysInline(t *testing.T) {
	a := boxHeightOf(t, `x \begin{tblr}{ll}a&b\\c&d\\\end{tblr}`)
	b := boxHeightOf(t, `x\par\begin{tblr}{ll}a&b\\c&d\\\end{tblr}`)
	if a >= b {
		t.Errorf("table in a box height %d, want less than %d (it sits on the line, not below it)", a, b)
	}
}
