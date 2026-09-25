// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// acmart does not print on US letter for every format. acmart.cls:664-672 requires
// geometry and then, for acmsmall, asks for a sheet of its own:
//
//	\RequirePackage{geometry}
//	…
//	\geometry{twoside=true,
//	  paperwidth=6.75in, paperheight=10in, …}
//
// This emulation loads no class file, so nothing published that, and an acmsmall
// document came out 612x792 where the class asks for 486x720 (in the PostScript
// points a PDF is measured in). acmartFormats already carried the class's text
// block and leading — and its comment already named the sheet — but classGeometry
// had nowhere to put it.
func TestAcmartFormatsCarryTheirPaper(t *testing.T) {
	for _, c := range []struct {
		format       string
		wantW, wantH float64 // inches, as acmart.cls writes them
	}{
		{"acmsmall", 6.75, 10},
		{"acmcp", 6.75, 10},
		// The rest stay on letter, which acmart.cls:667 and :678 say outright.
		{"acmlarge", 8.5, 11},
		{"sigconf", 8.5, 11},
		{"manuscript", 8.5, 11},
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass[` + c.format + `]{acmart}\begin{document}X\end{document}`); err != nil {
			t.Fatalf("%s: %v", c.format, err)
		}
		w, h, ok := e.paperSizePt()
		if !ok {
			t.Errorf("%s: no paper size", c.format)
			continue
		}
		// paperSizePt answers in TeX points (72.27 to the inch).
		if gotW, wantW := w/72.27, c.wantW; gotW < wantW-0.01 || gotW > wantW+0.01 {
			t.Errorf("%s: paper width %.3fin, want %.2fin", c.format, gotW, wantW)
		}
		if gotH, wantH := h/72.27, c.wantH; gotH < wantH-0.01 || gotH > wantH+0.01 {
			t.Errorf("%s: paper height %.3fin, want %.2fin", c.format, gotH, wantH)
		}
	}
}
