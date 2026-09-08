// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"compress/zlib"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// deflated wraps s in a PDF stream object whose body is zlib-compressed, the shape
// a PDF 1.5+ producer uses for an object stream.
func deflated(s string) []byte {
	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	w.Write([]byte(s))
	w.Close()
	var out bytes.Buffer
	out.WriteString("%PDF-1.5\n1 0 obj\n<< /Type /ObjStm /Filter /FlateDecode >>\nstream\n")
	out.Write(z.Bytes())
	out.WriteString("\nendstream\nendobj\n")
	return out.Bytes()
}

// A PDF 1.5+ producer may pack the page dictionary into a compressed object stream,
// so no literal /MediaBox appears in the file's bytes. 137 of the arXiv corpus's
// 1401 figure PDFs are like that, and a placeholder that falls back to a default
// reserves the wrong height — on 2401.13343 it put a lone caption on its own page.
func TestPDFBoxFromCompressedObjectStream(t *testing.T) {
	data := deflated("<< /Type /Page /MediaBox [ 0 0 391.91998 148.080002 ] >>")
	if bytes.Contains(data, []byte("/MediaBox")) {
		t.Fatal("the fixture is not actually compressed")
	}
	w, h, ok := pdfIntrinsicPoints(data)
	if !ok {
		t.Fatal("no box recovered from a compressed object stream")
	}
	if int(w+0.5) != 392 || int(h+0.5) != 148 {
		t.Errorf("box = %vx%v, want 392x148", w, h)
	}
}

// A stream that is not deflate must not abort the scan: the literal box, or a later
// stream, still has to be found.
func TestPDFBoxSkipsUndeflatableStreams(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("%PDF-1.5\n1 0 obj\n<< /Filter /DCTDecode >>\nstream\n")
	b.WriteString("\xff\xd8\xff\xe0 not deflate at all")
	b.WriteString("\nendstream\nendobj\n")
	b.Write(bytes.TrimPrefix(deflated("<< /MediaBox [ 0 0 200 100 ] >>"), []byte("%PDF-1.5\n")))
	w, h, ok := pdfIntrinsicPoints(b.Bytes())
	if !ok || int(w+0.5) != 200 || int(h+0.5) != 100 {
		t.Errorf("box = %vx%v ok=%v, want 200x100 true", w, h, ok)
	}
}

// figureDeclaredSize used to read only the first 64KB of a PDF. The page dictionary
// sits wherever the producer put it, and on 132 of the corpus's figure PDFs it is
// past that mark — those were reserved at a default size while the file said
// otherwise.
func TestFigureDeclaredSizeReadsPastTheHead(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "far.pdf")
	body := "%PDF-1.4\n" + strings.Repeat("% padding\n", 9000) +
		"3 0 obj\n<< /Type /Page /MediaBox [ 0 0 300 150 ] >>\nendobj\n"
	if len(body) < 64<<10 {
		t.Fatalf("fixture is only %d bytes, needs to exceed the 64KB head", len(body))
	}
	if err := os.WriteFile(name, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	w, h := figureDeclaredSize(name)
	if w != 300 || h != 150 {
		t.Errorf("figureDeclaredSize = %dx%d, want 300x150", w, h)
	}
}

// The box may be an INDIRECT REFERENCE, with the array in a separate object:
//
//	/CropBox 60 0 R … 60 0 obj [ 13.34 27.29 777 310.02 ] endobj
//
// 11 corpus figures are written that way, and reading only the literal array left
// them on the MediaBox — 3 to 10% too large, always in the same direction.
func TestPDFBoxResolvesAnIndirectReference(t *testing.T) {
	data := []byte("%PDF-1.4\n" +
		"5 0 obj\n<< /Type /Page /MediaBox [0 0 787.92 337.92] /CropBox 60 0 R >>\nendobj\n" +
		"60 0 obj\n[ 13.339996 27.290009 777 310.019989]\nendobj\n")
	w, h, ok := pdfIntrinsicPoints(data)
	if !ok {
		t.Fatal("no box")
	}
	// CropBox ∩ MediaBox = the CropBox here, 763.66 x 282.73 — what poppler reports.
	if int(w+0.5) != 764 || int(h+0.5) != 283 {
		t.Errorf("box = %.2fx%.2f, want 763.66x282.73", w, h)
	}
}

// PDF 32000-1 §14.11.2 clamps the crop box to the media box. pdfcrop leaves files
// whose CropBox is LARGER than the page, and preferring it outright made the
// figure 792x612 where poppler shows 368x269.
func TestPDFCropBoxIsClampedToTheMediaBox(t *testing.T) {
	data := []byte("%PDF-1.4\n<< /MediaBox [0 0 368 269.33] /CropBox [0 0 792.0 612.0] >>\n")
	w, h, ok := pdfIntrinsicPoints(data)
	if !ok {
		t.Fatal("no box")
	}
	if int(w+0.5) != 368 || int(h+0.5) != 269 {
		t.Errorf("box = %.2fx%.2f, want the media box 368x269.33", w, h)
	}
}

// A crop box that does not meet the page at all leaves the page standing, rather
// than an empty intersection that would reserve nothing.
func TestPDFDisjointCropBoxFallsBackToTheMediaBox(t *testing.T) {
	data := []byte("%PDF-1.4\n<< /MediaBox [0 0 100 100] /CropBox [500 500 600 600] >>\n")
	w, h, ok := pdfIntrinsicPoints(data)
	if !ok || int(w+0.5) != 100 || int(h+0.5) != 100 {
		t.Errorf("box = %.2fx%.2f ok=%v, want 100x100 true", w, h, ok)
	}
}
