// Copyright (c) 2026 the go-tex/engine authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package engine

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/go-pdfkit/pdfkit"
)

// pdfFigurePrecision is how many steps a PDF point is divided into when a
// figure's page box becomes an intrinsic size: the box is in bp (1/72 in), the
// size is carried as "pixels" at pdfFigureResolution dpi, so natSP converts it
// straight back to the box's own length without a rounding of its own.
const (
	pdfFigurePrecision  = 1000
	pdfFigureResolution = 72 * pdfFigurePrecision
)

// nativePDFFigure reads page page (1-based) of a PDF figure with go-pdfkit and
// returns its displayed size (the crop box clamped to the media box, turned by
// /Rotate: what pdfTeX takes by default) as intrinsic pixels at
// pdfFigureResolution dpi. ok is false when pdfkit would refuse to import the
// page, so the caller falls back to the rasteriser or a placeholder.
func nativePDFFigure(data []byte, page int) (iw, ih int, ok bool) {
	// A refusal comes back with a zero PageInfo, so its size is empty too: the
	// error is the one thing that says why, and it is checked with the size.
	info, err := pdfkit.New(pdfkit.Options{}).PDFPageInfo(data, page)
	iw = int(info.Width*pdfFigurePrecision + 0.5)
	ih = int(info.Height*pdfFigurePrecision + 0.5)
	return iw, ih, err == nil && iw > 0 && ih > 0
}

// pdfSVG is the SVG driver's rendering of a PDF figure, which that driver cannot
// embed as a page: the rasteriser's picture when one is wired, otherwise the
// frame of the box the figure reserves.
func (n imageNode) pdfSVG(x, y, w, h string) string {
	if RasterizePDF != nil {
		if img, err := RasterizePDF(n.data, pdfFigureDPI); err == nil {
			var buf bytes.Buffer
			if png.Encode(&buf, img) == nil {
				return fmt.Sprintf(`<image x="%s" y="%s" width="%s" height="%s" href="data:image/png;base64,%s"/>`,
					x, y, w, h, base64.StdEncoding.EncodeToString(buf.Bytes()))
			}
		}
	}
	return fmt.Sprintf(`<rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="#000" stroke-width=".4"/>`, x, y, w, h)
}
