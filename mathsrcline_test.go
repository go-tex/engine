// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"regexp"
	"strings"
	"testing"
)

// ⛔ A FORMULA USED TO CARRY NO SOURCE AT ALL — not a node, not a line.
//
// srcmap.go says the stamps exist for "source ↔ output navigation (click a
// glyph → jump to its line, à la SyncTeX)". collectHSpans skipped mathNode, so
// SourceSpans emitted nothing for a formula and LineAt returned 0 over one; and
// boxrender.go closed the open <g data-l="N"> before embedding the formula, so
// the rendered page carried nothing either. A reader clicking a displayed
// equation to find it in their source was clicking the one thing on the page
// that could not answer.
const mathSrcDoc = `\documentclass{article}
\begin{document}
Before the formula.

An inline $E=mc^2$ here.

\[ a^2 + b^2 = c^2 \]

After the formula.
\end{document}`

// mathSrcLines walks the built document and returns every formula's source and
// the line it was stamped with.
func mathSrcLines(t *testing.T, src string) map[string]int {
	t.Helper()
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case mathNode:
				out[c.src] = c.srcLine
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return out
}

func TestAFormulaCarriesTheLineItOpenedOn(t *testing.T) {
	got := mathSrcLines(t, mathSrcDoc)
	if len(got) == 0 {
		t.Fatal("no formula was built; this test cannot say anything")
	}
	for src, line := range got {
		if line <= 0 {
			t.Errorf("formula %q carries no source line", src)
		}
	}
	// The inline formula is on line 5 of the document above and the display on
	// line 7; stated exactly, because "nonzero" would pass on a stamp that was
	// merely always 1.
	for want, line := range map[int]string{5: "inline", 7: "display"} {
		found := false
		for _, got := range got {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Errorf("no formula stamped with line %d (the %s one); got %v", want, line, got)
		}
	}
}

// The spans are the programmatic half: LineAt over a formula must answer.
func TestLineAtResolvesAFormula(t *testing.T) {
	e, err := compile([]byte(mathSrcDoc), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	spans := e.SourceSpans(72)
	if len(spans) == 0 {
		t.Fatal("no pages")
	}
	var mathLines []int
	for _, s := range spans[0] {
		// The display formula's own line: a span carrying it that no glyph could
		// have produced, since no prose sits on line 7.
		if s.Line == 7 {
			mathLines = append(mathLines, s.Line)
		}
	}
	if len(mathLines) == 0 {
		t.Fatal("the display formula contributes no SourceSpan, so LineAt cannot resolve a click on it")
	}
}

// ⛔ And the rendered page: the formula sits in a data-l group like the prose
// around it. This is what the playground reads — it measures each group's box
// in the DOM — so a formula outside every group is a formula a tap cannot
// resolve.
// ⛔ TWO DOCUMENT SHAPES, because there are TWO maths painters. A formula
// reaches the page through the horizontal path or the vertical one ("display
// math on its own line") depending on how it was placed, and a fix applied to
// whichever path one fixture exercises leaves the other emitting a bare group.
// The first version of this test used only the document above, and it passed
// with the vertical painter still unfixed.
func TestTheRenderedFormulaSitsInALineGroup(t *testing.T) {
	for name, doc := range map[string]string{
		"inline and display": mathSrcDoc,
		"align environment": `\documentclass{article}
\begin{document}
Before.

\begin{align}
a &= b \\
c &= d
\end{align}

After.
\end{document}`,
	} {
		t.Run(name, func(t *testing.T) { assertFormulasAreParented(t, doc) })
	}
}

func assertFormulasAreParented(t *testing.T, doc string) {
	t.Helper()
	pages, err := CompileToSVGPages([]byte(doc), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	svg := pages[0]

	// Count the data-l groups open at each embedded formula, the same way the
	// measurement on go-tex.github.io#235 did.
	tok := regexp.MustCompile(`<g data-l="(\d+)"|</g>|<svg `)
	open, formulas, unparented := 0, 0, 0
	for _, m := range tok.FindAllStringSubmatchIndex(svg, -1) {
		switch frag := svg[m[0]:m[1]]; {
		case strings.HasPrefix(frag, `<g data-l=`):
			open++
		case frag == `</g>`:
			if open > 0 {
				open--
			}
		case frag == `<svg `:
			if m[0] == 0 {
				continue // the page's own root element
			}
			formulas++
			if open == 0 {
				unparented++
			}
		}
	}
	if formulas == 0 {
		t.Fatal("no embedded formula on the page; this test cannot say anything")
	}
	if unparented > 0 {
		t.Errorf("%d of %d formulas sit outside every data-l group, so a tap on them resolves to no line",
			unparented, formulas)
	}
}
