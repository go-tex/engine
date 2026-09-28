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
// placeholder and the default CLI output is byte-for-byte unchanged.
//
// GOTEX_PDFRENDER=1 on its own is the whole of it. GOTEX_FLOATS is ON unless it is
// set to "0" (see floatplace.go), so a figure already floats to a page top; naming
// the two together, as this comment once did, says nothing the first does not.
//
// WHAT IT COSTS AND BUYS, measured over the 154-paper arXiv corpus on 2026-09-28,
// the two modes rendered back to back:
//
//   - it buys the figures. The census channel "PDF figure, no rasteriser wired" —
//     955 uses across 82 papers — goes to nothing. What is left there is 42
//     figures whose file is genuinely absent from the corpus.
//   - it costs 534 s over the corpus (141 s to 675 s). No paper fails and none
//     renders an empty document, either way; the slowest is the 300-page one at
//     114 s.
//   - it moves the page count AWAY from tectonic, slightly: the sum of absolute
//     deviations goes 328 to 340 and exact pagination 35/154 to 33/154. Ten papers
//     move, eight of them away. We already under-paginate by 74 pages, and giving
//     figures their true height shortens the document further, to 90.
//
// So it stays an opt-in — but for the third reason, not because it is slow, and not
// because the renderer cannot be trusted with a corpus. Note also that a page count
// is a proxy: the flag makes the CONTENT strictly more faithful whatever it does to
// pagination, so this is a trade, not a regression.
//
// The renderer reads arbitrary bytes: a panic in it must cost the figure, not the
// document, so it is fenced and reported as an ordinary error — which the engine
// already answers with the placeholder (keeping the figure's true aspect, since
// loadImage still recovers the page box, turned by any /Rotate it states).
func init() {
	if os.Getenv("GOTEX_PDFRENDER") == "" {
		return
	}
	// \includepdf needs a page number; the figure seam always takes page 1.
	engine.RasterizePDFPage = func(data []byte, page int, dpi float64) (img image.Image, err error) {
		defer func() {
			if r := recover(); r != nil {
				img, err = nil, fmt.Errorf("pdfrender: %v", r)
			}
		}()
		return pdfrender.RasterizePage(data, page, dpi)
	}
	engine.PDFPageCount = func(data []byte) (n int) {
		defer func() {
			if recover() != nil {
				n = 0
			}
		}()
		return pdfrender.NumPages(data)
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
