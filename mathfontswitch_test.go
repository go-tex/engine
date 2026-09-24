// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A font or size switch reaches a formula through an INDIRECTION, and expanding it
// there costs the whole formula.
//
// The maths layer cannot select a face, so \small is unknown to it. The retry then
// expands it — and \small is not a primitive but a macro whose replacement text is
// \@setfontsize plus four display-skip assignments and a \def\@listi{…}
// (size10.clo:58). One unknown command became a dozen, and the equation was dropped
// against \edef: a primitive the document never wrote and cannot be asked to avoid.
//
// Marking the body's own tokens \noexpand cannot catch it, because nothing in that
// body says \small. One corpus paper writes
//
//	\newcommand*{\codefont}{\ttfamily\small}
//	\DeclareTextFontCommand{\codefontify}{\codefont}
//	\newcommand{\codify}[1]{\ensuremath{\mbox{\codefontify{#1}}}}
//
// so the switch appears only after \codify and \codefontify have both been
// expanded. All 30 of its displays were dropped, and the words inside them —
// identifiers like "choose" and "sumlist", read as prose by the reader — left the
// page with them.
//
// The formula keeps its content and loses the face it asked for. That is the
// deliberate trade: a size is a detail, a dropped equation is the whole sentence.
//
// Only the FIRST case below fails without the fix (main drops it against \edef).
// The other three already pass, because a switch written straight into the source
// reaches \mbox and is set in text mode before the maths layer ever sees it. They
// are guards on the paths either side of the one that broke, not witnesses to it.
func TestMathKeepsAFormulaWhoseMacroHidesAFontSwitch(t *testing.T) {
	for _, c := range []struct{ nom, preamble, body string }{
		{
			"through two indirections",
			`\newcommand*{\codefont}{\ttfamily\small}` +
				`\DeclareTextFontCommand{\codefontify}{\codefont}` +
				`\newcommand{\codify}[1]{\ensuremath{\mbox{\codefontify{#1}}}}`,
			`$x = \codify{sumlist} + y$`,
		},
		{"size switch written into the formula", ``, `$x = \mbox{\small y} + z$`},
		{"old font command", `\newcommand{\sample}{\mbox{\sf S}}`, `$\sample = 1$`},
		{"family switch behind a macro", `\newcommand{\m}{\ttfamily M}`, `$\mbox{\m} > 0$`},
	} {
		e, err := compile([]byte(`\documentclass{article}`+c.preamble+
			`\begin{document}`+c.body+`\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: the math layer refused the formula (%v)", c.nom, e.mathDropped)
		}
		if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
			t.Errorf("%s: no path — the formula is not typeset", c.nom)
		}
	}
}

// The switch must be suspended only while a maths macro body is flattened. Outside
// that, \small is an ordinary size change and the text after it is SMALLER: a guard
// that leaked into text mode would silently set a document at one size.
func TestFontSwitchStillWorksInTextMode(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`{\small small text}\par{\normalsize normal text}\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.mathFlatten {
		t.Error("mathFlatten left set after the document ran")
	}
	if svg := strings.Join(e.RenderPages(e.renderMargin(0)), ""); !strings.Contains(svg, "<path") {
		t.Error("no path — the text is not typeset")
	}
}
