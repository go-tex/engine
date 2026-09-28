// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \multirow is implemented — buildMultirow (tabular.go) parses \multirow{n}{width}{text} and
// spans the cell over n rows — but only for a cell that BEGINS with it, because isMultirow
// inspects the cell's raw tokens. Anywhere else the control sequence was undefined, and a
// skipped command leaves its ARGUMENTS behind.
//
// ⛔ On 2304.06819 the cells read \parbox[t]{0mm}{\multirow{3}{*}{\rotatebox…{WSI}}} and we
// printed "3*WSI": the row count and the width marker as literal text beside the content.
// 8 corpus papers, 20 occurrences, 10 of them in that one paper.
//
// Checked against tectonic on this witness: both engines render DEDANS and DEBUT with no
// stray "2" or "*". tectonic centres DEDANS across its two rows and this fallback does not —
// the span needs the tabular builder, which already has it for the position where it can be
// done.
func TestMultirowAwayFromTheCellStart(t *testing.T) {
	const src = `\documentclass{article}\usepackage{multirow}\begin{document}` +
		"\\begin{tabular}{ll}\n" +
		"A & B\\\\\n" +
		"\\parbox[t]{0mm}{\\multirow{2}{*}{DEDANS}} & C\\\\\n" +
		" & D\\\\\n" +
		"\\multirow{2}{*}{DEBUT} & E\\\\\n" +
		" & F\\\\\n" +
		"\\end{tabular}\n" + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := e.Diagnostics().Skipped["multirow"]; n != 0 {
		t.Errorf("\\multirow is still undefined away from the cell start (%d skipped)", n)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// The content of both positions survives.
	for _, want := range []string{"DEDANS", "DEBUT", "A", "F"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 120))
		}
	}
	// ⛔ And the ARGUMENTS do not. "2*" beside the content is what a skipped \multirow leaves,
	// and it is the half of this defect that puts ink on the page.
	if strings.Contains(text, "2*") || strings.Contains(text, "2 *") {
		t.Errorf("the row count and width marker leaked as text; it reads %q", firstN(text, 120))
	}
}

// ⛔ The fallback must NOT shadow the real implementation. isMultirow tests the raw token
// before any expansion, so a cell that begins with \multirow still takes the spanning path;
// if a later change made \multirow expand first, the span would silently become a plain cell
// and nothing else would notice.
func TestMultirowAtCellStartStillSpans(t *testing.T) {
	const src = `\documentclass{article}\usepackage{multirow}\begin{document}` +
		"\\begin{tabular}{ll}\n\\multirow{2}{*}{SPANNED} & E\\\\\n & F\\\\\n\\end{tabular}\n" +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	// A spanned cell is shifted to sit between its two rows: its y is BELOW the first row's
	// and above the second's. A plain cell would share the first row's y exactly.
	ySpan, yE, yF := tspanY(t, svg, "SPANNED"), tspanY(t, svg, "E"), tspanY(t, svg, "F")
	if ySpan == yE {
		t.Errorf("SPANNED sits on the first row's baseline (y=%s, E at y=%s, F at y=%s): the "+
			"cell-start path no longer spans — the fallback shadowed it", ySpan, yE, yF)
	}
}
