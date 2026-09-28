// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ⛔ A .pdf carrying an embedded EPS's %%BoundingBox was measured as that EPS.
//
// figureDeclaredSize dispatched on a SUBSTRING — bytes.Contains(head, "%%BoundingBox")
// over the first 64KB — before testing the %PDF- PREFIX, and a substring anywhere in
// 64KB beats a prefix that identifies the whole file. A .pdf produced from a vector
// drawing routinely carries an embedded EPS and with it that EPS's bounding box.
//
// Corpus witness, 2406.10437's kendall-structure.pdf: a %PDF- whose page box is
// 354.4x154.0pt, holding "%%BoundingBox: 51 -155 439 797" at offset 34398. Read as EPS
// it measured 386x951 — aspect 2.46 instead of 0.43 — so an \includegraphics
// [width=\textwidth] placeholder stood 886.9pt tall where the figure is 156.4pt.
// pdfIntrinsicPoints reads the page box correctly and was simply never reached.
//
// Measured: 9 of 1209 PDF figures over 3 of the 154 corpus papers are %PDF- files
// carrying a %%BoundingBox.

// minimalPDFWithEmbeddedEPSBox writes a one-page PDF whose page box is w x h and which
// also contains an EPS bounding box of a DIFFERENT shape, the way a converted drawing
// does. The %%BoundingBox sits inside a stream, past the header, exactly as in the
// corpus file.
func minimalPDFWithEmbeddedEPSBox(t *testing.T, w, h float64) string {
	t.Helper()
	body := fmt.Sprintf("%%PDF-1.4\n"+
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n"+
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n"+
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %g %g] "+
		"/Contents 4 0 R /Resources << >> >>\nendobj\n"+
		"4 0 obj\n<< /Length 44 >>\nstream\n"+
		"%%%%BoundingBox: 51 -155 439 797\n0 0 0 rg\n"+
		"endstream\nendobj\n"+
		"trailer\n<< /Size 5 /Root 1 0 R >>\n%%%%EOF\n", w, h)
	dir := t.TempDir()
	p := filepath.Join(dir, "fig.pdf")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// The page box wins. 51 -155 439 797 is 388x952 (aspect 2.45); the page box here is
// 354x154 (aspect 0.435), so the two readings cannot be confused for one another.
func TestAPDFCarryingAnEmbeddedEPSBoxIsMeasuredAsAPDF(t *testing.T) {
	p := minimalPDFWithEmbeddedEPSBox(t, 354.356, 153.952)
	w, h := figureDeclaredSize(p)
	if w != 354 || h != 154 {
		t.Errorf("figureDeclaredSize = %dx%d, want 354x154 (the page box); "+
			"388x952 means the embedded EPS box was read", w, h)
	}
}

// ⛔ And the assertion that matters to a reader: the typeset BOX. A correct
// figureDeclaredSize is only useful if the placeholder is sized from it, so this
// measures the height \includegraphics actually reserves. A 345pt-wide figure at the
// page box's aspect is about 150pt tall; read as the embedded EPS it would be about
// 846pt — taller than the text block, which is how the defect showed up at all.
func TestThePlaceholderHeightFollowsThePageBoxNotTheEmbeddedEPS(t *testing.T) {
	p := minimalPDFWithEmbeddedEPSBox(t, 354.356, 153.952)
	// ⛔ The file is named by its BARE name from its own directory, never by an
	// absolute path interpolated into TeX source. A Windows temp path is
	// "C:\\Users\\RUNNER~1\\AppData\\Local\\Temp\\..." — every backslash
	// is an escape character to TeX and the ~ is active (a non-breaking space), so
	// the name never resolves, figureDeclaredSize returns 0x0 and the placeholder
	// falls back to a SQUARE. That is what this test measured on windows-latest:
	// ZZBOX=345.0pt, exactly the requested width, while the fix itself was fine —
	// the sibling test, which passes the path to Go and not to TeX, passed there.
	t.Chdir(filepath.Dir(p))
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	src := `\documentclass[11pt]{article}\usepackage{graphicx}\begin{document}` +
		`\setbox0=\hbox{\includegraphics[width=345pt]{` + filepath.Base(p) + `}}` +
		`\typeout{ZZBOX=\the\ht0}x\end{document}`
	if _, err := e.Run(src); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	msg := e.Diagnostics().Messages
	// 345 * 153.952/354.356 = 149.9pt. The EPS reading would give 345 * 952/388 = 846pt.
	if !strings.Contains(msg, "ZZBOX=149.") && !strings.Contains(msg, "ZZBOX=150.") {
		t.Errorf("placeholder height: %q — want about 149.9pt; about 846pt means the "+
			"embedded EPS box was used", msg)
	}
}
