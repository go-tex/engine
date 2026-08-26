// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A '#' reduced from '##' in a macro body keeps its parameter-character nature, so a
// nested \def scanned from that body still treats #1 as a parameter — the mechanism
// amsart's \def\@andlistc##1{…} (inside \newcommand\nxandlist) relies on. Before the
// fix the reduced '#' became an "other" character, the nested \def saw no parameter,
// and the author-list recursion ran away.
func TestDoubledHashKeepsParamNesting(t *testing.T) {
	out := mustRun(t, `\newcommand\mk{\def\inner##1{<##1>}}\mk\message{\inner{Z}}`)
	if !strings.Contains(out, "<Z>") {
		t.Errorf("nested \\def from ## body = %q, want <Z>", out)
	}
}

// A stray literal '#' that reaches the stomach (a ## with no following digit consumer)
// is typeset as '#', not dropped.
func TestStrayHashTypeset(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}a\#b\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(got, "#") {
		t.Errorf("literal # not typeset: %q", got)
	}
}

// A \global-prefixed assignment through a register alias must consume its value
// (\global\topskip42\p@ previously dropped \topskip and leaked "42" onto the page).
func TestGlobalRegisterAssignmentConsumes(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\newdimen\gd\newskip\gs\global\gd=5pt\global\gs=3pt X\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if strings.ContainsAny(got, "0123456789") || strings.Contains(got, "pt") {
		t.Errorf("a \\global register assignment leaked its value: %q", got)
	}
	if !strings.Contains(got, "X") {
		t.Errorf("expected X in %q", got)
	}
}

// The magnification-independent "true" unit prefix parses like the plain unit.
func TestTrueUnitPrefix(t *testing.T) {
	out := mustRun(t, `\dimen4=.5truein \dimen6=.5in \ifdim\dimen4=\dimen6 \message{SAME}\else\message{DIFF}\fi`)
	if !strings.Contains(out, "SAME") {
		t.Errorf(".5truein != .5in: %q", out)
	}
}

// A space between "true" and its unit ("9.0 true in", as written in tmlr.sty and
// other class files) must still be read as inches, not silently defaulted to pt.
// The bug made \textheight 9.0 true in resolve to 9pt, giving tiny pages and a
// hundreds-of-pages runaway (arXiv 2608.12489: 306 pages vs a reference 40).
func TestTrueUnitPrefixWithSpace(t *testing.T) {
	out := mustRun(t, `\dimen4=9.0 true in \dimen6=9in \ifdim\dimen4=\dimen6 \message{SAME}\else\message{DIFF}\fi`)
	if !strings.Contains(out, "SAME") {
		t.Errorf("9.0 true in != 9in: %q", out)
	}
}

// \setbox accepts a \newbox-allocated control sequence as its target, not only a bare
// register number (amsart's \setbox\abstractbox=\vtop…). Otherwise the '=' leaks.
func TestSetboxAcceptsBoxRef(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter\begin{document}`+
		`\newbox\mybox\setbox\mybox=\hbox{Q}Z\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if strings.Contains(got, "=") {
		t.Errorf("\\setbox\\<boxref> leaked '=': %q", got)
	}
	if !strings.Contains(got, "Z") {
		t.Errorf("expected Z in %q", got)
	}
}

// The mode conditionals resolve during ordinary (non-math) processing: \ifmmode is
// false, and \ifvoid on an unused register is true.
func TestModeConditionals(t *testing.T) {
	out := mustRun(t, `\ifmmode\message{MM}\else\message{NOTMM}\fi`+
		`\makeatletter\ifvoid7 \message{VOID}\else\message{FULL}\fi`)
	if !strings.Contains(out, "NOTMM") || !strings.Contains(out, "VOID") {
		t.Errorf("mode conditionals = %q, want NOTMM and VOID", out)
	}
}
