// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// latex.ltx defines \textsuperscript as \@textsuperscript, whose body is
// \m@th\ensuremath{^{\mbox{\fontsize\sf@size\z@\selectfont #1}}}: the argument is typeset
// in an \mbox, so it is EXECUTED. sn-jnl.cls:519-521 puts a \@for loop in it (the author
// affiliation marks); expanded through the math layer instead, the loop never ran, its
// \@citex-style helpers stayed undefined and 68 corpus papers lost their title block
// (2405.10478: 23 pages, tectonic 27). The witness is tectonic's output for
//
//	\makeatletter BEFORE\textsuperscript{\@for\x:=1,2\do{\edef\y{\noexpand\g@addto@macro
//	  \noexpand\acc{\x;}}\y}}AFTER
//	\show\acc  ->  macro:->1;2;
func TestTextsuperscriptExecutesItsArgument(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\makeatletter\hsize=300pt \def\acc{}` +
		`BEFORE\textsuperscript{\@for\x:=1,2\do{\edef\y{\noexpand\g@addto@macro\noexpand\acc{\x;}}\y}}AFTER`); err != nil {
		t.Fatal(err)
	}
	if got := e.macroPlainText("acc"); got != "1;2;" {
		t.Errorf("\\acc = %q, want %q (the argument of \\textsuperscript must be executed)", got, "1;2;")
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	collectChars(e.parList, &b)
	if got := b.String(); !strings.Contains(got, "BEFORE") || !strings.Contains(got, "AFTER") {
		t.Errorf("text around \\textsuperscript lost: %q", got)
	}
}
