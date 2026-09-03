// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// renderPDFDoc runs a small document to PDF and returns the raw bytes as a string.
// The information dictionary (/Title, /Author) is not stream-compressed, so a test
// can grep the raw PDF for it.
func renderPDFDoc(t *testing.T, body string) string {
	t.Helper()
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\begin{document}` + body + `\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	return buf.String()
}

// \hypersetup{pdftitle=…,pdfauthor=…} reaches the PDF information dictionary.
func TestPDFInfoTitleAndAuthor(t *testing.T) {
	out := renderPDFDoc(t, `\hypersetup{pdftitle={A Study of Widgets},pdfauthor={Ada Lovelace}}Body.`)
	if !strings.Contains(out, `/Title (A Study of Widgets)`) {
		t.Errorf("pdftitle did not reach /Info /Title")
	}
	if !strings.Contains(out, `/Author (Ada Lovelace)`) {
		t.Errorf("pdfauthor did not reach /Info /Author")
	}
}

// \hypersetup{pdfsubject=…,pdfkeywords=…} reaches the info dictionary too.
func TestPDFInfoSubjectAndKeywords(t *testing.T) {
	out := renderPDFDoc(t, `\hypersetup{pdfsubject={On widget theory},pdfkeywords={widgets, gadgets}}Body.`)
	if !strings.Contains(out, `/Subject (On widget theory)`) {
		t.Errorf("pdfsubject did not reach /Info /Subject")
	}
	if !strings.Contains(out, `/Keywords (widgets, gadgets)`) {
		t.Errorf("pdfkeywords did not reach /Info /Keywords")
	}
}

// An accented pdftitle is written UTF-16BE, not as raw UTF-8 (the payoff of the
// go-pdfkit v0.10.0 text-string work).
func TestPDFInfoAccentedTitleIsUTF16(t *testing.T) {
	out := renderPDFDoc(t, `\hypersetup{pdftitle={Résumé des travaux}}Body.`)
	if !strings.Contains(out, "<FEFF") {
		t.Errorf("accented pdftitle was not written UTF-16BE")
	}
	if strings.Contains(out, "Résumé des travaux") {
		t.Errorf("accented pdftitle leaked into the PDF as raw UTF-8")
	}
}

// Without a pdftitle the document has no /Title in its /Info (the pre-existing
// behaviour is preserved — no empty title is emitted).
func TestPDFInfoNoTitleByDefault(t *testing.T) {
	out := renderPDFDoc(t, `Body with no metadata.`)
	if strings.Contains(out, "/Title") {
		t.Errorf("a /Title was emitted though no pdftitle was set")
	}
}

// The package-option form sets the metadata just as \hypersetup does.
func TestPDFInfoFromPackageOptions(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\usepackage[pdftitle={Wide Paper}]{hyperref}` +
		`\begin{document}Body.\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if !strings.Contains(buf.String(), `/Title (Wide Paper)`) {
		t.Errorf("\\usepackage[pdftitle=…]{hyperref} did not set /Title")
	}
}

// The parsed values are stored on the engine (a unit check independent of a font).
func TestPDFInfoStoredOnEngine(t *testing.T) {
	e := New()
	e.applyHypersetup(`pdftitle={My Title}, pdfauthor={Me}`)
	if e.pdfTitle != "My Title" || e.pdfAuthor != "Me" {
		t.Errorf("stored title=%q author=%q, want (My Title, Me)", e.pdfTitle, e.pdfAuthor)
	}
}
