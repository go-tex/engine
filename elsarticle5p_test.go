// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// elsarticle's 5p option is the two-column journal layout. The class declares it as
//
//	\DeclareOption{5p}{\xdef\jtype{5}\global\preprintfalse\ExecuteOptions{twocolumn}}
//
// (elsarticle.cls:62-63) and, for jtype 5, \maketitle is
//
//	\twocolumn[\finalMaketitle]\printFirstPageNotes          (:353-364)
//
// so the title block — \MaketitleBox, :299-312: title, authors, addresses, rule, abstract,
// keywords, rule — spans BOTH columns at the top of page 1 and the body flows in two
// columns of (522pt - 18pt)/2 = 252pt below it (geometry block :634-646). 3p and 1p are
// one column (:340-351, :366-389). The engine used to set 5p in one column with the title
// block above, which is the shape of 1p/3p.
func TestElsarticle5pTitleSpansBothColumns(t *testing.T) {
	var b strings.Builder
	b.WriteString(`\begin{frontmatter}\title{SPANTITLE}\author{A. Author}\address{Dept}` +
		`\begin{abstract}ABSTRACTTEXT\end{abstract}\begin{keyword}kw\end{keyword}\end{frontmatter}`)
	for i := 0; i < 40; i++ {
		b.WriteString("Body paragraph words words words words words words words words words words words words.\n\n")
	}
	src := `\documentclass[5p]{elsarticle}\begin{document}` + b.String() + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	var span *boxNode
	for _, r := range e.colRegions {
		if r.cols == 2 && r.span != nil {
			span = r.span
		}
	}
	if span == nil {
		t.Fatalf("5p: no \\twocolumn[title block] span recorded: %+v", e.colRegions)
	}
	if w := float64(span.width) / 65536; w < 500 || w > 523 {
		t.Errorf("5p title block is %.1fpt wide, want the 522pt text block", w)
	}
	pages := e.Pages()
	if len(pages) == 0 || !hasTwoColumns(pages[0]) {
		t.Fatalf("5p page 1 is not a two-column page below the title block")
	}
	if cw := float64(e.hsize) / 65536; cw < 251 || cw > 253 {
		t.Errorf("5p column measure is %.1fpt, want 252pt", cw)
	}
}

// 3p and 1p stay one column: no span, no column region.
func TestElsarticle3pStaysOneColumn(t *testing.T) {
	src := `\documentclass[3p]{elsarticle}\begin{document}\begin{frontmatter}\title{T}` +
		`\author{A}\address{D}\begin{abstract}AB\end{abstract}\end{frontmatter}Body.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatal(err)
	}
	if e.twoColumn {
		t.Errorf("3p is two-column")
	}
}
