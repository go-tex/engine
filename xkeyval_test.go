// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// pageText renders a lenient compile and returns the text of every page, so a
// test can assert on what actually reached the page.
func pageText(t *testing.T, src string) string {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	return b.String()
}

// setkeys runs a key's code, a bare key runs its [default], and an unknown key is
// ignored rather than fatal. \define@boolkey creates the \if<macpre><key> switch
// and flips it from the value (xkeyval.tex:206-232); its + form takes a second
// function for a value that is neither true nor false.
func TestXKeyvalBooleanKey(t *testing.T) {
	const src = `\documentclass{article}
\makeatletter
\define@boolkey+{fam}[@T@]{screen}[true]{FN}{ERR}
\begin{document}
\makeatletter
A\setkeys{fam}{screen=true}\if@T@screen VRAI\else FAUX\fi
B\setkeys{fam}{screen=false}\if@T@screen VRAI\else FAUX\fi
C\setkeys{fam}{screen}\if@T@screen VRAI\else FAUX\fi
D\setkeys{fam}{screen=oops}
E\setkeys{fam}{nosuchkey=1}reste
\end{document}`
	got := pageText(t, src)
	for _, want := range []string{"AFNVRAI", "BFNFAUX", "CFNVRAI", "DERR", "Ereste"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// \define@choicekey*+ sets the value macro and the 0-based index of the value in
// the choice list, -1 when it is not there (xkeyval.tex:288-300), and runs the
// second function in that case. Spaces around a list item and around a value are
// not part of either.
func TestXKeyvalChoiceKey(t *testing.T) {
	const src = `\documentclass{article}
\makeatletter
\define@choicekey*+{fam}{format}[\FMT\NR]{manuscript, acmsmall, sigconf, sigplan}[manuscript]{OK}{BAD}
\begin{document}
\makeatletter
A\setkeys{fam}{format=sigconf}[\FMT][\NR]
B\setkeys{fam}{format = sigplan }[\FMT][\NR]
C\setkeys{fam}{format=nope}[\NR]
D\setkeys{fam}{format}[\FMT]
\end{document}`
	got := pageText(t, src)
	for _, want := range []string{"AOK[sigconf][2]", "BOK[sigplan][3]", "CBAD[-1]", "DOK[manuscript]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

// \DeclareOptionX declares an option and \ProcessOptionsX applies the ones the
// document asked for. A declared option ALWAYS gets a default (\@testopt ... {},
// xkeyval.sty:94), which is what makes a BARE class option — the way every
// document names its format — do anything at all.
func TestXKeyvalDeclareAndProcessOptions(t *testing.T) {
	const src = `\documentclass{article}
\makeatletter
\gdef\PICKED{none}
\DeclareOptionX<myfam>{sigconf}{\gdef\PICKED{sigconf}}
\DeclareOptionX<myfam>{acmsmall}{\gdef\PICKED{acmsmall}}
\begin{document}
\makeatletter
\setkeys{myfam}{sigconf}[\PICKED]
\end{document}`
	if got := pageText(t, src); !strings.Contains(got, "[sigconf]") {
		t.Errorf("a bare declared option did not fire: %q", got)
	}
}

// A class computes the options it passes on: acmart.cls:254 is
// \LoadClass[\ACM@fontsize, reqno]{amsart}. latex.ltx stores an option list with
// an \xdef ("\xdef\@classoptionslist{\zap@space#2 \@empty}", latex.ltx:13712), so
// the list is EXPANDED; read raw it asks amsart for an option literally named
// "\ACM@fontsize", and a 9pt class comes out in the default 10pt.
func TestClassOptionListIsExpanded(t *testing.T) {
	const src = `\documentclass{article}
\makeatletter
\def\thesize{9pt}
\LoadClass[\thesize]{amsart}
\begin{document}
\makeatletter size=[\f@size]
\end{document}`
	if got := pageText(t, src); !strings.Contains(got, "size=[9]") {
		t.Errorf("the computed class option was not expanded: %q", got)
	}
}
