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

// The same fault in the emulation stubs, each checked against the package that
// defines the real command. A \par inside the argument abandons the call
// (tex.web §392), and for these that means a subfigure, a title or a footnote body
// is dropped — or, for a gobbler, LEAKED onto the page instead of swallowed.
func TestEmulationStubsAreLongWhereTheRealCommandIs(t *testing.T) {
	for _, c := range []struct {
		name, src, want string
	}{
		// subfig.sty:350,354 — \sf@@subfloat and \sf@@@subfloat are \long.
		{"subfloat", `\begin{figure}\subfloat[cap]{body A\par body B}\end{figure}`, "body"},
		// latex.ltx:12747 — \DeclareRobustCommand\title, unstarred, so \long.
		{"title", `\title{Line A\par Line B}\maketitle`, "Line"},
		// latex.ltx:13187 — \@footnotetext is \long. The gobbler must swallow the
		// whole body; abandoning the call leaks it into the text.
		{"footnotetext", `X\footnotetext[1]{note A\par note B}Y`, "X"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}`+
				c.src+`\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			d := e.Diagnostics()
			if d.RunawayArgs != 0 {
				t.Errorf("%d call(s) abandoned: %v", d.RunawayArgs, d.RunawayMacros)
			}
			if got := pageChars(e); !strings.Contains(got, c.want) {
				t.Errorf("page = %q, want it to contain %q", got, c.want)
			}
		})
	}
}

// \author is the deliberate exception: latex.ltx:12748 declares it with
// \DeclareRobustCommand*, and \@star@or@long (latex.ltx:1173) makes the STARRED form
// \relax, not \long. A \par in an author block is a runaway in real LaTeX too, so
// ours must report one rather than quietly accept it — three of the corpus's
// abandoned calls are that error, and they are the documents', not ours.
func TestAuthorIsNotLongLikeRealLaTeX(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\author{A\par B}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := e.Diagnostics().RunawayArgs; n != 1 {
		t.Errorf("RunawayArgs = %d, want 1: \\author is \\DeclareRobustCommand*", n)
	}
}
