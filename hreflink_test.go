// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// \href{url}{text} now becomes a clickable /Link annotation in the PDF (via
// go-pdfkit's AddLink), just as it already carries a live <a href> in the SVG
// output. The annotation dictionary is not stream-compressed, so /Annots, /Link
// and the URI appear in the PDF plaintext.
func TestHrefBecomesClickableLinkInPDF(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=300pt \baselineskip=15pt `)
	if _, err := e.Run(`See \href{https://go-tex.example/page}{the site}.\par`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"/Annots", "/Link", "/URI", "https://go-tex.example/page"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered PDF missing %q — \\href did not become a clickable /Link annotation", want)
		}
	}
}
