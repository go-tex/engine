// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// The overpic environment is gobbled to a modest framed picture placeholder — the
// same treatment as tikzpicture — and leaks NONE of its head or body as text.
// Without the handler the option list, file name and every \put coordinate/label
// typeset as garbage in the running prose.
func TestOverpicGobblesToPlaceholder(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `Before ` +
		`\begin{overpic}[width=\textwidth,grid,tics=10]{figures/panel.pdf}` +
		`\put(10,10){LABEL}\put(50,50){\large OVR}` +
		`\end{overpic}` +
		` After`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}

	// A framed placeholder stands in for the figure...
	if _, ok := firstFrame(e.mvl); !ok {
		t.Fatal("overpic did not reserve a picture placeholder")
	}
	// ...and the running text is just the surrounding prose — no option list, no file
	// name, no \put coordinates or overlay labels leaked in.
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "Before") || !strings.Contains(txt, "After") {
		t.Fatalf("surrounding prose lost: %q", txt)
	}
	for _, garbage := range []string{"LABEL", "OVR", "width", "grid", "tics", "panel", "pdf"} {
		if strings.Contains(txt, garbage) {
			t.Errorf("overpic leaked %q into the text: %q", garbage, txt)
		}
	}
}

// The starred overpic* variant is handled identically (it only adds a grid overlay,
// which the engine has no layer for anyway), and nesting is balanced.
func TestOverpicStarAndNesting(t *testing.T) {
	e := New()
	e.lenient = true
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `X \begin{overpic*}[width=3cm]{nope.pdf}\put(1,1){Z}\end{overpic*} Y`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "X") || !strings.Contains(txt, "Y") {
		t.Fatalf("prose lost around overpic*: %q", txt)
	}
	if strings.Contains(txt, "Z") || strings.Contains(txt, "width") || strings.Contains(txt, "nope") {
		t.Errorf("overpic* leaked its head/body: %q", txt)
	}
}
