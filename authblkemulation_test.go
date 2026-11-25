// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// authblk's \author[marks]{name} APPENDS to the author list; the kernel's \author
// replaces. With no authblk.sty to run, a four-author paper kept its last author only
// (2407.09954); tectonic sets all four, the marks as superscripts, and the affiliations
// in \Affilfont.
func TestAuthblkAuthorsAccumulate(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}
\usepackage{authblk}
\author[1,2]{Alice Aa}
\author[1]{Bob Bb}
\author[2]{Carol Cc}
\author[1]{Dave Dd}
\affil[1]{First Institute}
\affil[2]{Second Institute}
\title{T}
\date{}
\begin{document}
\maketitle
\end{document}
`), Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := pageText(e)
	for _, want := range []string{"Alice", "Bob", "Carol", "Dave", "FirstInstitute", "SecondInstitute"} {
		if !strings.Contains(got, want) {
			t.Errorf("the title block lost %q: %q", want, got)
		}
	}
	if i, j := strings.Index(got, "Alice"), strings.Index(got, "Dave"); i < 0 || j < i {
		t.Errorf("authors out of order: %q", got)
	}
	if !strings.Contains(got, "and") {
		t.Errorf("no \\Authand before the last author: %q", got)
	}
}

// \Affilfont is the font of every affiliation line (authblk.sty \Affilfont).
func TestAuthblkAffilfontApplies(t *testing.T) {
	sizes := lineGlyphSizes(t, `\documentclass{article}
\usepackage{authblk}
\renewcommand\Affilfont{\tiny}
\author{Alice Aa}
\affil{Institute}
\title{T}
\date{}
\begin{document}
\maketitle
\end{document}
`)
	found := false
	for _, s := range sizes {
		if s < 6 {
			found = true
		}
	}
	if !found {
		t.Errorf("no \\tiny line among %v", sizes)
	}
}
