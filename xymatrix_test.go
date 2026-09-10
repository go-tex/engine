// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

func TestXymatrixSourceTakesOnlyAWholeFormula(t *testing.T) {
	for _, c := range []struct{ src, opts, body string }{
		{`\xymatrix{A & B}`, "", "A & B"},
		{`  \xymatrix{A}  `, "", "A"},
		{`\xymatrix@C=1pc@R=2pc{A \\ B}`, "@C=1pc@R=2pc", `A \\ B`},
		{`\xymatrix{ {A} & {B} }`, "", " {A} & {B} "},
	} {
		opts, body, ok := xymatrixSource(c.src)
		if !ok || opts != c.opts || body != c.body {
			t.Errorf("xymatrixSource(%q) = (%q,%q,%v), want (%q,%q,true)", c.src, opts, body, ok, c.opts, c.body)
		}
	}
	// A diagram embedded in a larger formula is NOT taken: the surrounding
	// material would have to be laid out around it, and saying so plainly is
	// better than drawing half of it.
	for _, src := range []string{
		`x = \xymatrix{A & B}`,
		`\xymatrix{A & B} + 1`,
		`\frac{a}{b}`,
		`\xymatrix`,
		``,
	} {
		if _, _, ok := xymatrixSource(src); ok {
			t.Errorf("xymatrixSource(%q) was taken", src)
		}
	}
}

// Every number in the layout was read off tectonic. 2pc is the default and the
// separation is measured between cell BOXES, so these are the two that decide
// where everything lands.
func TestXySepDefaultsToTwoPica(t *testing.T) {
	for _, c := range []struct {
		opts string
		key  byte
		want float64
	}{
		{"", 'C', 24}, {"", 'R', 24},
		{"@C=1pc", 'C', 12}, {"@C=3pc", 'C', 36},
		{"@C=1pc@R=2pc", 'R', 24},
		{"@R=10pt", 'R', 10},
		{"@C=0pc", 'C', 0},
		{"@!0", 'C', 24}, // an option this does not read leaves the default
	} {
		if got := xySep(c.opts, c.key); got != c.want {
			t.Errorf("xySep(%q,%c) = %g, want %g", c.opts, c.key, got, c.want)
		}
	}
}

func TestParseXymatrixGrid(t *testing.T) {
	g := parseXymatrix(`A \ar[r] & B \\ C & D`)
	if len(g) != 2 || len(g[0]) != 2 || len(g[1]) != 2 {
		t.Fatalf("grid is %v", g)
	}
	if got := strings.TrimSpace(g[0][0].math); got != "A" {
		t.Errorf("the \\ar was left in the maths: %q", got)
	}
	if len(g[0][0].arrows) != 1 || g[0][0].arrows[0].dc != 1 || g[0][0].arrows[0].dr != 0 {
		t.Errorf("arrow = %+v, want one pointing right", g[0][0].arrows)
	}
	if len(g[1][1].arrows) != 0 || strings.TrimSpace(g[1][1].math) != "D" {
		t.Errorf("the last cell is %+v", g[1][1])
	}
}

// The forms the reported diagram uses, and the ones a paper reaches for next.
func TestParseXyArrowStylesAndLabels(t *testing.T) {
	g := parseXymatrix(`A \ar@{^(->}[r]\ar[d]_f & B \ar@{=}[d] \\ C \ar[r]^{g} & D`)
	a := g[0][0].arrows
	if len(a) != 2 {
		t.Fatalf("cell A has %d arrows, want 2", len(a))
	}
	if a[0].style != "^(->" || a[0].dc != 1 {
		t.Errorf("the hooked arrow is %+v", a[0])
	}
	if a[1].dr != 1 || len(a[1].labels) != 1 || a[1].labels[0].text != "f" || a[1].labels[0].side != '_' {
		t.Errorf("the labelled arrow is %+v", a[1])
	}
	if g[0][1].arrows[0].style != "=" {
		t.Errorf("the equality is %+v", g[0][1].arrows[0])
	}
	if l := g[1][0].arrows[0].labels; len(l) != 1 || l[0].text != "g" || l[0].side != '^' {
		t.Errorf("a braced label read as %+v", l)
	}
}

// A direction is a run of letters, so [rr] is two columns and [dr] is a diagonal.
func TestXyDirections(t *testing.T) {
	for src, want := range map[string][2]int{
		`A \ar[r] & B`:  {0, 1},
		`A \ar[l] & B`:  {0, -1},
		`A \ar[d] & B`:  {1, 0},
		`A \ar[u] & B`:  {-1, 0},
		`A \ar[rr] & B`: {0, 2},
		`A \ar[dr] & B`: {1, 1},
		`A \ar[ul] & B`: {-1, -1},
	} {
		a := parseXymatrix(src)[0][0].arrows
		if len(a) != 1 || a[0].dr != want[0] || a[0].dc != want[1] {
			t.Errorf("%s: %+v, want dr=%d dc=%d", src, a, want[0], want[1])
		}
	}
}

func TestReadXyStyle(t *testing.T) {
	for style, want := range map[string]struct {
		tail, head     int
		double, dashed bool
	}{
		"":     {0, 1, false, false},
		"->":   {0, 1, false, false},
		"-":    {0, 0, false, false},
		"=":    {0, 0, true, false},
		"=>":   {0, 1, true, false},
		"->>":  {0, 2, false, false},
		"^(->": {1, 1, false, false},
		"_(->": {-1, 1, false, false},
		".>":   {0, 1, false, true},
		"-->":  {0, 1, false, true},
	} {
		tail, head, double, dashed := readXyStyle(style)
		if tail != want.tail || head != want.head || double != want.double || dashed != want.dashed {
			t.Errorf("readXyStyle(%q) = (%d,%d,%v,%v), want %+v", style, tail, head, double, dashed, want)
		}
	}
}

// The whole thing, end to end: the reported diagram draws, nothing is dropped,
// and the maths layer is not asked to make sense of \xymatrix.
func TestTheReportedDiagramDraws(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	src := `\documentclass{article}\begin{document}` +
		`\[\xymatrix{` + "\n" +
		`A \ar@{^(->}[r]\ar[d]_f & B \ar@{=}[d] \\` + "\n" +
		`C \ar[r] & D}\]` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	if n := len(e.mathDropped); n != 0 {
		t.Errorf("the maths layer was asked and refused: %v", e.mathDropped)
	}
	svg := strings.Join(e.RenderPages(72), "")
	// Four objects and the lines between them: A, B, C, D and f are glyph paths,
	// the shafts and heads are filled paths of this file's making.
	if n := strings.Count(svg, "<path"); n < 12 {
		t.Errorf("%d paths on the page — the diagram did not draw:\n%s", n, svg)
	}
	for _, want := range []string{"<svg", `fill="black"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}

func TestBoxExitLeavesAtTheEdge(t *testing.T) {
	p := placed{cx: 10, cy: 20, halfW: 4, halfH: 3}
	for _, c := range []struct{ dx, dy, wx, wy float64 }{
		{1, 0, 14, 20},  // due east: the vertical edge
		{-1, 0, 6, 20},  // due west
		{0, 1, 10, 23},  // due south: the horizontal edge
		{0, -1, 10, 17}, // due north
	} {
		x, y := boxExit(p, c.dx, c.dy)
		if x != c.wx || y != c.wy {
			t.Errorf("boxExit(%g,%g) = (%g,%g), want (%g,%g)", c.dx, c.dy, x, y, c.wx, c.wy)
		}
	}
	// A diagonal leaves through whichever edge it reaches first.
	if x, y := boxExit(p, 1, 1); x != 13 || y != 23 {
		t.Errorf("a diagonal left at (%g,%g), want (13,23)", x, y)
	}
}
