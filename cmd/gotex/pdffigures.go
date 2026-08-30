// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"fmt"
	"image"

	engine "github.com/go-tex/engine"
	"github.com/go-tex/pdfrender"
)

// A vector .pdf is the commonest figure on arXiv — 673 of the 1694 graphics the
// 200-paper corpus includes, ahead of PNG (567), JPEG (235) and EPS (146). The
// engine core keeps no PDF renderer (the browser build must stay small), so it
// exposes a seam and shows a framed placeholder when nothing is wired. This is the
// CLI wiring the seam, which the seam's own comment has always described.
//
// The renderer is third-party code reading arbitrary files: a panic in it must cost
// the figure, not the document, so it is fenced and reported as an ordinary error —
// which the engine already answers with the placeholder.
func init() {
	engine.RasterizePDF = func(data []byte, dpi float64) (img image.Image, err error) {
		defer func() {
			if r := recover(); r != nil {
				img, err = nil, fmt.Errorf("pdfrender: %v", r)
			}
		}()
		return pdfrender.Rasterize(data, dpi)
	}
}
