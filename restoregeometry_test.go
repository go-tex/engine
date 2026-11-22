// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// \restoregeometry (geometry.sty:1141-1144) is \clearpage and a return to the layout
// \begin{document} found. It was a no-op, so mdpi.cls (about 30 corpus files) and
// tlsflyleaf.sty (paper 2402.04711) stayed on the title-page geometry for the whole
// body.
//
// The expected numbers are what tectonic 0.15 prints for
//
//	\documentclass[12pt,a4paper]{book}
//	\usepackage[left=1in,right=1in,top=1in,bottom=1in]{geometry}
//	\begin{document}\newgeometry{centering,vmargin=2cm,hmargin=3cm} ... \restoregeometry
//
// A: textwidth=426.79135pt, textheight=731.23584pt on page 1;
// B: textwidth=452.9679pt, textheight=700.50687pt on page 2.
func TestRestoregeometryReturnsToThePreambleLayout(t *testing.T) {
	const pre = `\documentclass[12pt,a4paper]{book}` +
		`\usepackage[left=1in,right=1in,top=1in,bottom=1in]{geometry}` +
		`\begin{document}`
	a := runGeom(t, pre+`\newgeometry{centering,vmargin=2cm,hmargin=3cm}A`)
	if want := texSP(t, "426.79135pt"); a.hsize != want {
		t.Errorf("after \\newgeometry: hsize = %d, want %d (tectonic 426.79135pt)", a.hsize, want)
	}
	if want := texSP(t, "731.23584pt"); a.vsize != want {
		t.Errorf("after \\newgeometry: vsize = %d, want %d (tectonic 731.23584pt)", a.vsize, want)
	}
	b := runGeom(t, pre+`\newgeometry{centering,vmargin=2cm,hmargin=3cm}A\restoregeometry B`)
	if want := texSP(t, "452.9679pt"); b.hsize != want {
		t.Errorf("after \\restoregeometry: hsize = %d, want %d (tectonic 452.9679pt)", b.hsize, want)
	}
	if want := texSP(t, "700.50687pt"); b.vsize != want {
		t.Errorf("after \\restoregeometry: vsize = %d, want %d (tectonic 700.50687pt)", b.vsize, want)
	}
	// Both commands \clearpage first: tectonic's witness has B on page 2.
	if n := len(b.Pages()); n != 2 {
		t.Errorf("pages = %d, want 2 (tectonic: \\newgeometry and \\restoregeometry each break the page)", n)
	}
}
