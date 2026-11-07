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
		// ⛔ Probed through \rightmark, the name a page style reads, and not
		// through the pair's storage. These two tests passed against a PRIVATE
		// pair that only our own \markboth wrote, so they were green on every
		// document whose class brings its own \markboth — amsart among them —
		// and #426's head stayed empty. A test that names the storage cannot
		// tell a mark that was set from a mark that is reachable.
		`\message{[\rightmark]}\makeatother\end{document}`)
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

// ⛔⛔ THE TEST #426 NEEDED, AND THE ONE EVERY GREEN TEST HERE WAS MISSING.
//
// A class may bring its OWN \markboth, and amsart.cls — one of the four classes
// this engine advertises running — does (texmf/amsart.cls:335-340):
//
//	\def\markboth#1#2{\begingroup
//	  \@temptokena{{#1}{#2}}\xdef\@themark{\the\@temptokena}%
//	  \mark{\the\@temptokena}\endgroup …}
//
// It writes \@themark, the name the kernel uses (ltpage.dtx:355), and nothing
// else. Every other link in the chain already worked — \ps@headings does
// \let\@mkboth\markboth, amsart calls \@mkboth{\shortauthors}{\shorttitle} at
// :622, the mark was set — and the head was still empty, because \rightmark read
// a private pair that only OUR \markboth wrote. The engine's \markboth answering
// correctly in isolation is what made that invisible: a primitive that answers is
// not a mechanism that is reached.
func TestAClassThatBringsItsOwnMarkbothReachesTheHead(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\documentclass{article}\pagestyle{headings}\makeatletter` +
		// amsart's idiom, verbatim in shape: the class's own \markboth, which
		// overrides ours and writes the kernel's name.
		`\def\markboth#1#2{\@temptokena{{#1}{#2}}\xdef\@themark{\the\@temptokena}}` +
		`\makeatother\begin{document}\markboth{AUTEURS}{TITRE COURT}` +
		`\message{[\leftmark|\rightmark]}\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[AUTEURS|TITRE COURT]") {
		t.Errorf("a class's own \\markboth does not reach the head: %q", out)
	}
}

// \markright replaces the right half and keeps the left, which is why the kernel
// routes it through \@markright (ltpage.dtx:356-357) instead of assigning both.
// \ps@headings's \sectionmark IS a \markright, so a wrong \markright wipes the
// left half of the head at every section.
func TestMarkrightKeepsTheLeftHalf(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\documentclass{article}\begin{document}` +
		`\markboth{GAUCHE}{DROITE}\markright{AUTRE}` +
		`\message{[\leftmark|\rightmark]}\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[GAUCHE|AUTRE]") {
		t.Errorf("\\markright did not keep the left mark: %q", out)
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
			src + `\message{[\rightmark]}\end{document}`)
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
