// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// LaTeX's own page styles declare \@oddhead; fancyhdr's six fields are the
// exception, not the rule. article.cls:
//
//	\def\ps@headings{%
//	  \let\@oddfoot\@empty
//	  \def\@oddhead{{\slshape\rightmark}\hfil\thepage}%
//	  \let\@mkboth\markboth
//	  \def\sectionmark##1{\markright{\MakeUppercase{…##1}}}}
//
// doPagestyle already RAN \ps@name, so the \def's took effect and nothing read
// them back. Three links were missing: the style itself, the read-back
// (latexHead), and the sectioning call that sets the mark.
func TestPagestyleHeadingsDeclaresAHead(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\documentclass{article}\pagestyle{headings}\begin{document}` +
		`\section{Ma Section}\makeatletter\message{[\meaning\@oddhead]}` +
		`\message{[\gotex@rightmark]}\makeatother\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `\rightmark`) || !strings.Contains(out, `\thepage`) {
		t.Errorf(`\@oddhead = %q, want \rightmark … \thepage`, out)
	}
	// \section set the mark through \sectionmark, which \ps@headings redefined.
	if !strings.Contains(out, "Ma Section") {
		t.Errorf("the section did not reach the mark: %q", out)
	}
}

// \pagestyle{empty} and {plain} must CLEAR a head an earlier style set, or a
// document that switches keeps one it asked to drop.
func TestPlainAndEmptyClearTheHead(t *testing.T) {
	for _, style := range []string{"empty", "plain"} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\pagestyle{headings}` +
			`\pagestyle{` + style + `}\begin{document}X\end{document}`); err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		if e.hasLatexHead() {
			t.Errorf(`\pagestyle{%s} left a head behind`, style)
		}
	}
}

// The starred form does not mark — latex.ltx's \@ssect has no \…mark call — and
// the numbered one marks AFTER \refstepcounter, so the mark carries the section
// it names rather than the one before it.
func TestOnlyNumberedSectionsMark(t *testing.T) {
	run := func(src string) string {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		out, err := e.Run(`\documentclass{article}\pagestyle{headings}\begin{document}` +
			src + `\makeatletter\message{[\gotex@rightmark]}\makeatother\end{document}`)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	if got := run(`\section*{Sans Numero}`); strings.Contains(got, "Sans Numero") {
		t.Errorf("a starred section marked: %q", got)
	}
	if got := run(`\section{Une}\section{Deux}`); !strings.Contains(got, "Deux") {
		t.Errorf("the mark does not carry the latest section: %q", got)
	}
}
