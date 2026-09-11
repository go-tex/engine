// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// revtex's reprint frontmatter spans both columns and the body flows BELOW IT on
// the SAME page — it is \twocolumn[frontmatter] in all but name.
//
// It used to become a one-column region of its own. Regions are page-aligned
// (\onecolumn and \twocolumn both \clearpage), so the body started on page two
// and most of page one stayed blank: on 2203.15077 our page 1 held ink from y=70
// to y=141 of 660 where the reference fills it to y=603, and the paper set in 6
// pages against its reference's 5.
//
// Sixteen of the corpus's 154 papers are revtex.
func TestRevtexFrontmatterSharesItsPage(t *testing.T) {
	body := strings.Repeat(`Du texte de remplissage pour occuper la colonne. `, 60)
	src := `\documentclass[reprint,aps]{revtex4-2}\begin{document}` +
		`\title{Un titre}\author{Alice Martin}\affiliation{Laboratoire A}` +
		`\begin{abstract}Un resume court.\end{abstract}\maketitle ` + body +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.colRegions) != 1 {
		t.Fatalf("%d column regions, want 1 — the frontmatter is a span, not a region of its own: %+v",
			len(e.colRegions), e.colRegions)
	}
	r := e.colRegions[0]
	if r.at != 0 || r.cols != 2 {
		t.Errorf("region = {at:%d cols:%d}, want {at:0 cols:2}", r.at, r.cols)
	}
	if r.span == nil {
		t.Fatal("the region carries no span: the frontmatter went nowhere")
	}
	if got, want := r.span.width, e.fullWidth(); got != want {
		t.Errorf("span width %d, want the full width %d", got, want)
	}
	// And the body really does share page one: its first page carries more than the
	// frontmatter's own height.
	pages := e.Pages()
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	if h := pages[0].height + pages[0].depth; h <= r.span.height+r.span.depth {
		t.Errorf("page 1 is %d tall and the frontmatter alone is %d — the body did not follow it",
			h, r.span.height+r.span.depth)
	}
}

// A frontmatter taller than the text block has no page to share: a span is placed
// whole and cannot be broken, so that case keeps its own region.
func TestATallRevtexFrontmatterKeepsItsOwnRegion(t *testing.T) {
	var authors strings.Builder
	for i := 0; i < 400; i++ {
		authors.WriteString(`\author{Chercheur Numero ` + string(rune('A'+i%26)) + `}\affiliation{Laboratoire de la Ville Numero ` + string(rune('A'+i%26)) + `}`)
	}
	src := `\documentclass[reprint,aps]{revtex4-2}\begin{document}` +
		`\title{Un titre}` + authors.String() + `\maketitle Du texte.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.colRegions) < 2 {
		t.Errorf("%d region(s): a frontmatter taller than the page must keep its own", len(e.colRegions))
	}
}
