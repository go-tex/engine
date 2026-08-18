// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

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
