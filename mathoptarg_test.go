// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \newcommand{\cmd}[n][default] makes the FIRST of the n parameters optional: \cmd
// is \@protected@testopt, which supplies {default} when no [ follows
// (latex.ltx:1187-1199), and the remaining n-1 are grabbed as usual.
//
// The maths source path read all n as MANDATORY, which is worse than not expanding
// at all: with the optional argument written out the macro still "matched", by
// taking the brackets themselves as arguments —
//
//	\qbin [p]{N}{k}  ->  \genfrac []{0pt}{}{p}{]}_{[}{N}{k}
//	\qbin {N}{k}+1   ->  \genfrac []{0pt}{}{k}{+}_{N}1
//
// — and the formula came out silently wrong. Without it, nothing matched and the
// formula was dropped.
func TestMathMacroWithAnOptionalArgument(t *testing.T) {
	e, err := NewDocument(Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	e.Run(`\newcommand{\qbin}[3][q]{\genfrac[]{0pt}{}{#2}{#3}_{#1}}`)
	for _, c := range []struct{ src, want string }{
		{`\qbin {N}{k}`, `\genfrac []{0pt}{}{N}{k}_{q}`},
		{`\qbin [p]{N}{k}`, `\genfrac []{0pt}{}{N}{k}_{p}`},
		{`\qbin {N}{k}+1`, `\genfrac []{0pt}{}{N}{k}_{q}+1`},
	} {
		got, ok := e.expandMacroInMathSource(c.src, "qbin")
		if !ok || got != c.want {
			t.Errorf("expandMacroInMathSource(%q) = (%q, %v), want (%q, true)",
				c.src, got, ok, c.want)
		}
	}
}

// And end to end: the formula must reach the page rather than being dropped or set
// from stolen tokens.
func TestMathMacroWithAnOptionalArgumentTypesets(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+
		`\newcommand{\qbin}[3][q]{\genfrac[]{0pt}{}{#2}{#3}_{#1}}`+
		`\begin{document}AVANT $\qbin{N}{k}$ APRÈS\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("la couche maths a refusé la formule (%v)", e.mathDropped)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "<path"); n < 15 {
		t.Errorf("%d tracés: le binôme et le texte qui l'entoure ne sont pas tous là", n)
	}
}

// takeMathOptArg keeps what skipMathOptArg throws away, and must not mistake a
// bracket inside a brace group — or an unclosed one — for the argument.
func TestTakeMathOptArg(t *testing.T) {
	for _, c := range []struct {
		src, arg, rest string
		ok             bool
	}{
		{`[p]{N}`, "p", "{N}", true},
		{`  [p]{N}`, "p", "{N}", true},
		{`{N}{k}`, "", `{N}{k}`, false},
		{`[{a]b}]x`, `{a]b}`, "x", true},
		{`[p{N}`, "", `[p{N}`, false},
	} {
		arg, rest, ok := takeMathOptArg(c.src)
		if arg != c.arg || rest != c.rest || ok != c.ok {
			t.Errorf("takeMathOptArg(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.src, arg, rest, ok, c.arg, c.rest, c.ok)
		}
	}
}
