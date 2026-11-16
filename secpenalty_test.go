// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⭐ A HEADING CARRIES \@secpenalty BEFORE IT, AND IT IS A PENALTY ON THE PAGE.
//
// \addpenalty was an empty no-op, so \@startsection's
// \addpenalty\@secpenalty\addvspace\@tempskipa (ltsect.dtx:331) contributed its
// skip and lost its penalty. \@secpenalty is -300 (ltsect.dtx:229-230).
//
// Measured with \showlists on `Avant.\section{Titre}Texte.`, go-tex/engine#691:
//
//	tectonic            ours, before
//	\penalty -300       (nothing)
//	\glue 15.08499…     glue w=16.555
//	…                   …
//
// ⛔ THE PENALTY IS NEGATIVE, SO IT ENCOURAGES A PAGE BREAK BEFORE A HEADING —
// which is the point: TeX prefers to break there rather than two lines later,
// and without it the page builder had no reason to prefer the one place a reader
// would choose.
//
// ⚠ This test asserts the NODE, not the pagination. A penalty of -300 changes
// where pages fall only when it competes with another candidate, so a witness
// built on a page count would pass on a document that never breaks and say
// nothing. The node arriving is the thing \addpenalty owes.
func TestASectionContributesItsSecPenalty(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`Avant.\section{Titre}Texte.\end{document}`); err != nil {
		t.Fatalf("run: %v", err)
	}

	// The control FIRST: the heading has to be on the list at all, or an empty
	// document would pass the penalty assertion by having no counter-example.
	var boxes int
	for _, n := range e.mvl {
		if _, ok := n.(*boxNode); ok {
			boxes++
		}
	}
	if boxes == 0 {
		t.Fatalf("no box on the main vertical list, so this test is not measuring a "+
			"sectioned document: %d node(s)", len(e.mvl))
	}

	var got []int
	for _, n := range e.mvl {
		if p, ok := n.(penaltyNode); ok {
			got = append(got, p.penalty)
		}
	}
	t.Logf("penalties on the list: %v (over %d nodes, %d box(es))", got, len(e.mvl), boxes)

	found := false
	for _, p := range got {
		if p == -300 {
			found = true
		}
	}
	if !found {
		t.Errorf("no \\penalty -300 on the vertical list: \\addpenalty dropped "+
			"\\@secpenalty, so a page may break two lines after a heading where the "+
			"reference breaks before it (ltsect.dtx:331, :229-230). Penalties seen: %v", got)
	}
}
