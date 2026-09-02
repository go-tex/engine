// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"strings"
	"testing"
)

// These tests pin the SVG a basic tikzpicture renders through the real pgf/TikZ
// sources and the engine's own system-layer driver (texmf/pgfsys-gotex.def). They
// are the counterpart of the gobble tests (tikzgobble_test.go), which prove that
// WITHOUT GOTEX_PGF a picture is stood in for and its body never leaks: here, WITH
// it, the same primitives reach the page as vector paths.
//
// pgf's own sources are not embedded — a host supplies them on GOTEX_TEXMF — so
// like the other rendering tests these skip when that tree is absent. Point
// GOTEX_TEXMF at a directory holding tikz.code.tex (a flattened pgf tree) to run
// them.
//
// The geometry is exact and worth reading: TikZ works in centimetres and pgf
// converts to big points itself, so 1cm is 28.45274bp and 2cm is 56.90549bp. A
// picture opens one scope that maps pgf's coordinates onto the page
// (scale(0.996264,-0.996264) after a translate); colour and line width are set on
// nested <g> scopes, not on the path element.

// renderPicture compiles a tikzpicture body under the real sources and returns the
// concatenated SVG of every page.
func renderPicture(t *testing.T, body string) string {
	t.Helper()
	if os.Getenv("GOTEX_TEXMF") == "" {
		t.Skip("sources pgf absentes : définir GOTEX_TEXMF sur un arbre qui contient tikz.code.tex")
	}
	t.Setenv("GOTEX_PGF", "1")
	e, err := buildEngine(Options{Lenient: true, NoProgressLimit: NoProgressLimitHeavy}, true)
	if err != nil {
		t.Fatal(err)
	}
	src := `\documentclass{article}\usepackage{tikz}\begin{document}` +
		`\begin{tikzpicture}` + body + `\end{tikzpicture}\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%s : %v", body, err)
	}
	return strings.Join(e.RenderPages(72), "")
}

// A straight \draw between two coordinates becomes one open SVG path, in exact
// big-point geometry, painted with no fill.
func TestTikzRenderStraightLine(t *testing.T) {
	svg := renderPicture(t, `\draw (0,0) -- (2,1);`)
	if want := `<path d="M 0.0 0.0 L 56.90549 28.45274" fill="none"/>`; !strings.Contains(svg, want) {
		t.Errorf("le tracé rectiligne %q est absent :\n%s", want, svg)
	}
	// The picture's coordinate scope is what makes the numbers land where they mean.
	if !strings.Contains(svg, `scale(0.996264,-0.996264)`) {
		t.Errorf("le repère de l'image (bp→pt, y inversé) est absent :\n%s", svg)
	}
}

// A \draw ... rectangle becomes a single closed path: the four corners and a Z.
// pgf emits its own current-point movetos around the outline; what matters is that
// the closed outline, in exact geometry, reaches the page.
func TestTikzRenderRectangle(t *testing.T) {
	svg := renderPicture(t, `\draw (0,0) rectangle (2,1);`)
	for _, want := range []string{
		`L 0.0 28.45274`,      // up the left side (1cm)
		`L 56.90549 28.45274`, // across the top (2cm × 1cm)
		`L 56.90549 0.0`,      // down the right side
		`Z`,                   // closed
		`fill="none"`,         // an unfilled \draw
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("le rectangle ne contient pas %q :\n%s", want, svg)
		}
	}
}

// \fill[red] paints the interior: the shape is wrapped in a colour scope that sets
// both stroke and fill, and the path itself is drawn with stroke="none" so only the
// fill shows. This is the discriminating case against a \draw, which is fill="none".
func TestTikzRenderFilledColour(t *testing.T) {
	svg := renderPicture(t, `\fill[red] (0,0) rectangle (1,1);`)
	if want := `fill="#f00"`; !strings.Contains(svg, want) {
		t.Errorf("la couleur de remplissage %q est absente :\n%s", want, svg)
	}
	if want := `stroke="none"`; !strings.Contains(svg, want) {
		t.Errorf("un chemin rempli doit être tracé sans contour (%q) :\n%s", want, svg)
	}
	// The corner is at 1cm on each axis.
	if want := `L 28.45274 28.45274`; !strings.Contains(svg, want) {
		t.Errorf("le coin du rectangle rempli (%q) est absent :\n%s", want, svg)
	}
}

// A line-width option reaches the driver as a stroke-width scope: thick is 0.8pt
// where the default rule is 0.4pt, and a colour option sets the stroke colour.
func TestTikzRenderLineWidthAndColour(t *testing.T) {
	svg := renderPicture(t, `\draw[thick,blue] (0,0) -- (1,0);`)
	for _, want := range []string{
		`stroke-width="0.8"`, // thick
		`stroke="#00f"`,      // blue
		`<path d="M 0.0 0.0 L 28.45274 0.0" fill="none"/>`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("le tracé épais bleu ne contient pas %q :\n%s", want, svg)
		}
	}
}

// A node's text is set inside the picture and reaches the page as glyph paths, each
// an outline placed by its own transform. Two letters give two such paths.
func TestTikzRenderNodeText(t *testing.T) {
	svg := renderPicture(t, `\draw (0,0) node {Hi};`)
	if got := strings.Count(svg, `<path transform=`); got < 2 {
		t.Errorf("le texte du nœud « Hi » a produit %d glyphes, en attendait au moins 2 :\n%s", got, svg)
	}
}
