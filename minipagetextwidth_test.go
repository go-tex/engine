// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A minipage is the ONE place where \textwidth legitimately stops being the page
// measure. latex.ltx:11855-11861, \@iiiminipage:
//
//	\setbox\@tempboxa\vbox\bgroup
//	  \color@begingroup
//	    \hsize\@tempdima
//	    \textwidth\hsize \columnwidth\hsize
//	    \@parboxrestore
//
// all three on consecutive lines. \parbox does NOT do it — \@iiiparbox sets \hsize
// and nothing else — and tectonic agrees: inside a 0.33\textwidth minipage
// \the\textwidth reads 113.85063pt, inside the same-width \parbox it still reads
// 345.0pt.
//
// #375 made \textwidth its own register, correctly, and this case went with it:
// \textwidth kept reporting the OUTER block inside a minipage, so
// \includegraphics[width=\textwidth] in a 0.32\textwidth panel scaled to the full
// measure — three times too wide, and its proportional height three times too tall
// (#406).
func TestTextwidthInsideAMinipageIsTheBoxWidth(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\textwidth=345pt` +
		`\message{[out \the\textwidth]}` +
		`\begin{minipage}{100pt}\message{[mp \the\textwidth]}\end{minipage}` +
		`\parbox{100pt}{\message{[pb \the\textwidth]}}` +
		`\message{[after \the\textwidth]}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[out 345.0pt]",
		"[mp 100.0pt]",    // the minipage sets it…
		"[pb 345.0pt]",    // …and \parbox does not
		"[after 345.0pt]", // …and it is restored on the way out
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %q", want, out)
		}
	}
}

// The width a figure is sized against must follow, since that is what the defect
// cost: a panel three times too wide.
func TestFigureWidthFollowsTheMinipage(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	out, err := e.Run(`\textwidth=345pt` +
		`\begin{minipage}{0.32\textwidth}\message{[w \the\textwidth]}\end{minipage}`)
	if err != nil {
		t.Fatal(err)
	}
	// 110.40253pt, not 110.4pt: TeX multiplies in scaled points and tectonic
	// answers the same value for the same source, so this pins TeX's arithmetic
	// rather than ours.
	if !strings.Contains(out, "[w 110.40253pt]") {
		t.Errorf("got %q, want [w 110.40253pt] (0.32 x 345pt, as tectonic sets it)", out)
	}
}
