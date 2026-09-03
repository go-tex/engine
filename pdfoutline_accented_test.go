// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// An accented section title reaches the PDF bookmark sidebar correctly: go-pdfkit
// v0.10.0 writes a non-ASCII outline /Title as a UTF-16BE string, so \section{Résumé}
// shows "Résumé" in a viewer rather than mojibake. The bookmark carries the UTF-16BE
// form, and the raw UTF-8 bytes do not appear as an outline title.
func TestAccentedSectionOutlineIsUTF16(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`\section{Résumé}Contenu.\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/Outlines") || !strings.Contains(out, "<FEFF") {
		t.Errorf("the accented bookmark title was not written UTF-16BE")
	}
	if strings.Contains(out, "Résumé") {
		t.Errorf("the accented title leaked into the PDF as raw UTF-8")
	}
}
