// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \hyperref recovers the VISIBLE text the undefined command used to drop: the
// [label]{text} form typesets text (label discarded), the four-argument form
// typesets its final {text}, and macros inside the text compose normally.
func TestHyperrefRecoversVisibleText(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `A\hyperref[sec:x]{Section3}B ` + // label form
		`C\hyperref[fig:y]{\textbf{Figure2}}D ` + // macro in the text
		`E\hyperref{http://x}{cat}{name}{VisibleText}F` // four-arg form
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	for _, want := range []string{"Section3", "Figure2", "VisibleText"} {
		if !strings.Contains(txt, want) {
			t.Errorf("hyperref lost visible text %q: %q", want, txt)
		}
	}
	// The label / url / category / name are NOT typeset.
	for _, garbage := range []string{"sec:x", "fig:y", "http", "cat", "name"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("hyperref leaked %q into the text: %q", garbage, txt)
		}
	}
	// The label-form text lands exactly between its surrounding letters.
	if !strings.Contains(txt, "ASection3B") {
		t.Errorf("hyperref text displaced: %q", txt)
	}
}
