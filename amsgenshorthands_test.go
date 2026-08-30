// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// amsgen.sty lets \@xp be \expandafter and \@nx be \noexpand, and every
// AMS-derived class then writes its \csname assignments through them. Undefined,
// \@xp is skipped and the \expandafter never happens, so the assignment hits
// \csname instead of the name it was building — and the rest spills onto the
// page. This is that spill, reduced: the page must carry the x and nothing else.
func TestAmsgenExpandafterShorthands(t *testing.T) {
	src := `\documentclass{article}\begin{document}\makeatletter
\@xp\gdef\csname r@tocindent\endcsname{0pt}\@xp\def\csname zz\endcsname{}x\end{document}`
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := pageChars(e); got != "x" {
		t.Errorf("la page porte %q, want %q", got, "x")
	}
	// \@nx is \noexpand: inside an \edef it must keep the following command whole.
	e2, err := compile([]byte(`\documentclass{article}\begin{document}\makeatletter
\def\inner{NON}\edef\outer{\@nx\inner}\def\inner{OUI}\outer\end{document}`), Options{})
	if err != nil {
		t.Fatalf("compile (\\@nx): %v", err)
	}
	if got := pageChars(e2); got != "OUI" {
		t.Errorf("\\@nx: la page porte %q, want %q (le \\def suivant doit gagner)", got, "OUI")
	}
}
