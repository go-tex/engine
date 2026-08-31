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
