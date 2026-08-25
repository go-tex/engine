// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
	"testing"
)

// The driver declares four events — a picture opening and closing, and a box's
// inverse opening and closing — and {?unmap} answers with the inverse of the
// page maps ACTUALLY in force. Everything the nested-picture fix does is decided
// here, so it is worth testing without pgf in the way.
func TestUnmapAnswersTheMapsInForce(t *testing.T) {
	const pic1 = `<gotex:origin x="10" y="20"/>`
	const pic2 = `<gotex:origin x="30" y="40"/>`
	const endPic = `<gotex:endorigin/>`
	const unmap = `<gotex:unmap/>`
	const endUnmap = `<gotex:endunmap/>`

	for _, c := range []struct{ name, stream, want string }{
		{
			"au sommet, rien n'est en vigueur",
			`{?unmap}`,
			``,
		},
		{
			"une image ouverte",
			pic1 + `{?unmap}`,
			`scale(1.00375,-1.00375)translate(-10,-20)`,
		},
		{
			"deux images imbriquées : les deux sont défaites, la plus intérieure d'abord",
			pic1 + pic2 + `{?unmap}`,
			`scale(1.00375,-1.00375)translate(-30,-40)scale(1.00375,-1.00375)translate(-10,-20)`,
		},
		{
			"une image refermée ne compte plus",
			pic1 + endPic + `{?unmap}`,
			``,
		},
		{
			"l'inverse d'une boîte annule l'image qui l'entoure",
			pic1 + unmap + `{?unmap}`,
			``,
		},
		{
			"…et cesse de l'annuler une fois refermé",
			pic1 + unmap + endUnmap + `{?unmap}`,
			`scale(1.00375,-1.00375)translate(-10,-20)`,
		},
		{
			"une image placée par une boîte : rien à défaire devant elle",
			pic1 + unmap + pic2 + `{?unmap}`,
			`scale(1.00375,-1.00375)translate(-30,-40)`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			var r originResolver
			got := r.next(c.stream)
			if got != c.want {
				t.Errorf("obtenu  %q\nattendu %q", got, c.want)
			}
		})
	}
}

// The declarations are consumed: they are directives to the resolver, never
// output. A stray one left in the stream would be an unknown element in the SVG.
func TestScopeDeclarationsAreConsumed(t *testing.T) {
	var r originResolver
	got := r.next(`A<gotex:origin x="1" y="2"/>B<gotex:unmap/>C<gotex:endunmap/>D<gotex:endorigin/>E`)
	if got != "ABCDE" {
		t.Errorf("obtenu %q, attendu \"ABCDE\"", got)
	}
	if strings.Contains(got, "gotex:") {
		t.Error("une déclaration a survécu dans le flux")
	}
}

// The origin back-references still name the innermost picture, which is what a
// box's own inverse is built from.
func TestOriginBackReferencesFollowTheStack(t *testing.T) {
	var r originResolver
	got := r.next(`<gotex:origin x="10" y="20"/>[{?ox},{?oy}]` +
		`<gotex:origin x="30" y="40"/>[{?-ox},{?-oy}]` +
		`<gotex:endorigin/>[{?ox},{?oy}]`)
	if want := `[10,20][-30,-40][10,20]`; got != want {
		t.Errorf("obtenu %q, attendu %q", got, want)
	}
}

// The whole point, end to end: a picture opened directly inside another must not
// have the page map applied twice. Measured against a real LaTeX, pgfplots' curve
// through (0,0) (1,1) (2,4) RISES; with the map applied twice it fell, because
// the y-flip cancels itself and the plot then lies about its data.
func TestNestedPictureDoesNotFlipTwice(t *testing.T) {
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("sources pgf absentes : définir GOTEX_TEXMF sur un arbre qui contient tikz.code.tex")
	}
	t.Setenv("GOTEX_PGF", "1")
	e, err := buildEngine(Options{Lenient: true, NoProgressLimit: NoProgressLimitHeavy}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\documentclass{article}\usepackage{tikz}\begin{document}` +
		`\tikz \node[draw] {\tikz \draw (0,0) -- (1,1);};\end{document}`); err != nil {
		t.Fatal(err)
	}
	svg := strings.Join(e.RenderPages(72), "")
	// The inner picture is placed by the node's box, whose own inverse already
	// took the page map off, so nothing precedes the inner picture's transform.
	if strings.Contains(svg, `transform="scale(1.00375,-1.00375)translate(`) &&
		strings.Contains(svg, `stroke-miterlimit="10" transform="scale(`) {
		t.Error("une image placée par une boîte s'est vu défaire une carte qui n'était plus en vigueur")
	}
}
