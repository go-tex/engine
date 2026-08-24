// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
	"testing"
)

// \tikz \draw …; — the form with no braces — is handled by \tikz@@single, which
// installs a local \tikz@path@do@at@end holding the \endgroup \endtikzpicture
// \endgroup that ends the picture. \tikz@@command@path then opens a group of its
// own and overrides that hook inside it, and \tikz@finish closes that group
// *before* reading the hook, so what runs is the local definition made outside.
// The braced form does not use the hook at all — \tikz@ carries the ending with
// \aftergroup — so it kept working while this one did not, and the two forms
// disagreeing is the sign that the hook was lost.
//
// The picture is the same drawing either way, so the two forms must produce the
// same geometry and leave no group open.
func TestTikzSingleMatchesBracedForm(t *testing.T) {
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("sources pgf absentes : définir GOTEX_TEXMF sur un arbre qui contient tikz.code.tex")
	}
	t.Setenv("GOTEX_PGF", "1")
	const preamble = `\documentclass{article}\usepackage{tikz}`
	run := func(body string) (string, int) {
		t.Helper()
		e, err := buildEngine(Options{}, true)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := e.Run(preamble + body); err != nil {
			t.Fatalf("%s : %v", body, err)
		}
		return strings.Join(e.RenderPages(72), ""), len(e.groups)
	}
	sans, ouverts := run(`X\tikz \draw (0,0)--(1,1);`)
	if ouverts != 0 {
		t.Errorf("la forme sans accolades laisse %d groupes ouverts, attendu 0", ouverts)
	}
	avec, ouvertsB := run(`X\tikz{\draw (0,0)--(1,1);}`)
	if ouvertsB != 0 {
		t.Errorf("la forme avec accolades laisse %d groupes ouverts, attendu 0", ouvertsB)
	}
	if sans != avec {
		t.Errorf("les deux formes rendent différemment\n  sans accolades : %d octets\n  avec accolades : %d octets", len(sans), len(avec))
	}
	// The drawing is one centimetre on each axis, and pgf converts it itself.
	if want := "L 28.45274 28.45274"; !strings.Contains(sans, want) {
		t.Errorf("le tracé diagonal %q est absent du rendu sans accolades", want)
	}
}

// Every node on a path must reach the page, not just the last one. A node voids
// \tikz@figbox inside the group that collects its text (tikz.code.tex:3903) and
// then appends itself to that same box; the nodes already accumulated there come
// back only because the group restores the register. While box registers were
// unscoped here, each node wiped its predecessors and a two-node path drew one
// node.
func TestTikzAllNodesOnAPathAreDrawn(t *testing.T) {
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("sources pgf absentes : définir GOTEX_TEXMF sur un arbre qui contient tikz.code.tex")
	}
	t.Setenv("GOTEX_PGF", "1")
	for _, c := range []struct {
		name, body string
		want       int
	}{
		{"un seul nœud", `\tikz \node {AAAA};`, 4},
		{"deux nœuds sur un chemin", `\tikz \draw (0,0) node {AA} -- (2,0) node {BBBBBB};`, 8},
		{"trois segments, deux nœuds", `\tikz \draw (0,0) node {AA} -- (2,0) -- (2,2) node {BBBBBB};`, 8},
		{"nœuds dans des chemins séparés", `\tikz{\node {AA}; \node at (2,0) {BBBBBB};}`, 8},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := buildEngine(Options{}, true)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := e.Run(`\documentclass{article}\usepackage{tikz}\begin{document}` + c.body + `\end{document}`); err != nil {
				t.Fatal(err)
			}
			svg := strings.Join(e.RenderPages(72), "")
			// One <path transform= per glyph; the folio adds one.
			if got := strings.Count(svg, `<path transform=`); got != c.want+1 {
				t.Errorf("%s\n  %d glyphes rendus, attendu %d (le folio compris)", c.body, got, c.want+1)
			}
		})
	}
}
