// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// applyTwoColumnMeasure halves the paragraph measure to the column width once, and
// only when two-column mode is on. A second call is a no-op (the guard).
func TestTwoColumnMeasure(t *testing.T) {
	e := New()
	e.hsize = 400 * unity
	e.columnsep = 10 * unity

	// off: no change.
	e.applyTwoColumnMeasure()
	if e.hsize != 400*unity {
		t.Fatalf("measure changed with two-column off: hsize=%d", e.hsize)
	}

	// on: hsize becomes (hsize - columnsep) / 2.
	e.twoColumn = true
	e.applyTwoColumnMeasure()
	if want := (400*unity - 10*unity) / 2; e.hsize != want {
		t.Fatalf("column measure: hsize=%d, want %d", e.hsize, want)
	}
	// idempotent: a second call does not halve again.
	got := e.hsize
	e.applyTwoColumnMeasure()
	if e.hsize != got {
		t.Fatalf("measure applied twice: hsize=%d, want %d (once)", e.hsize, got)
	}
}

// A \documentclass[twocolumn] document paginates two full-height columns per page:
// each page carries a full-width hbox of two side-by-side column vboxes, no page is
// taller than \vsize, and it is denser (fewer pages) than the same body one-column.
func TestTwoColumnPagination(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1") // activate the opt-in \documentclass[twocolumn] wiring
	body := func(opt string) string {
		var b strings.Builder
		b.WriteString(`\documentclass` + opt + `{article}\begin{document}`)
		for i := 0; i < 60; i++ {
			b.WriteString(`\noindent word word word word word word word word word word word word word word word word word word word word word word word word.

`)
		}
		b.WriteString(`\end{document}`)
		return b.String()
	}
	run := func(src string) *Engine {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(src); err != nil {
			t.Fatal(err)
		}
		return e
	}

	two := run(body(`[twocolumn]`))
	if !two.twoColumn {
		t.Fatal("twocolumn class option did not enable two-column mode")
	}
	twoPages := two.Pages()
	onePages := run(body(``)).Pages()

	if len(twoPages) == 0 {
		t.Fatal("two-column produced no pages")
	}
	if len(twoPages) >= len(onePages) {
		t.Errorf("two-column not denser: %d pages vs one-column %d", len(twoPages), len(onePages))
	}
	vsize := two.effectiveVsize()
	for i, p := range twoPages {
		if p.height > vsize {
			t.Errorf("two-column page %d height %d exceeds vsize %d", i, p.height, vsize)
		}
	}
	// The first page's content must be a wide hbox holding at least two column vboxes
	// laid side by side (the two columns).
	if !hasTwoColumns(twoPages[0]) {
		t.Errorf("first two-column page is not a two-column hbox of vboxes")
	}
}

// hasTwoColumns reports whether the page box contains a horizontal box holding at
// least two vertical sub-boxes (the left and right columns).
func hasTwoColumns(page *boxNode) bool {
	var walk func(n node) bool
	walk = func(n node) bool {
		b, ok := n.(*boxNode)
		if !ok {
			return false
		}
		if b.kind == hbox {
			vboxes := 0
			for _, c := range b.list {
				if cb, ok := c.(*boxNode); ok && cb.kind == vbox {
					vboxes++
				}
			}
			if vboxes >= 2 {
				return true
			}
		}
		for _, c := range b.list {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return walk(page)
}
