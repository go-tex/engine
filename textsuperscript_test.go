// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \textsuperscript / \textsubscript are the user-level scripts (only the internal
// \@textsuperscript was defined). Undefined, they were skipped and their content —
// the "2" of mc\textsuperscript{2}, a footnote mark, an ordinal — silently dropped.
// They now render through the math layer, so the content survives as a raised or
// lowered script.
func TestTextSuperscriptSubscriptNotDropped(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt mc\textsuperscript{2} H\textsubscript{2}O`); err != nil {
		t.Fatal(err)
	}
	if e.skippedCS["textsuperscript"] != 0 || e.skippedCS["textsubscript"] != 0 {
		t.Fatalf("text scripts were skipped as undefined: %v", e.skippedCS)
	}
	if !hasMathNode(e.mvl) && !hasMathNode(e.parList) {
		t.Error("no script (math) box placed for \\textsuperscript/\\textsubscript")
	}
}

// \text used in TEXT mode (amsmath's \text is handled inside math by the math
// layer, but a \text{…} in ordinary text reached execCS undefined and dropped its
// words). It now typesets its argument in place; math \text is unaffected because
// the math layer reads the raw source, not this macro.
func TestTextInTextModeKeepsContent(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt A\text{BCD}E`); err != nil {
		t.Fatal(err)
	}
	if e.skippedCS["text"] != 0 {
		t.Fatalf("\\text was skipped as undefined in text mode: %v", e.skippedCS)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	collectChars(e.parList, &b)
	if got := b.String(); !strings.Contains(got, "BCD") {
		t.Errorf("\\text dropped its argument in text mode; got %q", got)
	}
}
