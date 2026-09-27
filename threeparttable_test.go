// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// threeparttable's \begin{threeparttable}[<pos>] and \begin{tablenotes}[<opts>]
// (threeparttable.sty:107-120, :258-262). The real package MEASURES the tabular and boxes
// the caption, the table and the notes as one unit at the table's width. This engine cannot
// reshape that and does not need to: measured on a witness, the content and the note labels
// already came out in the right order and at the right size (\TPTnoteSettings only sets list
// margins, not a font size).
//
// What was wrong was the OPTIONAL ARGUMENT and \@captype:
//
//   - undefined, \begin{threeparttable}[b] resolved to \relax through \csname and typeset
//     "[b]" on the page, ahead of the caption;
//   - with no \@captype, a \caption inside one produced "by1:" and leaked "\the@captype".
//     On corpus paper 2402.04711 the lost tokens are exactly "\the@captype" (three times),
//     "[!ht]" and "by1:", and the gained ones are "3.10:", "3.9.", "3.7," — real numbers.
//
// Three corpus papers each. Measured with measure sweeppair, whose control is HEAD^ by
// construction: Sigma 330 -> 330, +106 glyphs, nothing lost.
func TestThreeparttableConsumesItsArgumentAndNumbersItsCaption(t *testing.T) {
	const src = `\documentclass{article}\usepackage{threeparttable}\begin{document}BEFORE` +
		`\begin{table}\begin{threeparttable}[b]\caption{THECAPTION}` +
		`\begin{tabular}{ll}AA & BB\\\end{tabular}` +
		`\begin{tablenotes}\item[a] NOTEA\end{tablenotes}` +
		`\end{threeparttable}\end{table}AFTER\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, env := range []string{"threeparttable", "tablenotes"} {
		if got := e.Diagnostics().UndefinedEnvs[env]; got != 0 {
			t.Errorf("%s still reported undefined (%d)", env, got)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if strings.Contains(text, "[b]") {
		t.Error("the position argument was typeset")
	}
	// The caption has to carry a NUMBER, which is what \@captype decides.
	if !strings.Contains(text, "Table 1: THECAPTION") {
		t.Errorf("the caption is not numbered as a table; page reads %q", firstN(text, 70))
	}
	for _, want := range []string{"BEFORE", "AA", "NOTEA", "AFTER"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page", want)
		}
	}
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
