// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// A document's sections become the PDF's outline (the viewer's bookmark sidebar):
// each \section/\subsection is recorded as a \@tocentry and RenderPDF turns those
// into /Outlines items jumping to the section's page. The outline tree is not
// stream-compressed, so /Outlines, /Title and the titles appear in the PDF plaintext.
func TestSectionsBecomePDFOutline(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\section{Introduction}Text.` +
		`\subsection{Background}More text.` +
		`\section{Method}Even more.` +
		`\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"/Outlines", "/Title", "(Introduction)", "(Background)", "(Method)", "/Fit"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered PDF missing %q — sections did not become a PDF outline", want)
		}
	}
}
