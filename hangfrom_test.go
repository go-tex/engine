// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// Real LaTeX's \@hangfrom takes ONE argument (latex.ltx:12865), so the amsart
// family's second brace group is ordinary material and the \par that ends it is
// harmless (mathincs.cls:1177):
//
//	\@hangfrom{\hskip #3\relax\@svsec}{\interlinepenalty\@M #8\par}
//
// Ours takes that group as #2, and a \par in the argument of a macro that is not
// \long abandons the call (tex.web §392) — dropping the WHOLE heading, number and
// title both. On one corpus paper that was 44 headings.
func TestHangFromAcceptsAParInTheHeadingBody(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}\makeatletter`+
		`\@hangfrom{1.2.}{\interlinepenalty\@M Heading Text\par}`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := e.Diagnostics().RunawayArgs; n != 0 {
		t.Errorf("\\@hangfrom abandoned %d call(s); it must be \\long", n)
	}
	got := pageChars(e)
	for _, want := range []string{"1.2.", "Heading", "Text"} {
		if !strings.Contains(got, want) {
			t.Errorf("heading lost %q; page = %q", want, got)
		}
	}
}
