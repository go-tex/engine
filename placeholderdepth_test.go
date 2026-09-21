// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// placeholderBox returns the height and depth of the frame standing in for a figure
// the engine cannot rasterise. The walk itself is firstFrame, in boxframe_test.go.
func placeholderBox(t *testing.T, src string) (h, d int) {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\usepackage{graphicx}\begin{document}`+
		src+`\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := firstFrame(e.mvl)
	if !ok {
		t.Fatal("no placeholder frame on the page")
	}
	return f.height(), f.depth()
}

// A real \includegraphics box has depth 0: the figure sits ON the baseline, entirely
// above it. Our placeholder is a frame, and a frame adds \fboxsep+\fboxrule on every
// side — so it had 3.4pt of depth below the baseline. #350 made the TOTAL right and
// left the split wrong, which a document could see: measured against tectonic,
// \includegraphics[height=50pt] gives h=50 d=0 there and gave h=46.6 d=3.4 here.
func TestPlaceholderSitsOnTheBaseline(t *testing.T) {
	h, d := placeholderBox(t, `\includegraphics[height=50pt]{absent.pdf}`)
	if d != 0 {
		t.Errorf("placeholder depth = %d sp (%.2fpt), want 0 — the box must sit on the baseline",
			d, float64(d)/float64(unity))
	}
	if want := 50 * unity; h != want {
		t.Errorf("placeholder height = %.2fpt, want the 50pt the document asked for",
			float64(h)/float64(unity))
	}
}

// The TOTAL that #350 established is unchanged: only the split between height and
// depth moves.
func TestPlaceholderTotalIsUnchanged(t *testing.T) {
	h, d := placeholderBox(t, `\includegraphics[height=50pt]{absent.pdf}`)
	if got, want := h+d, 50*unity; got != want {
		t.Errorf("placeholder height+depth = %.2fpt, want %.2fpt",
			float64(got)/float64(unity), float64(want)/float64(unity))
	}
}
