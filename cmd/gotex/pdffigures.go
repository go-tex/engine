// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"image"
	"os"

	engine "github.com/go-tex/engine"
	"github.com/go-tex/pdfrender"
)

// A vector .pdf is the commonest figure on arXiv. The engine core keeps no PDF
// renderer (image.go's RasterizePDF seam is nil there, so the browser/wasm build
// stays small and a PDF figure frames a placeholder); this is the CLI wiring the
// seam to go-tex/pdfrender so \includegraphics of a vector .pdf typesets as a real
// raster with its true height — which the seam's own comment has always described.
//
// It is GATED behind GOTEX_PDFRENDER and OFF by default: with the variable clear the
// seam stays nil, exactly as on the engine core, so a PDF figure frames the same
// placeholder and the default CLI output is byte-for-byte unchanged from current
// main. Wiring the renderer gives figures their real HEIGHT, which is a deliberate
// pagination change and belongs behind an opt-in — the memory-noted result is that
// real height alone (floats off) over-paginates; its intended companion is
// GOTEX_FLOATS, so a PDF figure floats to a page top at its true size, which is what
// TeXLive does. So the faithful mode is GOTEX_PDFRENDER=1 GOTEX_FLOATS=1 together.
//
// The renderer is third-party code reading arbitrary bytes: a panic in it must cost
// the figure, not the document, so it is fenced and reported as an ordinary error —
// which the engine already answers with the placeholder (keeping the figure's true
// aspect, since loadImage still recovers the page box).
func init() {
	if os.Getenv("GOTEX_PDFRENDER") == "" {
		return
	}
	engine.RasterizePDF = func(data []byte, dpi float64) (img image.Image, err error) {
		defer func() {
			if r := recover(); r != nil {
				img, err = nil, fmt.Errorf("pdfrender: %v", r)
			}
		}()
		return pdfrender.Rasterize(data, dpi)
	}
}
