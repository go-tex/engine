// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \@toproom IS CHARGED THE SEPARATION, AND THE FIRST FLOAT'S IS \textfloatsep.
//
// ltoutput.dtx:6016-6018 refuses a top float unless the room left STRICTLY exceeds its
// height, and 6009 decrements \@toproom by the height plus
//
//	\ifx #2\@empty \textfloatsep \else \floatsep \fi
//
// — \textfloatsep (20pt) when the top list is still empty, \floatsep (12pt) after
// that. This engine charged \floatsep BETWEEN floats and nothing for the first, so its
// top budget was 8pt more generous than LaTeX's on every page that carries floats.
//
// The fixture makes the two rules disagree, which is the only thing that makes it a
// test. \topfraction\@colht is 0.7 x 550 = 385pt; the floats are 200pt and 170pt tall,
// captionless so their heights are the rules':
//
//	LaTeX   used after the first = 200 + 20 = 220, room = 165, and 165 > 170 is FALSE
//	        → the second float is deferred
//	before  want = 200 + 12 + 170 = 382 <= 385 → the second float rides page 1
//
// 8pt decides it, which is exactly \textfloatsep − \floatsep.
func TestTopRoomChargesTextfloatsepForTheFirstFloat(t *testing.T) {
	t.Setenv("GOTEX_FLOATS", "1")
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		strings.Repeat(`Opening paragraph. `, 6) + `\par` +
		`\begin{figure}\rule{1pt}{200pt}\end{figure}` +
		strings.Repeat(`Middle paragraph. `, 6) + `\par` +
		`\begin{figure}\rule{1pt}{170pt}\end{figure}` +
		strings.Repeat(`Closing paragraph. `, 6) + `\par`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	pages := e.Pages()
	if len(pages) < 2 {
		t.Fatalf("%d page(s): les deux flottants et le texte ne tiennent pas sur une page "+
			"une fois le second reporté", len(pages))
	}
	// Each float is a 1pt-wide rule, so counting rules per page counts floats.
	first, second := rulesOnPage(pages[0].list), rulesOnPage(pages[1].list)
	if first != 1 {
		t.Errorf("page 1 porte %d filet(s), want 1: le second flottant doit être reporté — "+
			"165pt de place restante ne dépasse pas 170pt de flottant", first)
	}
	if second != 1 {
		t.Errorf("page 2 porte %d filet(s), want 1 (le flottant reporté)", second)
	}
}

func rulesOnPage(list []node) int {
	n := 0
	var walk func([]node)
	walk = func(l []node) {
		for _, x := range l {
			switch c := x.(type) {
			case ruleNode:
				n++
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(list)
	return n
}
