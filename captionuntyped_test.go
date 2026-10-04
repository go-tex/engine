// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ A \caption with no \@captype typeset TeX SOURCE. \@captype undefined makes
// \csname c@\@captype\endcsname into \c@, which is undefined and therefore \relax,
// and \advance\relax stops before its keyword — so the literal "by1" reached the
// page and the caption read "by1: X" (go-tex/engine#558).
//
// Real LaTeX raises "\caption outside float", so there is no correct number to
// print; lenient, the caption's TEXT is what the document wanted on the page.
//
// The control that makes this test mean something is the second case: a caption
// inside a real float must still be numbered. Without it, a \caption that emitted
// nothing at all would pass.
func TestCaptionWithNoCaptypeLeaksNoSource(t *testing.T) {
	for _, c := range []struct {
		name, src, want, absent string
	}{
		{"no @captype: the text, no label", `\caption{CAP}X`, "CAPX", "by1"},
		{"inside a float: still numbered", `\begin{figure}\caption{CAP}\end{figure}X`, "Figure1:CAPX", "by1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}`+c.src+`\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			got := pageChars(e)
			if !strings.Contains(got, c.want) {
				t.Errorf("the page carries %q, which is missing %q", got, c.want)
			}
			if strings.Contains(got, c.absent) {
				t.Errorf("the page carries %q, which still leaks %q", got, c.absent)
			}
		})
	}
}

// ⛔ The body of an SC float must not be EXPANDED while its optionals are being
// looked for. doSCfloat used the expanding bracket scan, which expands the next
// token to decide whether it is a "[" — so with no optional present it expanded
// the body's first token BEFORE \figure had run \def\@captype{figure}.
//
// This is the witness, reduced to one question asked at that exact position: is
// the type in force for the first thing in the body? One token of distance
// (\relax) used to be enough to change the answer from UNDEF to DEF.
//
// Captions survived it only by accident — \caption expands one step, the scan
// sees \par instead of "[" and pushes the expansion back, so the tokens EXECUTE
// after \figure — but anything deciding something during that expansion read the
// wrong state, which is what blocked the fix above.
func TestSCfloatDoesNotExpandItsBodyLookingForOptionals(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"no optionals", `\begin{SCfigure}\@ifundefined{@captype}{UNDEF}{DEF}\end{SCfigure}`},
		{"two optionals", `\begin{SCfigure}[0.5][t]\@ifundefined{@captype}{UNDEF}{DEF}\end{SCfigure}`},
		{"table, one optional", `\begin{SCtable}[h]\@ifundefined{@captype}{UNDEF}{DEF}\end{SCtable}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\usepackage{sidecap}\makeatletter`+
				`\begin{document}`+c.src+`\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			if got := pageChars(e); !strings.Contains(got, "DEF") || strings.Contains(got, "UNDEF") {
				t.Errorf("the page carries %q: \\@captype is not in force for the body's first token", got)
			}
		})
	}
}
