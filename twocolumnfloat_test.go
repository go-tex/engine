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
