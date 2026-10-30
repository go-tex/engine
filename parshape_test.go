// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \parshape 1 <indent> <length> sets every line of the paragraph at that margin and
// that measure. tex.web §16676-16689: "the other 2n words contain the left margins and
// line lengths for the first n lines of the paragraph; the specifications for line n
// apply to all subsequent lines."
//
// ⛔ The assertion is the LEADING GLUE of each line, not the box width. Every line is
// packed to \hsize (paragraph.go), so a walk over box widths reports 345pt for a
// shaped line and 345pt for an unshaped one — the same number before and after the
// fix, i.e. a test that can neither fail for the right reason nor pass for it. The
// indent arrives as a glue node at the head of the line, and that is what differs.
func TestParshapeSetsTheMarginOfEveryLine(t *testing.T) {
	const indent = 25 * unity
	lines := shapedLines(t, `{\parshape 1 25pt 295pt\noindent `+witnessProse(4)+`\par}`)
	if len(lines) < 2 {
		t.Fatalf("%d ligne(s), want au moins 2 — une forme qui ne tient qu'une ligne "+
			"ne dit rien sur « toutes les suivantes »", len(lines))
	}
	for i, ln := range lines {
		g, ok := ln[0].(glueNode)
		if !ok {
			t.Errorf("ligne %d ne commence pas par une glue: %T", i, ln[0])
			continue
		}
		if g.spec.width != indent {
			t.Errorf("ligne %d: marge %.2fpt, want 25pt", i, float64(g.spec.width)/unity)
		}
	}
}

// ⚠ And it is restored at the end of its group: \parshape lives at par_shape_loc in
// TeX's equivalents table (tex.web:4754). Without that, one \begin{quote} would narrow
// every paragraph to the end of the document.
func TestParshapeIsRestoredWithItsGroup(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`{\parshape 1 25pt 295pt\noindent `+witnessProse(3)+`\par}`+
		`\noindent GOTEXAPRES `+witnessProse(3)+`\par`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	after := linesContaining(e, "GOTEXAPRES")
	if len(after) == 0 {
		t.Fatal("le paragraphe d'après n'est pas sur la page")
	}
	if g, ok := after[0][0].(glueNode); ok && g.spec.width == 25*unity {
		t.Error("la ligne d'après porte encore la marge de la forme — elle a fui hors du groupe")
	}
}

// \parshape 0 clears it (tex.web:8512-8513 reads it back as 0).
func TestParshapeZeroClearsIt(t *testing.T) {
	e := New()
	e.parshape = []parshapePair{{indent: 25 * unity, length: 295 * unity}}
	if _, _, ok := e.parshapeMeasure(); !ok {
		t.Fatal("la forme posée à la main n'est pas vue")
	}
	if _, err := e.Run(`\parshape 0 `); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, _, ok := e.parshapeMeasure(); ok {
		t.Error("\\parshape 0 n'a pas effacé la forme")
	}
}

// ⚠ A shape with n > 1 is RECORDED, not approximated: the line breaker takes one
// measure, so honouring a per-line shape would mean changing its signature, and
// silently using one of the pairs would set lines 1…n-1 at a width the document never
// asked for while reporting nothing.
func TestParshapeWithSeveralLinesIsRecordedAsUnimplemented(t *testing.T) {
	e := New()
	if _, err := e.Run(`\parshape 2 0pt 100pt 20pt 80pt `); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var found bool
	for k := range e.skippedCS {
		if strings.Contains(k, "parshape") {
			found = true
		}
	}
	if !found {
		t.Errorf("une forme à 2 lignes n'est pas signalée: %v", e.skippedCS)
	}
	// And it still consumed exactly its 2n dimens: a scanner that stopped early would
	// leave "20pt 80pt" in the stream as text.
	if _, _, ok := e.parshapeMeasure(); !ok {
		t.Error("la première paire n'a pas été retenue")
	}
}

func witnessProse(n int) string {
	return strings.Repeat("severallines of ordinary prose that has to be broken somewhere twice ", n)
}

// shapedLines returns the node lists of the witness's lines, head glue included.
func shapedLines(t *testing.T, body string) [][]node {
	t.Helper()
	return linesOf(shapedLineBoxes(t, body))
}

// shapedLineBoxes returns the witness's line BOXES, so a test can assert the measure
// each one was packed to as well as its margins.
func shapedLineBoxes(t *testing.T, body string) []*boxNode {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	boxes := boxesContaining(e, "severallines")
	if len(boxes) == 0 {
		t.Fatalf("le témoin n'est pas sur la page")
	}
	return boxes
}

// linesOf is the node lists of a set of line boxes.
func linesOf(boxes []*boxNode) [][]node {
	out := make([][]node, 0, len(boxes))
	for _, b := range boxes {
		out = append(out, b.list)
	}
	return out
}

// linesContaining returns the hboxes whose text carries want, as node lists.
func linesContaining(e *Engine, want string) [][]node { return linesOf(boxesContaining(e, want)) }

// boxesContaining returns the line hboxes whose text carries want.
func boxesContaining(e *Engine, want string) []*boxNode {
	var out []*boxNode
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			b, ok := n.(*boxNode)
			if !ok {
				continue
			}
			if b.kind == hbox && len(b.list) > 0 && strings.Contains(mvlText([]node{b}), want) {
				out = append(out, b)
			}
			walk(b.list)
		}
	}
	walk(e.mvl)
	return out
}
