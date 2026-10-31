// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔⛔ A PARAGRAPH'S OPENING GLUE BELONGS TO ITS FIRST LINE, AND IT TOOK TWO HALVES.
//
// tex.web's post_line_break (17273-17279) justifies a line and appends it, and only
// THEN prunes the head of the next one; 17297 says why — the pruning drops the glue
// the break consumed. A paragraph's first line follows no break and keeps whatever the
// paragraph began with. The other end is 17001: `prev_p:=cur_p; {glue at beginning is
// not a legal breakpoint}`.
//
// Neither half sufficed alone, which was measured rather than argued (#657):
//
//	go-typeset/linebreak v1.2.1 alone   the witness still read 133.77
//	this exemption alone                still 133.77
//	both                                153.70 against the reference's 153.69
func TestOpeningGlueSurvivesInTheFirstLine(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	// \noindent so the glue really is the first node: with an indent box in front of
	// it the box shields it and nothing shows, which is why only half of the original
	// witness failed.
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\noindent\hskip 20pt apres\par`); err != nil {
		t.Fatal(err)
	}
	line := firstNonEmptyLine(e.mvl)
	if line == nil {
		t.Fatal("aucune ligne produite")
	}
	g, ok := line.list[0].(glueNode)
	if !ok {
		t.Fatalf("la ligne commence par %T, want la colle de \\hskip", line.list[0])
	}
	if g.spec.width != ptToSP(20) {
		t.Errorf("colle de tête = %d, want %d (20pt)", g.spec.width, ptToSP(20))
	}
}

// ⛔⛔ AND NOT EVERY FRAGMENT'S FIRST LINE — only the PARAGRAPH's.
//
// \\ is a forced penalty inside the paragraph (latex.go, \def\\{\penalty-10000 }), and
// endParagraph splits the list at those penalties into fragments laid out one by one.
// Exempting every fragment's head would keep glue that follows a break: a probe showed
// the three fragments of `A\\ \hspace*{50pt}B\\ \hspace{50pt}C` arriving as three calls
// with i=0 each, and TestHspaceStarSurvivesALineBreak failed exactly that way.
//
// So: the glue after the \\ must be gone, and the \\ must not have ended the paragraph
// either — both are asserted, because an implementation that made \\ a \par would pass
// the first assertion for the wrong reason.
func TestGlueAfterAForcedBreakIsStillDropped(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\noindent A\\ \hskip 50pt B\par`); err != nil {
		t.Fatal(err)
	}
	var lines []*boxNode
	for _, n := range e.mvl {
		if b, ok := n.(*boxNode); ok && b != nil && len(b.list) > 0 {
			lines = append(lines, b)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("%d ligne(s), want 2: le \\\\ coupe la ligne, il ne doit pas "+
			"terminer le paragraphe", len(lines))
	}
	if _, isGlue := lines[1].list[0].(glueNode); isGlue {
		t.Error("la ligne après le \\\\ commence par une colle: celle qui suit une " +
			"coupe forcée doit être jetée (tex.web:17297)")
	}
}

func firstNonEmptyLine(mvl []node) *boxNode {
	for _, n := range mvl {
		if b, ok := n.(*boxNode); ok && b != nil && len(b.list) > 0 {
			return b
		}
	}
	return nil
}
