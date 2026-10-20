// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// calc.sty makes a LENGTH ARGUMENT an arithmetic expression. The grammar is
// calc.dtx's own (required/tools/calc.dtx):
//
//	<expr>   ::= <term> | <expr> ('+'|'-') <term>
//	<term>   ::= <factor> | <term> ('*'|'/') <integer>
//	<factor> ::= <dimen> | '(' <expr> ')'
func TestCalcExpressionsInALengthArgument(t *testing.T) {
	for _, c := range []struct {
		expr string
		want string
	}{
		{`100pt+50pt`, "150.0pt"},
		{`100pt-25pt`, "75.0pt"},
		{`100pt*3`, "300.0pt"},
		{`100pt/4`, "25.0pt"},
		{`(100pt+20pt)*2`, "240.0pt"},
		{`100pt+20pt*2`, "140.0pt"}, // * binds tighter than +
		{`100pt`, "100.0pt"},        // a plain length still works
	} {
		got := theLength(t, `\usepackage{calc}`, c.expr)
		if got != c.want {
			t.Errorf("\\setlength{\\x}{%s} = %s, attendu %s", c.expr, got, c.want)
		}
	}
}

// ⛔ And WITHOUT calc the arithmetic must NOT happen. Making every <dimen>
// calc-aware would change what "\hskip 2pt -3pt" means in any document that happens
// to load the package, so the hooks are inert without it.
//
// ⚠ The assertion is that the sum is not computed, NOT what the value is. Measured
// on v0.260.0, before this change, \setlength{\x}{100pt+50pt} without calc gives
// 0.0pt and typesets "+50pt" — where LaTeX gives 100pt and typesets "+50pt". That is
// a separate pre-existing defect in \setlength's own scanning, and pinning it here
// would make this test block its fix.
func TestWithoutCalcTheArithmeticDoesNotHappen(t *testing.T) {
	if got := theLength(t, ``, `100pt+50pt`); got == "150.0pt" {
		t.Error("sans calc, l'expression a été évaluée: les accroches ne sont pas inertes")
	}
}

// ⛔ The real failure this fixes is not an arithmetic error, it is TEXT ON THE PAGE.
// Without calc's scanning, \begin{minipage}[t]{\textwidth-57.625mm} reads
// \textwidth, stops, and the characters "-57.625mm" are typeset at the top of the
// body — which is how arXiv 2406.08646 and 2302.04180 open (sagej.cls:318).
func TestACalcExpressionIsNotTypesetAsText(t *testing.T) {
	src := `\documentclass{article}\usepackage{calc}\begin{document}` +
		`\begin{minipage}[t]{\textwidth-100pt}GOTEXBODY\end{minipage}\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "GOTEXBODY") {
		t.Fatalf("le corps du minipage est absent: %q", txt)
	}
	if strings.Contains(txt, "100pt") {
		t.Errorf("l'expression est composée comme du texte: %q", txt)
	}
}

// \real and \ratio are NOT implemented. They must be recorded as skipped rather
// than silently producing a number, because a wrong factor mis-sizes a box with
// nothing to show for it.
func TestCalcRealAndRatioAreRecordedNotGuessed(t *testing.T) {
	for _, expr := range []string{`100pt*\real{1.5}`, `100pt*\ratio{2pt}{1pt}`} {
		e := lengthEngine(t, `\usepackage{calc}`, expr)
		found := false
		for k := range e.skippedCS {
			if strings.HasPrefix(k, "calc:") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: rien n'est recensé; les sauts sont %v", expr, e.skippedCS)
		}
	}
}

// theLength returns what \the prints for a length set from expr.
func theLength(t *testing.T, preamble, expr string) string {
	t.Helper()
	e := lengthEngine(t, preamble, expr)
	return strings.TrimSpace(mvlText(e.mvl))
}

func lengthEngine(t *testing.T, preamble, expr string) *Engine {
	t.Helper()
	src := `\documentclass{article}` + preamble + `\newlength{\gotexlen}\begin{document}` +
		`\setlength{\gotexlen}{` + expr + `}\the\gotexlen\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile %q: %v", expr, err)
	}
	return e
}
