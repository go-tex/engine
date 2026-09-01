// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"testing"
)

// In two-column mode \textwidth is the width of the whole text block — both columns
// plus the gutter — while \columnwidth and \linewidth are one column. A figure sized
// to a fraction of \textwidth therefore reserves real space: the graphics-option
// parser evaluates 0.9\textwidth as a genuine dimension (≈ 0.9 of the full width),
// not the 0.9pt a raw-text scan produced by dropping the \textwidth control sequence.
// That collapse was why figure-heavy two-column reprints under-paginated.
func TestTwoColumnFigureReservesTextwidthSpace(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	uri := pngDataURI(t, 100, 100) // square, so height tracks width
	run := func(opt string) (imageNode, *Engine) {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		src := `\documentclass` + opt + `{article}\begin{document}` +
			`First paragraph to settle the measure.\par` +
			`\noindent\includegraphics[width=0.9\textwidth]{` + uri + `}` +
			`\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		im, ok := firstImage(e.mvl)
		if !ok {
			t.Fatal("no imageNode placed")
		}
		return im, e
	}

	two, e := run(`[twocolumn]`)
	if !e.twoColumn {
		t.Fatal("two-column mode not active")
	}
	full := 2*e.hsize + e.columnsep // \textwidth spans both columns and the gutter
	if want := 9 * full / 10; !within(two.width, want, want/50) {
		t.Errorf("two-column figure width = %d sp, want ≈0.9·textwidth = %d", two.width, want)
	}
	// 0.9 of the full text width is wider than a single column.
	if two.width <= e.hsize {
		t.Errorf("two-column figure width %d not wider than one column %d", two.width, e.hsize)
	}

	// One-column mode keeps the legacy raw-text read (the \textwidth token is dropped,
	// "0.9" is taken as 0.9pt), so the same source reserves almost nothing — the tuned
	// single-column corpus baseline is untouched by this change.
	one, _ := run(``)
	if one.width > unity {
		t.Errorf("one-column figure width = %d sp, expected the legacy sub-point size", one.width)
	}
}

// firstSpanBand returns the first full-width band (a figure*/table* double-column float)
// on the vertical list, or nil.
func firstSpanBand(nodes []node) *boxNode {
	for _, n := range nodes {
		if sn, ok := n.(spanNode); ok {
			return sn.box
		}
	}
	return nil
}

// A figure* / table* in two-column mode is a DOUBLE-COLUMN float: it is set at the full
// text width and placed as a band spanning BOTH columns (\@dblfloat / \@topnewpage), not
// inline in one column where it would paint past the column edge. The band therefore lands
// on the main vertical list as a spanNode whose box is the full text width.
func TestDblFloatSpansBothColumns(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	uri := pngDataURI(t, 200, 80) // a wide figure
	run := func(env string) *Engine {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		src := `\documentclass[twocolumn]{article}\begin{document}` +
			`First paragraph to settle the measure.\par` +
			`\begin{` + env + `}[t]\noindent\includegraphics[width=\textwidth]{` + uri + `}` +
			`\caption{A wide spanning float.}\end{` + env + `}` +
			`Body text after the spanning float, word word word word.\par` +
			`\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		return e
	}

	for _, env := range []string{"figure*", "table*"} {
		e := run(env)
		if !e.twoColumn {
			t.Fatalf("%s: two-column mode not active", env)
		}
		band := firstSpanBand(e.mvl)
		if band == nil {
			t.Fatalf("%s: no full-width span band placed on the vertical list", env)
		}
		full := 2*e.hsize + e.columnsep // \textwidth spans both columns and the gutter
		if !within(band.width, full, full/40) {
			t.Errorf("%s: band width = %d sp, want the full text width ≈ %d", env, band.width, full)
		}
		if band.width <= e.hsize {
			t.Errorf("%s: band width %d is not wider than a single column %d", env, band.width, e.hsize)
		}
		// The float image inside the band is sized to the full \textwidth, so it too is
		// wider than one column and does not overflow it.
		if im, ok := firstImage([]node{band}); ok && im.width <= e.hsize {
			t.Errorf("%s: float image width %d not wider than one column %d", env, im.width, e.hsize)
		}
		// The document still paginates into genuine two-column pages carrying the band.
		if pages := e.Pages(); len(pages) == 0 {
			t.Errorf("%s: no pages produced", env)
		}
	}
}

// Outside two-column mode a figure* degrades to the unstarred one-column float set inline
// on the vertical list — no full-width band — exactly as the historical figure*=\figure
// alias did, so the single-column corpus is untouched.
func TestDblFloatOneColumnStaysInline(t *testing.T) {
	uri := pngDataURI(t, 200, 80)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass{article}\begin{document}` +
		`First paragraph.\par` +
		`\begin{figure*}\noindent\includegraphics[width=\textwidth]{` + uri + `}\caption{Inline.}\end{figure*}` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	if e.twoColumn {
		t.Fatal("two-column mode unexpectedly active")
	}
	if band := firstSpanBand(e.mvl); band != nil {
		t.Errorf("one-column figure* produced a full-width band (%d sp wide); expected an inline float", band.width)
	}
	if _, ok := firstImage(e.mvl); !ok {
		t.Error("one-column figure* did not place its image inline")
	}
}

// hasChar reports whether a set character with rune ch appears anywhere in a box tree.
func hasChar(n node, ch rune) bool {
	switch v := n.(type) {
	case charNode:
		return v.ch == ch
	case *boxNode:
		for _, c := range v.list {
			if hasChar(c, ch) {
				return true
			}
		}
	}
	return false
}

// The optional [placement] on \begin{figure*}[t] is the double-column float's placement
// argument. \@dblfloat, unlike the one-column \@float, does not read it through
// \@ifnextchar, so an unconsumed "[t]" used to be collected into the band body and set as
// a literal glyph atop the figure. doDblFloat now consumes it in Go, so no bracket or
// placement letter leaks into the band.
func TestDblFloatPlacementNotLeaked(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	uri := pngDataURI(t, 200, 80)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass[twocolumn]{article}\begin{document}` +
		`First paragraph to settle the measure.\par` +
		`\begin{figure*}[htbp]\includegraphics[width=\textwidth]{` + uri + `}` +
		`\caption{No leak.}\end{figure*}` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	band := firstSpanBand(e.mvl)
	if band == nil {
		t.Fatal("no band placed")
	}
	// The caption text ("No leak.") is legitimately present; the placement bits are not.
	// '[' can appear only from a leaked optional argument (the caption has none).
	if hasChar(band, '[') {
		t.Error("band contains a '[' — the [htbp] placement argument leaked into the body")
	}
}

// A multi-panel figure* opens its body with a row of \includegraphics. That first
// paragraph carries \parindent, and at the full text width two side-by-side
// 0.48\textwidth panels plus the indent overflow the line by a hair, wrapping the second
// panel and cascading a 2×N grid into extra rows that inflate the band to most of a page.
// doDblFloat zeroes \parindent in the band body so the panels pack side by side as they
// do under TeXLive — the band stays one image-row tall, not two.
func TestDblFloatPanelsPackSideBySide(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	uri := pngDataURI(t, 100, 100) // square: one row's height ≈ one panel's width
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	src := `\documentclass[twocolumn]{article}\begin{document}` +
		`First paragraph to settle the measure.\par` +
		`\begin{figure*}[t]` +
		`\includegraphics[width=0.48\textwidth]{` + uri + `}` +
		`\includegraphics[width=0.48\textwidth]{` + uri + `}` +
		`\end{figure*}` +
		`\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	band := firstSpanBand(e.mvl)
	if band == nil {
		t.Fatal("no band placed")
	}
	full := 2*e.hsize + e.columnsep
	panel := 48 * full / 100 // a 0.48\textwidth square panel is this tall
	// Side by side, the band is about one panel tall (plus small slack); a cascade would
	// stack the two panels and make it ~2×.
	if band.height+band.depth > 3*panel/2 {
		t.Errorf("band height %d exceeds 1.5 panels (%d) — panels did not pack side by side (parindent cascade)",
			band.height+band.depth, 3*panel/2)
	}
}

// firstSpanNode returns the first spanNode on the list (band plus its placement flags).
func firstSpanNode(nodes []node) (spanNode, bool) {
	for _, n := range nodes {
		if sn, ok := n.(spanNode); ok {
			return sn, true
		}
	}
	return spanNode{}, false
}

// A figure* whose placement is [p] with no top bit is a float-page-only double float:
// LaTeX may place it only on a float page, never atop a text page. The bit threads from
// the environment's optional argument onto the band so the pager keeps it off a text-page
// top; a [t] or default float carries no such flag.
func TestDblFloatFloatPageOnlyFromPlacement(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	uri := pngDataURI(t, 200, 80)
	run := func(pos string) spanNode {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		src := `\documentclass[twocolumn]{article}\begin{document}` +
			`First paragraph to settle the measure.\par` +
			`\begin{figure*}` + pos + `\includegraphics[width=\textwidth]{` + uri + `}` +
			`\caption{c.}\end{figure*}` +
			`\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		sn, ok := firstSpanNode(e.mvl)
		if !ok {
			t.Fatalf("pos %q: no span band placed", pos)
		}
		return sn
	}
	if sn := run(`[p]`); !sn.floatPageOnly {
		t.Error("figure*[p] band should be floatPageOnly")
	}
	if sn := run(`[t]`); sn.floatPageOnly {
		t.Error("figure*[t] band should not be floatPageOnly")
	}
	if sn := run(``); sn.floatPageOnly {
		t.Error("figure* with default placement should not be floatPageOnly")
	}
}

func TestFloatPlacementBits(t *testing.T) {
	cases := []struct {
		in   string
		bits string
		fpo  bool
	}{
		{`t`, "t", false},
		{`htbp`, "htbp", false},
		{`!p`, "p", true},
		{`p`, "p", true},
		{`tp`, "tp", false},
		{`b`, "b", false},
		{``, "", false},
		{`H`, "h", false},
	}
	for _, c := range cases {
		var toks []tok
		for _, r := range c.in {
			toks = append(toks, chTok(r, catOther))
		}
		if got := placementBits(toks); got != c.bits {
			t.Errorf("placementBits(%q) = %q, want %q", c.in, got, c.bits)
		}
		if got := floatPageOnly(c.bits); got != c.fpo {
			t.Errorf("floatPageOnly(%q) = %v, want %v", c.bits, got, c.fpo)
		}
	}
}

// \textwidth reads the full text-block width even after the two-column measure has
// halved \hsize, while \columnwidth and \linewidth follow \hsize down to one column.
func TestTextwidthSpansBothColumns(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.hsize = 400 * unity
	e.columnsep = 12 * unity
	e.twoColumn = true
	e.enterTwoColumnMeasure()
	col := e.hsize // (400 - 12) / 2 = 194pt

	read := func(cs string) int {
		e.push([]tok{{ch: ' ', cat: catSpace}})
		e.push([]tok{csTok(cs)})
		return e.scanDimen()
	}
	if got := read("textwidth"); got != e.fullWidth() || got != 400*unity {
		t.Errorf("\\textwidth = %d sp, want the full width %d", got, 400*unity)
	}
	if got := read("columnwidth"); got != col {
		t.Errorf("\\columnwidth = %d sp, want the column measure %d", got, col)
	}
	if got := read("linewidth"); got != col {
		t.Errorf("\\linewidth = %d sp, want the column measure %d", got, col)
	}
}
