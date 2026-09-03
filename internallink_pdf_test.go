// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// \hyperlink/\hypertarget are now navigable in the PDF, not only the SVG: the target
// registers a named /Dest and the link a /GoTo /Link annotation into the /Names
// /Dests tree (via go-pdfkit's AddNamedDest/AddNamedLink). The annotation and name
// tree are not stream-compressed, so /Names, /Dests, /GoTo and the name appear in the
// PDF plaintext.
func TestInternalLinkGoToInPDF(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=300pt \baselineskip=15pt `)
	if _, err := e.Run(`Jump to \hyperlink{sec}{the section}.\par
\hypertarget{sec}{Section target}\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"/Names", "/Dests", "/GoTo", "/Link", "(sec)"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered PDF missing %q — internal navigation did not reach the PDF", want)
		}
	}
}
