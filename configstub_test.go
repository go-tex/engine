// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// Since #302 an undefined control sequence no longer swallows the groups after it:
// tex.web forgets the command, and the following {…} is an ordinary group, which is
// TYPESET. So every configuration command the engine lacked left its arguments on
// the page — and in a preamble that opens a page of its own.
func TestConfigurationCommandsLeaveNothingOnThePage(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"fancyheadoffset", `\fancyheadoffset[LE,RO]{0.5cm}X\par`},
		{"fancyheadoffset sans crochet", `\fancyheadoffset{0.5cm}X\par`},
		{"fancyfootoffset", `\fancyfootoffset[LE]{1cm}X\par`},
		{"DeclareNewFootnote", `\DeclareNewFootnote[para]{A}[roman]X\par`},
		{"DeclareNewFootnote nu", `\DeclareNewFootnote{A}X\par`},
		{"AtBeginEnvironment", `\AtBeginEnvironment{figure}{\centering}X\par`},
		{"AtEndEnvironment", `\AtEndEnvironment{figure}{\hrule}X\par`},
		{"counterwithout", `\counterwithout{footnote}{section}X\par`},
		{"counterwithin", `\counterwithin{equation}{section}X\par`},
		{"setcellgapes", `\setcellgapes{2pt}X\par`},
		{"makegapedcells", `\makegapedcells X\par`},
		// latex.ltx:7523-7527 — \@ifstar then four. 11 corpus papers, 53 occurrences.
		{"DeclareMathSizes", `\DeclareMathSizes{10}{10}{7}{5}X\par`},
		{"DeclareMathSizes étoilé", `\DeclareMathSizes*{10}{10}{7}{5}X\par`},
		// amsthm.sty — \newcommand{\newtheoremstyle}[9].
		{"newtheoremstyle", `\newtheoremstyle{n}{3pt}{3pt}{}{}{\bfseries}{.}{.5em}{}X\par`},
		// natbib, one argument.
		{"setcitestyle", `\setcitestyle{authoryear}X\par`},
		// xcolor.sty:616-618 — \@testopt then four.
		{"definecolorset", `\definecolorset{rgb}{a}{b}{x,1,0,0}X\par`},
		{"definecolorset crochet", `\definecolorset[named]{rgb}{a}{b}{x,1,0,0}X\par`},
		// mathpartir's layout options.
		{"mprset", `\mprset{flushleft}X\par`},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if txt := mvlText(e.mvl); txt != "X" {
			t.Errorf("%s left %q on the page, want X alone", c.name, txt)
		}
	}
}

// None of them may swallow what FOLLOWS: a command whose optional argument is
// absent must not take the next group, or it would eat the document.
func TestConfigurationCommandsDoNotSwallowWhatFollows(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"DeclareNewFootnote", `\DeclareNewFootnote{A}AVANT \textit{APRES}\par`},
		{"makegapedcells", `\makegapedcells AVANT \textit{APRES}\par`},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		txt := mvlText(e.mvl)
		if txt != "AVANTAPRES" {
			t.Errorf("%s: got %q, want AVANTAPRES", c.name, txt)
		}
	}
}
