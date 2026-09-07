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
