package engine

import (
	"bytes"
	"os"
	"testing"
)

func TestRenderPDFProducesValidPDF(t *testing.T) {
	fp := "/System/Library/Fonts/Supplemental/Georgia.ttf"
	if _, err := os.Stat(fp); err != nil {
		t.Skip("no system font")
	}
	e := New()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	e.Run(`\hsize=300pt \baselineskip=15pt `)
	e.Run(`The go-tex engine can now emit a real PDF via go-pdfkit, drawing the ` +
		`same box tree it renders to SVG, with the font embedded as a subset.\par`)
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	b := buf.Bytes()
	if len(b) < 500 {
		t.Fatalf("PDF too small: %d bytes", len(b))
	}
	if string(b[:5]) != "%PDF-" {
		t.Fatalf("not a PDF, header=%q", b[:8])
	}
	if !bytes.Contains(b, []byte("%%EOF")) {
		t.Errorf("PDF missing EOF marker")
	}
	// a font must be embedded (FontFile2 for TrueType subset)
	if !bytes.Contains(b, []byte("/FontFile2")) && !bytes.Contains(b, []byte("/FontFile3")) {
		t.Errorf("no embedded font subset found in PDF")
	}
}

func TestRenderPDFNeedsFont(t *testing.T) {
	e := New()
	e.SetFont(spMock{}) // not embeddable
	if err := e.RenderPDF(new(bytes.Buffer), 0); err == nil {
		t.Error("expected an error when the font is not embeddable")
	}
}
