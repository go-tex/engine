// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// firstMath returns the first math box placed on the page, so a formula's geometry
// can be compared against another formula's rather than against a number I chose.
func firstMath(nodes []node) (mathNode, bool) {
	for _, n := range nodes {
		switch v := n.(type) {
		case mathNode:
			return v, true
		case *boxNode:
			if m, ok := firstMath(v.list); ok {
				return m, true
			}
		}
	}
	return mathNode{}, false
}

// mathGeom compiles one formula and returns its box, with the leftindex package
// requested.
func mathGeom(t *testing.T, body string) mathNode {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath,leftindex}`+
		`\begin{document}`+body+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatalf("%s: %v", body, err)
	}
	if len(e.mathDropped) != 0 {
		t.Fatalf("%s: the math layer refused it: %v", body, e.mathDropped)
	}
	m, ok := firstMath(e.mvl)
	if !ok {
		m, ok = firstMath(e.parList)
	}
	if !ok {
		t.Fatalf("%s: no math box on the page", body)
	}
	return m
}

// leftindex.sty sets a symbol's indices on its LEFT. It cannot be embedded — it is
// expl3, requires xparse and mathtools, and its signature is
//
//	leftindex.sty:12  \DeclareDocumentCommand\leftindex { o o E{^_}{{}{}} m }
//
// which no \newcommand can express: two optional bracket phantoms, then ^ and _ in
// EITHER ORDER and either omitted, then the symbol. So the engine rewrites it into
// TeX's own left-index idiom, an empty nucleus carrying the scripts, which
// go-tex/math already renders.
//
// The test is an EQUIVALENCE against that idiom rather than a table of widths: a
// number of my own choosing would only pin today's metrics, while {}_{a}I is what
// the package's own output reduces to.
//
// 27 equations on one arXiv paper carried \leftindex, and one unknown command drops
// the whole equation.
func TestLeftIndexMatchesTheEmptyNucleusIdiom(t *testing.T) {
	for _, c := range [][2]string{
		{`$\leftindex_a I$`, `${}_{a}I$`},
		{`$\leftindex^b J$`, `${}^{b}J$`},
		{`$\leftindex^b_a K$`, `${}^{b}_{a}K$`},
		// The embellishments are order-free, which is what E{^_} means and what a
		// fixed-order reading would get wrong.
		{`$\leftindex_a^b K$`, `${}^{b}_{a}K$`},
		// The two optional phantoms only tune kerning, so dropping them must leave
		// the same box as not writing them.
		{`$\leftindex[x][y]_a I$`, `${}_{a}I$`},
		// No embellishment at all is legal: E's default is {{}{}}.
		{`$\leftindex{N}$`, `$N$`},
		// The symbol keeps its OWN right-hand scripts.
		{`$\leftindex_a I^{c}$`, `${}_{a}I^{c}$`},
	} {
		got, want := mathGeom(t, c[0]), mathGeom(t, c[1])
		if got.width != want.width || got.height != want.height || got.depth != want.depth {
			t.Errorf("%s = %d/%d/%d, %s gives %d/%d/%d",
				c[0], got.width, got.height, got.depth,
				c[1], want.width, want.height, want.depth)
		}
	}
}

// A left index must actually WIDEN the symbol. The equivalence above would still
// hold if both sides rendered the bare symbol and dropped the indices on the floor,
// which is the failure this catches.
func TestLeftIndexIsNotSilentlyDiscarded(t *testing.T) {
	bare := mathGeom(t, `$I$`)
	for _, body := range []string{`$\leftindex_a I$`, `$\leftindex^b I$`, `$\leftindex^b_a I$`} {
		if got := mathGeom(t, body); got.width <= bare.width {
			t.Errorf("%s is %d wide, no wider than $I$ at %d — the index was dropped",
				body, got.width, bare.width)
		}
	}
}

// Without \usepackage{leftindex} nothing is rewritten, as resolvePhysics does for the
// physics package: a document that never asked for the package must not have its
// \leftindex reinterpreted, because the name could be its own.
func TestLeftIndexIsGatedOnThePackage(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\usepackage{amsmath}`+
		`\begin{document}$\leftindex_a I$\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.mathDropped) == 0 {
		t.Error(`\leftindex was rewritten without \usepackage{leftindex}`)
	}
}
