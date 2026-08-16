// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// A PDF figure is rasterised through the injected RasterizePDF seam and embedded as
// PNG; with no rasteriser wired it is a load error the caller turns into a
// placeholder (the browser build's behaviour). The engine core carries no PDF
// dependency — only this func hook.
func TestPDFRasterizeSeam(t *testing.T) {
	pdf := filepath.Join(t.TempDir(), "fig.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n% a figure\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// No rasteriser: a PDF figure cannot be loaded (→ placeholder upstream).
	RasterizePDF = nil
	if _, _, _, _, err := loadImage(pdf); err == nil {
		t.Fatal("expected an error with no PDF rasteriser wired")
	}

	// Wired rasteriser: the figure is rasterised and embedded as PNG at the image's
	// own pixel size.
	var gotDPI float64
	RasterizePDF = func(data []byte, dpi float64) (image.Image, error) {
		gotDPI = dpi
		img := image.NewRGBA(image.Rect(0, 0, 48, 32))
		img.Set(1, 1, color.RGBA{200, 0, 0, 255})
		return img, nil
	}
	defer func() { RasterizePDF = nil }()

	data, format, w, h, err := loadImage(pdf)
	if err != nil {
		t.Fatalf("wired PDF load: %v", err)
	}
	if format != imgPNG || w != 48 || h != 32 || len(data) == 0 {
		t.Fatalf("embed = fmt %v %dx%d len %d", format, w, h, len(data))
	}
	if gotDPI != pdfFigureDPI {
		t.Errorf("dpi = %v, want %v", gotDPI, pdfFigureDPI)
	}
}
