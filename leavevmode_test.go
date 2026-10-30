// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ \rule LEAVES VERTICAL MODE, AND THE PARAGRAPH IT STARTS IS INDENTED.
//
// tex.web:21104-21109 lists the commands that, met in vertical mode, back the input up
// and call new_graf(TRUE) — among them vmode+vrule and vmode+un_hbox. LaTeX's \rule is
// \leavevmode followed by an \hbox holding a \vrule (ltboxes.dtx:1498-1505), and
// \leavevmode is \unhbox\voidb@x, so both routes reach the same place.
//
// Without it the rule was contributed straight to the VERTICAL list: it became an item
// of its own rather than a line's content, and — because nothing went through
// append_to_vlist — \prevdepth was never armed, so the next box lost its interline
// glue (tex.web:13318-13326). Measured against tectonic on a float whose body is a
// \rule: 219.52pt here against 224.05998 there, short by the 12 − 0 − 6.94 = 5.06pt of
// \baselineskip the caption should have had (go-tex/engine#651).
func TestRuleLeavesVerticalMode(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\rule{20pt}{6pt} apres le filet\par`); err != nil {
		t.Fatal(err)
	}
	// ⛔ The rule must be INSIDE a line, not beside one. A bare rule in the vertical
	// list is exactly the defect: it typesets, it is visible, and it silently costs
	// the next box its interline glue.
	for _, n := range e.mvl {
		if _, bare := n.(ruleNode); bare {
			t.Fatalf("filet NU dans la liste verticale: \\rule n'a pas quitté le mode "+
				"vertical (mvl = %d nœuds)", len(e.mvl))
		}
	}
	line := firstLineBox(e.mvl)
	if line == nil {
		t.Fatal("aucune ligne produite")
	}
	if n := countRuleNodes(line.list); n != 1 {
		t.Errorf("la ligne porte %d filet(s), want 1: le filet doit être le contenu "+
			"de la ligne", n)
	}
	// And the paragraph is indented: new_graf(TRUE) puts a \parindent-wide empty box
	// at the head (tex.web:21124-21126).
	if !startsWithIndentBox(line.list) {
		t.Errorf("la ligne ne commence pas par la boîte de \\parindent: "+
			"new_graf(true) est indenté (tex.web:21108, 21124-21126); tête = %T",
			headOf(line.list))
	}
}

// ⛔ \leavevmode indents, so a \parbox or a minipage opening a paragraph sits
// \parindent in, not flush left. Measured on disc/indent.tex: the reference puts the
// box at x=148.71 and we used to put it at 133.77, the 15pt of \parindent exactly.
// A document that wants it flush says \noindent, and that path was never broken.
func TestLeavevmodeIndents(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\parbox{40pt}{xx} apres\par`); err != nil {
		t.Fatal(err)
	}
	line := firstLineBox(e.mvl)
	if line == nil {
		t.Fatal("aucune ligne produite")
	}
	if !startsWithIndentBox(line.list) {
		t.Errorf("un \\parbox en tête de paragraphe doit être indenté; tête = %T",
			headOf(line.list))
	}

	// ⛔ The control: \noindent must still produce a flush line, or the fix would have
	// traded one wrong position for another.
	e2 := New()
	if err := e2.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\documentclass{article}\begin{document}` +
		`\noindent\parbox{40pt}{xx} apres\par`); err != nil {
		t.Fatal(err)
	}
	if l := firstLineBox(e2.mvl); l == nil || startsWithIndentBox(l.list) {
		t.Error("\\noindent doit rester à fleur de marge")
	}
}

// vmode+hskip is in the same list (tex.web:21106), so an \hskip met in vertical mode
// starts an indented paragraph too.
func TestHskipInVerticalModeIndents(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\hskip 20pt apres\par`); err != nil {
		t.Fatal(err)
	}
	line := firstLineBox(e.mvl)
	if line == nil {
		t.Fatal("aucune ligne produite")
	}
	if !startsWithIndentBox(line.list) {
		t.Errorf("un \\hskip en mode vertical ouvre un paragraphe INDENTÉ "+
			"(tex.web:21106, 21108); tête = %T", headOf(line.list))
	}
}

func headOf(list []node) node {
	if len(list) == 0 {
		return nil
	}
	return list[0]
}

// startsWithIndentBox reports whether the line opens with new_graf's empty
// \parindent-wide box: width > 0, nothing in it.
func startsWithIndentBox(list []node) bool {
	b, ok := headOf(list).(*boxNode)
	return ok && b != nil && len(b.list) == 0 && b.width > 0
}

func firstLineBox(mvl []node) *boxNode {
	for _, n := range mvl {
		if b, ok := n.(*boxNode); ok && b != nil && len(b.list) > 0 {
			return b
		}
	}
	return nil
}

func countRuleNodes(list []node) int {
	n := 0
	var walk func([]node)
	walk = func(l []node) {
		for _, x := range l {
			switch c := x.(type) {
			case ruleNode:
				n++
			case *boxNode:
				if c != nil {
					walk(c.list)
				}
			}
		}
	}
	walk(list)
	return n
}

// Keeps the import honest if the assertions above ever stop needing it.
var _ = strings.Contains
