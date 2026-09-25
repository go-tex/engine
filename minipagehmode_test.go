// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// latex.ltx opens both \@iiiminipage (11853) and \@iiiparbox with \leavevmode, so
// a panel met in VERTICAL mode starts a paragraph and the next one joins it on the
// same LINE. Without it each became its own paragraph and side-by-side panels
// stacked, which is how most papers set multi-panel figures: a \vbox holding two
// 40pt panels measured 61pt where tectonic gives 22.5pt (#398). 12 of the corpus
// papers set figures that way, 54 times.
//
// The same witness pins the [c] reference point, which is the math AXIS and not
// the baseline: one 40pt panel is 22.5pt tall in tectonic, not 20pt.
func TestPanelsShareALine(t *testing.T) {
	const panel = `\begin{minipage}{50pt}\rule{40pt}{40pt}\end{minipage}`
	const pbox = `\parbox{50pt}{\rule{40pt}{40pt}}`
	for _, c := range []struct{ nom, src string }{
		{"one minipage", `\setbox0\vbox{` + panel + `}`},
		{"two minipages", `\setbox0\vbox{` + panel + `\hfill` + panel + `}`},
		{"three minipages", `\setbox0\vbox{` + panel + `\hfill` + panel + `\hfill` + panel + `}`},
		{"two parboxes", `\setbox0\vbox{` + pbox + `\hfill` + pbox + `}`},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(c.src); err != nil {
			t.Fatalf("%s: %v", c.nom, err)
		}
		b := e.box[0]
		if b == nil {
			t.Fatalf("%s: no box", c.nom)
		}
		// Every case is ONE line of panels, so the vbox is one panel tall however
		// many panels it holds. spMock's baselineskip does not apply to a single
		// line, so the height is the panel's own: 40pt total, centred on the axis.
		if got, want := b.height+b.depth, 40*unity; got != want {
			t.Errorf("%s: vbox = %d sp, want %d (one line of panels)", c.nom, got, want)
		}
	}
}
