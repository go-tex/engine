// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// baselineYs is the DISTINCT text baselines on an SVG page, top to bottom. One
// <tspan> is emitted per word, so a line of four words puts its baseline in the
// page four times — reading the list raw made "the second baseline" the second word
// of the first line, and every gap came out 0.00.
func baselineYs(t *testing.T, svg string) []float64 {
	t.Helper()
	re := regexp.MustCompile(`<tspan[^>]*\sy="([0-9.]+)"`)
	seen := map[float64]bool{}
	var ys []float64
	for _, m := range re.FindAllStringSubmatch(svg, -1) {
		v, err := strconv.ParseFloat(m[1], 64)
		if err != nil || seen[v] {
			continue
		}
		seen[v] = true
		ys = append(ys, v)
	}
	sort.Float64s(ys)
	return ys
}

// ⛔⛔ \@vspacer SAVES AND RESTORES \prevdepth AROUND ITS RULE, and those two lines
// are the difference between a paragraph that stays on the grid and one that rides
// 2.58pt too high.
//
//	\def\@vspacer#1{\ifvmode
//	  \dimen@\prevdepth              latex.ltx:6586
//	  \hrule \@height\z@ \nobreak
//	  \@vspace@calcify{#1}\vskip\z@skip
//	  \prevdepth\dimen@              latex.ltx:6591
//
// A rule sets prev_depth := ignore_depth (tex.web:21025), which suppresses the
// interline glue before the next box. That part this engine had right. \@vspacer
// then puts the depth back, and that part it did not, so every `\vspace*` swallowed
// one `\glue(\baselineskip)`.
//
// The oracle on `Premiere ligne du document.\par \vspace*{50pt} Apres le vspace
// etoile.\par` in article, the two baselines:
//
//	tectonic   134.76 → 196.53   (62.0)
//	before     134.74 → 193.93   (59.19)
//	after      134.74 → 196.51   (61.77)
//
// and the reference's own node list names the node that was missing:
// `\glue(\baselineskip) 4.95` between the `\vskip` and the `\hbox`.
//
// ⭐ THE ASSERTION IS FONT-INDEPENDENT, which is why it is written as a difference
// of differences rather than as a number of points. Without the star's glue two
// paragraphs sit one \baselineskip apart; with it they sit one \baselineskip plus
// the glue apart. So `\vspace*{50pt}` must add EXACTLY 50pt to the gap — no more,
// and no less. Before the fix it added 47.19pt, having paid for the rule with the
// interline glue it destroyed.
func TestVspaceStarAddsExactlyItsGlueAndKeepsTheBaselineskip(t *testing.T) {
	gap := func(src string) float64 {
		pages, err := CompileToSVGPages([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 1 {
			t.Fatalf("%d pages, want 1", len(pages))
		}
		ys := baselineYs(t, pages[0])
		if len(ys) < 2 {
			t.Fatalf("%d baselines on the page, want at least 2", len(ys))
		}
		return ys[1] - ys[0]
	}

	const without = `Premiere ligne du document.\par Apres le vspace etoile.\par`
	const with = `Premiere ligne du document.\par \vspace*{50pt} Apres le vspace etoile.\par`

	added := gap(with) - gap(without)
	if added < 49.9 || added > 50.1 {
		t.Errorf("\\vspace*{50pt} ajoute %.2fpt à l'écart des lignes, want 50.00: "+
			"le filet de \\@vspacer met \\prevdepth à ignore_depth et il faut le "+
			"RESTAURER (latex.ltx:6586, 6591), sinon la colle d'interligne de la "+
			"ligne suivante est perdue avec", added)
	}
}

// ⛔ THE CONTROL that keeps the assertion above about \prevdepth rather than about
// arithmetic: the UNSTARRED form has no rule to restore anything around, and it must
// add exactly the same 50pt. If it did not, the test above would be measuring the
// glue scan rather than the depth.
func TestVspaceAddsExactlyItsGlueToo(t *testing.T) {
	gap := func(src string) float64 {
		pages, err := CompileToSVGPages([]byte(src), Options{})
		if err != nil {
			t.Fatal(err)
		}
		ys := baselineYs(t, pages[0])
		if len(ys) < 2 {
			t.Fatalf("%d baselines on the page, want at least 2", len(ys))
		}
		return ys[1] - ys[0]
	}
	const without = `Premiere ligne du document.\par Apres le vspace.\par`
	const with = `Premiere ligne du document.\par \vspace{50pt} Apres le vspace.\par`
	if added := gap(with) - gap(without); added < 49.9 || added > 50.1 {
		t.Errorf("\\vspace{50pt} ajoute %.2fpt, want 50.00", added)
	}
}
