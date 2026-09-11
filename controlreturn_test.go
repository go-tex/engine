// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// "control <return> = control <space>" — latex.ltx:560:
//
//	\def\^^M{\ } % control <return> = control <space>
//	\let\^^I\^^M % same for <tab>
//
// A line that ENDS with a backslash makes one, and a .bbl is full of them:
// revtex writes "{Sullivan}},\ and\⏎  \bibinfo {author}", putting the interword
// space of "and" AT the line break. Undefined, it was the most frequent unknown
// command in the whole corpus — 577 of them across 25 of 157 papers — and every
// one was a space that went missing: "andBeta", "Lachowiec}},and".
func TestAControlReturnIsAControlSpace(t *testing.T) {
	// The scanner sees the file's own terminator, so the same command is written
	// \<LF> in a unix file and \<CR> in a DOS one. A tab makes it too.
	for _, nl := range []string{"\n", "\r\n", "\t"} {
		src := "\\documentclass{article}\\begin{document}Alpha and\\" + nl + "Beta.\\end{document}"
		glued := "\\documentclass{article}\\begin{document}Alpha andBeta.\\end{document}"
		spaced := "\\documentclass{article}\\begin{document}Alpha and Beta.\\end{document}"
		n, g, s := glues(t, src), glues(t, glued), glues(t, spaced)
		if n != s {
			t.Errorf("%q: %d glues, a plain space gives %d", nl, n, s)
		}
		if n <= g {
			t.Errorf("%q: %d glues, no more than the glued-together %d", nl, n, g)
		}
	}
}

// glues compiles a document and counts the glue nodes on its page — the spaces
// pageChars cannot see, because it collects characters and a space is not one.
func glues(t *testing.T, src string) int {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	n := 0
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, x := range ns {
			switch v := x.(type) {
			case glueNode:
				n++
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	walk(e.parList)
	return n
}

// And the words are still there, whole: the command is a space, not a swallow.
func TestAControlReturnSwallowsNothing(t *testing.T) {
	e, err := compile([]byte("\\documentclass{article}\\begin{document}Alpha and\\\r\nBeta.\\end{document}"),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "Alpha") || !strings.Contains(got, "Beta") {
		t.Errorf("the page reads %q", got)
	}
}
