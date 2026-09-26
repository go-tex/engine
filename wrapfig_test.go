// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// wrapfig's \begin{wrapfigure}[<lines>]{<position>}{<width>} — wrapfig.sty:40-43:
//
//	\def\wrapfloat#1{\def\@captype{#1}\@ifnextchar[\WF@wr{\WF@wr[]}}
//	\def\wrapfigure{\wrapfloat{figure}}
//	\def\wraptable{\wrapfloat{table}}
//
// Undefined, \begin{wrapfigure} resolved to \relax through \csname, so its POSITION and
// WIDTH were typeset on the page and the body was set as running prose — a figure and its
// caption in the middle of a sentence, and the caption itself came out as "by1:" instead
// of "Figure 1:". Eight corpus papers use it seventeen times; two use \wraptable.
//
// This engine cannot reshape a paragraph, so the text does not wrap: the body becomes an
// ordinary centred float of the stated width. Measured against its parent on the 154-paper
// corpus: Sigma 323 -> 324 (+1), and on the one paper that moves the ONLY words lost are
// "R0.3" and "r0.5" — the arguments, which the reference does not print.
func TestWrapfigureConsumesItsArgumentsAndFloats(t *testing.T) {
	for _, c := range []struct{ name, begin, captype string }{
		{"wrapfigure, no optional", `\begin{wrapfigure}{r}{0.4321\textwidth}`, "Figure"},
		{"wrapfigure, with lines", `\begin{wrapfigure}[8]{l}{0.4321\textwidth}`, "Figure"},
		{"wraptable", `\begin{wraptable}{r}{0.4321\textwidth}`, "Table"},
	} {
		env := "wrapfigure"
		if strings.Contains(c.begin, "wraptable") {
			env = "wraptable"
		}
		src := `\documentclass{article}\usepackage{wrapfig}\begin{document}BEFORE` +
			c.begin + `BODYWORD\caption{THECAPTION}\end{` + env + `}AFTER\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: compile: %v", c.name, err)
		}
		if got := e.Diagnostics().UndefinedEnvs[env]; got != 0 {
			t.Errorf("%s: %s still reported undefined (%d)", c.name, env, got)
		}
		svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
		// The arguments are arguments, not content. The width carries a distinctive
		// digit string: searching for "0.4" hits the SVG's own numeric attributes, which
		// is how this assertion first came out red against a fix that works.
		if strings.Contains(svg, "4321") {
			t.Errorf("%s: the width argument reached the page", c.name)
		}
		for _, want := range []string{"BEFORE", "BODYWORD", "THECAPTION", "AFTER", c.captype} {
			if !strings.Contains(svg, want) {
				t.Errorf("%s: %q is not on the page", c.name, want)
			}
		}
	}
}
