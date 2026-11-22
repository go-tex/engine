// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ssMacro returns the characters of a plain macro's body.
func ssMacro(e *Engine, name string) string {
	m := e.eq[name]
	if m == nil || m.kind != mMacro {
		return ""
	}
	var b []rune
	for _, t := range m.body {
		if !t.cs_ {
			b = append(b, t.ch)
		}
	}
	return string(b)
}

// setspace.sty single-spaces floats and footnotes, however the body is stretched:
//
//	\let\latex@xfloat=\@xfloat
//	\def\@xfloat #1[#2]{\latex@xfloat #1[#2]\def\baselinestretch{\setspace@singlespace}\normalsize}
//	\long\def\@footnotetext#1{\insert\footins{\def\baselinestretch{\setspace@singlespace}
//	    \reset@font\footnotesize ...
//
// (setspace.sty:425-430 and 449-453; \@mpfootnotetext, 469-473, does the same.) Witness s1
// (unblock-outliers-2.md), book at 12pt with \baselinestretch 1.25: tectonic sets table
// rows 14.45pt apart and footnote lines 11.95pt, we set 18.05pt and 18.05pt, because the
// body's 1.25 reached into both.
func TestSetspaceSingleSpacesFloatsAndFootnotes(t *testing.T) {
	run := func(pkg, body string) *Engine {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		src := `\documentclass[12pt,oneside,a4paper]{book}
\usepackage{float}
` + pkg + `
\renewcommand{\baselinestretch}{1.25}
\begin{document}
\xdef\bodyBS{\the\baselineskip}
` + body + `
\xdef\afterBS{\the\baselineskip}
\end{document}
`
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		return e
	}
	floats := `\begin{table}[t]\xdef\floatBS{\the\baselineskip}\end{table}
\begin{table}[H]\xdef\hereBS{\the\baselineskip}\small\xdef\hereSmallBS{\the\baselineskip}\end{table}
BODYD\footnote{\xdef\footBS{\the\baselineskip}}`

	e := run(`\usepackage{setspace}`, floats)
	for _, c := range []struct{ name, want string }{
		{"bodyBS", "18.125pt"},
		{"floatBS", "14.5pt"},
		{"hereBS", "14.5pt"},
		{"hereSmallBS", "13.6pt"},
		{"footBS", "12.0pt"},
		{"afterBS", "18.125pt"}, // and the body comes back stretched
	} {
		if got := ssMacro(e, c.name); got != c.want {
			t.Errorf("with setspace, \\baselineskip %s = %q, want %q", c.name, got, c.want)
		}
	}

	// Without setspace nothing single-spaces a float: \baselinestretch reaches it.
	e = run(``, floats)
	for _, c := range []struct{ name, want string }{
		{"floatBS", "18.125pt"},
		{"footBS", "15.0pt"},
	} {
		if got := ssMacro(e, c.name); got != c.want {
			t.Errorf("without setspace, \\baselineskip %s = %q, want %q", c.name, got, c.want)
		}
	}
}
