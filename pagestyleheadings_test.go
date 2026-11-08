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
	// ⛔⛔ THE MARK IS READ OFF THE DRAWN HEAD, not out of an expansion.
	//
	// This asserted `\message{[\rightmark]}` at DOCUMENT time, and with marks
	// travelling there is nothing there to read: \rightmark expands \firstmark,
	// which the page builder publishes per page, and the page builder runs after
	// the whole document has been read. A real LaTeX answers empty at that point
	// too, for the same reason — latex.ltx has carried a note about it since 1993
	// (ltpage.dtx:318-323, with an upstream \task still open).
	//
	// So the question becomes the one that matters anyway: does the section's mark
	// reach the HEAD of the page it belongs to? \newpage is needed because page 1
	// of article carries no head.
	out, err := e.Run(`\documentclass{article}\pagestyle{headings}\begin{document}` +
		`\section{Ma Section}\makeatletter\message{[\meaning\@oddhead]}\makeatother` +
		`\newpage Deuxieme page.\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `\rightmark`) || !strings.Contains(out, `\thepage`) {
		t.Errorf(`\@oddhead = %q, want \rightmark … \thepage`, out)
	}
	// \section set the mark through \sectionmark, which \ps@headings redefined to
	// \markright{\MakeUppercase{…}} — so the head carries it uppercased.
	// No space in the needle: pageText collects the glyphs, and the interword glue
	// between them is not one of them.
	if page := pageText(e); !strings.Contains(page, "MASECTION") {
		t.Errorf("the section's mark did not reach the drawn head: %q", page)
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
	// ⛔ The fixture now emits the \mark too, as amsart does, and the earlier
	// version did NOT: it copied the \@themark half of amsart's idiom and left out
	// `\mark{\the\@temptokena}`. That was enough while \rightmark read the pair
	// directly, and it is not a class any more now that the pair travels — a
	// fixture missing the line that makes the mechanism work tests the mechanism
	// not working.
	_, err := e.Run(`\documentclass{article}\pagestyle{headings}\makeatletter` +
		`\def\markboth#1#2{\@temptokena{{#1}{#2}}\xdef\@themark{\the\@temptokena}` +
		`\mark{\the\@temptokena}}` +
		`\makeatother\begin{document}\markboth{AUTEURS}{TITRE COURT}Du texte.` +
		`\newpage Deuxieme page.\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	// article's \ps@headings puts \rightmark in the odd head, which is the pair's
	// RIGHT half — the short title.
	if page := pageText(e); !strings.Contains(page, "TITRECOURT") {
		t.Errorf("a class's own \\markboth does not reach the drawn head: %q", page)
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
	// ⛔ Asserted on \@themark, the kernel's OWN variable for the live pair
	// (ltpage.dtx:355), and not on \leftmark/\rightmark: those read \botmark and
	// \firstmark, which belong to a PAGE, so at document time there is nothing
	// there — in a real LaTeX either. \@themark is a documented name that classes
	// read (amsart writes it), so this is an interface and not an internal; what
	// it cannot show is reachability, which the drawn-head tests above do.
	out, err := e.Run(`\documentclass{article}\begin{document}` +
		`\markboth{GAUCHE}{DROITE}\markright{AUTRE}` +
		`\makeatletter\message{[\@themark]}\makeatother\end{document}`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[{GAUCHE}{AUTRE}]") {
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
	// Read off the drawn head rather than out of \rightmark at document time: the
	// mark belongs to a page, so there is nothing to expand before the page
	// builder has run. The needles carry no spaces — pageText collects glyphs, not
	// the glue between them — and they are uppercase because \ps@headings's
	// \sectionmark is \markright{\MakeUppercase{…}}.
	run := func(src string) string {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\pagestyle{headings}\begin{document}` +
			src + `\newpage Deuxieme page.\end{document}`); err != nil {
			t.Fatal(err)
		}
		return pageText(e)
	}
	// ⛔ The starred form must not mark, and the head of the page it sits on is
	// where that shows: latex.ltx's \@ssect has no \…mark call.
	if got := run(`\section*{Sans Numero}`); strings.Contains(got, "SANSNUMERO") {
		t.Errorf("a starred section reached the head: %q", got)
	}
	// Two numbered sections: the head carries the LATEST, because \sectionmark
	// marks after \refstepcounter.
	if got := run(`\section{Une}\section{Deux}`); !strings.Contains(got, "DEUX") {
		t.Errorf("the head does not carry the latest section: %q", got)
	}
}
