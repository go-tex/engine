// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// A PDF figure that cannot be rasterised (no renderer wired, or the renderer
// fails) is stood in for by a framed placeholder. To reserve the RIGHT amount of
// vertical space — so the surrounding text paginates as it would with the real
// figure — the placeholder must have the figure's aspect ratio, not a blind
// square. This file recovers that aspect (and the natural size) from the PDF's
// own page box with a dependency-free scan, keeping the engine core free of a PDF
// parser: it reads the CropBox (what a viewer displays), else the MediaBox, as a
// literal `[a b c d]` array. When the box is compressed inside an object stream
// (uncommon for the pdfcrop/matplotlib figures arXiv ships) the scan reports no
// size and the caller keeps its default-sized placeholder — no regression.

import (
	"regexp"
	"strconv"
)

// pdfBoxRE matches a CropBox or MediaBox declaration with a literal four-number
// array, tolerating the whitespace variants real producers emit
// (`/MediaBox[ 0 0 470.16 442.08]` and `/MediaBox [ 0 0 423.56 217.39 ]`).
var pdfBoxRE = regexp.MustCompile(
	`/(CropBox|MediaBox)\s*\[\s*(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s*\]`)

// pdfIntrinsicPoints returns a PDF figure's natural width and height in PDF points
// (1 point = 1 bp = 1/72 inch), read from its page box. The CropBox is preferred
// over the MediaBox (it is the extent a viewer shows, and what pdfcrop tightens);
// with both present the CropBox wins even when the MediaBox appears first. ok is
// false when no literal box is found or the box is degenerate.
func pdfIntrinsicPoints(data []byte) (wPt, hPt float64, ok bool) {
	matches := pdfBoxRE.FindAllSubmatch(data, -1)
	if matches == nil {
		return 0, 0, false
	}
	var mediaW, mediaH float64
	var haveMedia bool
	for _, m := range matches {
		x0, e0 := strconv.ParseFloat(string(m[2]), 64)
		y0, e1 := strconv.ParseFloat(string(m[3]), 64)
		x1, e2 := strconv.ParseFloat(string(m[4]), 64)
		y1, e3 := strconv.ParseFloat(string(m[5]), 64)
		if e0 != nil || e1 != nil || e2 != nil || e3 != nil {
			continue
		}
		w, h := abs64(x1-x0), abs64(y1-y0)
		if w <= 0 || h <= 0 {
			continue
		}
		if string(m[1]) == "CropBox" {
			return w, h, true // a viewer shows the CropBox — prefer it outright
		}
		if !haveMedia {
			mediaW, mediaH, haveMedia = w, h, true
		}
	}
	if haveMedia {
		return mediaW, mediaH, true
	}
	return 0, 0, false
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
