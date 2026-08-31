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

// Mid-document \onecolumn/\twocolumn/\onecolumn splits the document into page-aligned
// regions: the one-column material, then the two-column body on a fresh page, then a
// one-column tail on another fresh page. Each command \clearpage's (verified against the
// reference), so no page mixes two regions.
func TestTwoColumnRegionSwitch(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	var b strings.Builder
	b.WriteString(`\documentclass{article}\usepackage[textheight=8in,textwidth=6in]{geometry}\begin{document}`)
	para := func(tag string, n int) {
		for i := 0; i < n; i++ {
			b.WriteString(`\noindent ` + tag + ` word word word word word word word word word word word word.

`)
		}
	}
	para("AAA", 10)
	b.WriteString(`\twocolumn `)
	para("BBB", 30)
	b.WriteString(`\onecolumn `)
	para("CCC", 6)
	b.WriteString(`\end{document}`)

	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(b.String()); err != nil {
		t.Fatal(err)
	}
	// Regions: an implicit/explicit one-column head, a two-column region, a one-column tail.
	if len(e.colRegions) < 2 {
		t.Fatalf("expected ≥2 column regions, got %d: %+v", len(e.colRegions), e.colRegions)
	}
	sawTwo := false
	for _, r := range e.colRegions {
		if r.cols >= 2 {
			sawTwo = true
		}
	}
	if !sawTwo {
		t.Fatalf("no two-column region recorded: %+v", e.colRegions)
	}
	// At least one page must be a genuine two-column hbox (the BBB body).
	pages := e.Pages()
	twoColPages := 0
	for _, p := range pages {
		if hasTwoColumns(p) {
			twoColPages++
		}
	}
	if twoColPages == 0 {
		t.Errorf("no two-column page produced across %d pages", len(pages))
	}
}

// \twocolumn[span] typesets the optional argument full-width and places it across the
// top of the two-column region's first page (\@topnewpage): the span box is wider than a
// single column, and the first page is a genuine two-column page.
func TestTwoColumnSpan(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	var b strings.Builder
	b.WriteString(`\documentclass{article}\usepackage[textheight=8in,textwidth=6in]{geometry}\begin{document}`)
	b.WriteString(`\twocolumn[\noindent SPANWIDE full width top material across the whole page]`)
	for i := 0; i < 30; i++ {
		b.WriteString(`\noindent body word word word word word word word word.

`)
	}
	b.WriteString(`\end{document}`)

	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(b.String()); err != nil {
		t.Fatal(err)
	}
	var spanBox *boxNode
	for _, r := range e.colRegions {
		if r.span != nil {
			spanBox = r.span
		}
	}
	if spanBox == nil {
		t.Fatalf("no \\twocolumn[...] span recorded: %+v", e.colRegions)
	}
	// The span is full-width — wider than a single (halved) column measure.
	if spanBox.width <= e.hsize {
		t.Errorf("span width %d not wider than the column measure %d", spanBox.width, e.hsize)
	}
	if pages := e.Pages(); len(pages) == 0 || !hasTwoColumns(pages[0]) {
		t.Errorf("first page is not a two-column page carrying the span")
	}
}

// A bare twocolumn class option switches on the generic two-column routine only for
// classes that use LaTeX's standard \twocolumn. Classes that run their own column engine
// (revtex via ltxgrid, the acmart/IEEEtran journal layouts) are excluded, so the option
// does not mis-drive them.
func TestTwoColumnClassExclusion(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	for _, tc := range []struct {
		cls  string
		want bool
	}{
		{"article", true},
		{"report", true},
		{"revtex4-2", false},
		{"acmart", false},
		{"IEEEtran", false},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass[twocolumn]{` + tc.cls + `}\begin{document}x

y\end{document}`); err != nil {
			t.Fatalf("%s: %v", tc.cls, err)
		}
		if e.twoColumn != tc.want {
			t.Errorf("%s[twocolumn]: twoColumn=%v, want %v", tc.cls, e.twoColumn, tc.want)
		}
	}
}

// A standard class's [twocolumn] option drives the two-column page builder LIVE — no
// GOTEX_TWOCOLUMN needed — and lets the document's own \twocolumn / \onecolumn fire.
func TestTwoColumnStandardClassLive(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\documentclass[twocolumn]{article}\begin{document}x

y\end{document}`); err != nil {
		t.Fatal(err)
	}
	if !e.twoColumn || !e.twoColLive {
		t.Errorf("article[twocolumn] not live: twoColumn=%v twoColLive=%v", e.twoColumn, e.twoColLive)
	}
}

// After a mid-document \onecolumn restores e.hsize to the full width, the two-column
// region must still paginate at the HALVED column measure it was typeset with: the
// measure is captured in the region, not read back from the mutated e.hsize. This is a
// regression guard for the bug where a trailing \onecolumn made the two columns twice
// as wide as the page (their combined width overflowed the paper).
func TestTwoColumnMeasureSurvivesOneColumn(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	var b strings.Builder
	b.WriteString(`\documentclass{article}\usepackage[textheight=8in,textwidth=6in]{geometry}\begin{document}`)
	b.WriteString(`\twocolumn[\noindent SPAN top material]`)
	for i := 0; i < 30; i++ {
		b.WriteString(`\noindent body word word word word word word word word.

`)
	}
	b.WriteString(`\onecolumn `) // restores e.hsize to full width
	b.WriteString(`tail\end{document}`)

	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(b.String()); err != nil {
		t.Fatal(err)
	}
	// The two-column region's captured colW must be about half the full width, and
	// 2*colW + columnsep must not exceed the full width (no overflow).
	var twoCol *colRegion
	for i := range e.colRegions {
		if e.colRegions[i].cols >= 2 {
			twoCol = &e.colRegions[i]
		}
	}
	if twoCol == nil {
		t.Fatalf("no two-column region: %+v", e.colRegions)
	}
	full := e.fullWidth()
	if twoCol.colW <= 0 || twoCol.colW >= full {
		t.Fatalf("two-column colW=%d not a halved measure (full=%d)", twoCol.colW, full)
	}
	if fullW := 2*twoCol.colW + e.columnsep; fullW > full {
		t.Errorf("two columns overflow: 2*%d+%d = %d > full width %d", twoCol.colW, e.columnsep, fullW, full)
	}
	// And every page fits the paper width.
	pages := e.Pages()
	paperW, _, _ := e.paperSizePt()
	limit := ptToSP(paperW) // page content must fit within the paper
	for i, p := range pages {
		if p.width > limit {
			t.Errorf("page %d width %d exceeds paper %d", i, p.width, limit)
		}
	}
}

// A revtex reprint / journal document (emulated, no bundled .cls) sets the body in two
// columns with a FULL-WIDTH frontmatter: \maketitle keeps everything typeset so far
// (title/authors/abstract) one-column, then switches the body to two columns. A revtex
// PREPRINT document stays one-column throughout. Revtex two-column is gated behind
// GOTEX_TWOCOLUMN (it renders correctly but under-paginates long reprints pending
// column-leading / float work), so the test opts in.
func TestRevtexReprintTwoColumn(t *testing.T) {
	t.Setenv("GOTEX_TWOCOLUMN", "1")
	doc := func(opt string) string {
		var b strings.Builder
		b.WriteString(`\documentclass[` + opt + `]{revtex4-2}\begin{document}`)
		b.WriteString(`\title{A Title}\author{An Author}\affiliation{Somewhere}`)
		b.WriteString(`\begin{abstract}` + strings.Repeat(`abstract word word word. `, 20) + `\end{abstract}`)
		b.WriteString(`\maketitle `)
		for i := 0; i < 40; i++ {
			b.WriteString(`\noindent body sentence word word word word word word word word.

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

	rep := run(doc("reprint,aps,prl"))
	if !rep.revtexReprint {
		t.Fatal("revtex reprint mode not detected")
	}
	if !rep.twoColumn {
		t.Fatal("revtex reprint did not switch the body to two-column")
	}
	// A one-column frontmatter region precedes a two-column body region.
	sawFront, sawBody := false, false
	for _, r := range rep.colRegions {
		if r.cols == 1 {
			sawFront = true
		}
		if r.cols >= 2 {
			sawBody = true
		}
	}
	if !sawFront || !sawBody {
		t.Fatalf("expected a one-column frontmatter and a two-column body: %+v", rep.colRegions)
	}
	repPages := rep.Pages()
	twoColPages := 0
	for _, p := range repPages {
		if hasTwoColumns(p) {
			twoColPages++
		}
	}
	if twoColPages == 0 {
		t.Errorf("revtex reprint produced no two-column page across %d pages", len(repPages))
	}

	// Preprint stays one-column and denser papers than reprint (reprint two-columns).
	pre := run(doc("preprint,aps,prl"))
	if pre.revtexReprint {
		t.Error("revtex preprint should not be reprint mode")
	}
	if pre.twoColumn {
		t.Error("revtex preprint should stay one-column")
	}
}

// revtexReprintMode: reprint / twocolumn / a journal substyle imply two-column; the
// bare society option aps does not; an explicit preprint or onecolumn overrides.
func TestRevtexReprintMode(t *testing.T) {
	for _, tc := range []struct {
		opts []string
		want bool
	}{
		{[]string{"reprint", "aps", "prl"}, true},
		{[]string{"twocolumn"}, true},
		{[]string{"aps", "prd"}, true},            // journal substyle implies reprint
		{[]string{"aps"}, false},                  // society alone: default preprint
		{[]string{"preprint"}, false},             // explicit preprint
		{[]string{"reprint", "onecolumn"}, false}, // explicit onecolumn overrides
		{nil, false},
	} {
		if got := revtexReprintMode(tc.opts); got != tc.want {
			t.Errorf("revtexReprintMode(%v)=%v, want %v", tc.opts, got, tc.want)
		}
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
