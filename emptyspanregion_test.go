// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \twocolumn[span] appended a column region at the current end of the main vertical list even
// when the region it thereby closed held NOTHING that puts ink on a page. Regions are
// page-aligned, so that empty region is a page — and with a bundled acmart.cls it was a BLANK
// PAGE 1, with the whole title block pushed onto page 2.
//
// ⛔ Measured on the nine corpus papers that bundle the real acmart.cls: every one lost page 1.
// acmart's \maketitle is \twocolumn[\box\mktitle@bx] (acmart.cls:2319-2343, once per format),
// and at that moment the list held exactly three nodes — two pageStyleNode and a zero-width
// glueNode. Reproduced in seven lines: [sigplan] gave a blank page 1 with the title on page 2,
// [acmsmall] (one column, no \twocolumn) was correct; with this change the witness is one page
// with the title on it.
//
// ⛔ The assertion is on the REGION LIST, not on the page. A first version of this test asserted
// that the title is on page 1 and it PASSED against the unfixed engine: this witness cannot load
// acmart.cls, and without acmart's geometry the pager happens to fit the empty region and the
// span on one page anyway. The two regions are the defect; the blank page is what they cost a
// real document.
func TestContentlessRegionDoesNotBecomeItsOwnRegion(t *testing.T) {
	const src = `\documentclass[twocolumn]{article}\begin{document}` +
		`\pagestyle{plain}\vskip0pt` +
		`\twocolumn[\centerline{THETITLE}]` +
		`\section{Intro}THEBODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := len(e.colRegions); n != 1 {
		t.Errorf("%d column regions where 1 is right: the contentless material before the "+
			"span was given a region, and a region is a PAGE", n)
	}
	if len(e.colRegions) > 0 && e.colRegions[0].span == nil {
		t.Error("the region carries no span — the full-width title block was not placed on it")
	}
	all := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"THETITLE", "THEBODY"} {
		if !strings.Contains(all, want) {
			t.Errorf("%q was lost; the document reads %q", want, firstN(all, 100))
		}
	}
}

// The guard in the other direction: material that DOES put ink on the page keeps a region of
// its own, which is what \twocolumn[…] after real frontmatter must do. Without this the fix
// would swallow a legitimate one-column region and set that material at the wrong measure.
func TestRealMaterialBeforeASpanKeepsItsRegion(t *testing.T) {
	const src = `\documentclass[twocolumn]{article}\begin{document}` +
		`BEFORESPAN\par` +
		`\twocolumn[\centerline{THETITLE}]` +
		`\section{Intro}THEBODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := len(e.colRegions); n != 2 {
		t.Errorf("%d column regions where 2 are right: BEFORESPAN puts ink on the page, so it "+
			"must keep its own region", n)
	}
	all := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"BEFORESPAN", "THETITLE", "THEBODY"} {
		if !strings.Contains(all, want) {
			t.Errorf("%q was lost; the document reads %q", want, firstN(all, 120))
		}
	}
}
