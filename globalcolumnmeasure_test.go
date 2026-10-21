// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \twocolumn writes the column measure GLOBALLY (latex.ltx:15108-15115: six
// assignments, every one \global). A class may call it from inside a group, and in
// TeX a global assignment discards the values the enclosing groups saved for that
// register — so the measure survives their \endgroup. acmart does exactly this: it
// sets \hsize=\textwidth at acmart.cls:2101 and calls \@printtopmatter — which is
// \twocolumn[\box\mktitle@bx] — four lines later, both inside the \begingroup…
// \endgroup that spans \maketitle (acmart.cls:2095-2191).
//
// ⛔ The witness must start in ONE column. With \documentclass[twocolumn] the measure
// is already halved when the group opens, so \hsize=\textwidth saves the HALVED value
// and \endgroup restores it either way: the test would pass without the fix and assert
// nothing. The group has to be the thing that saved the FULL width.
func TestTwoColumnMeasureIsGlobal(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\makeatletter\begingroup\hsize=\textwidth\twocolumn\endgroup\makeatother`+
		`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	full := e.oneColHsize
	if full == 0 {
		t.Fatal("le témoin n'a jamais retenu la largeur une colonne")
	}
	want := (full - e.columnsep) / 2
	if e.hsize != want {
		t.Errorf("après \\endgroup, hsize=%d, want %d (pleine largeur: %d) — "+
			"le groupe a rendu la mesure", e.hsize, want, full)
	}
}

// And the measure is what the body is actually line-broken at, not merely what the
// register reads: before the fix every column of an acmart paper was broken at the
// full 505.89pt and drawn into a 241pt column, so the two columns lay one over the
// other and a quarter of each line fell off the sheet (go-tex/engine#615).
func TestTextAfterTheGroupIsSetAtTheColumnMeasure(t *testing.T) {
	body := `\makeatletter\begingroup\hsize=\textwidth\twocolumn\endgroup\makeatother ` +
		strings.Repeat("severallines of ordinary prose that has to be broken somewhere ", 12)
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+body+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if txt := mvlText(e.mvl); !strings.Contains(txt, "severallines") {
		t.Fatalf("le document ne porte pas le texte du témoin: %q", txt)
	}
	full := e.oneColHsize
	col := (full - e.columnsep) / 2
	var widest int
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			if b, ok := n.(*boxNode); ok {
				if b.width > widest {
					widest = b.width
				}
				walk(b.list)
			}
		}
	}
	walk(e.mvl)
	if widest > col {
		t.Errorf("la ligne la plus large fait %d, la colonne %d (pleine largeur %d)",
			widest, col, full)
	}
}
