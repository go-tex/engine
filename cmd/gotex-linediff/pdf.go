// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"github.com/go-pdfkit/reader"
)

func openPDF(b []byte) (*reader.Document, error) { return reader.Open(b) }

// pageHeight reads the page's /MediaBox so a baseline can be reported as a
// distance from the TOP of the page, which is how a reader counts down a page
// and how both engines' own logs number their lines. A page with no readable
// box reports 0, and the baselines come out as PDF user-space y.
func pageHeight(doc *reader.Document, page int) float64 {
	pg, err := doc.Page(page)
	if err != nil {
		return 0
	}
	box, ok := reader.ToArray(resolve(doc, pg["MediaBox"]))
	if !ok || len(box) < 4 {
		return 0
	}
	y0, ok1 := reader.ToFloat(resolve(doc, box[1]))
	y1, ok2 := reader.ToFloat(resolve(doc, box[3]))
	if !ok1 || !ok2 {
		return 0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return y1 - y0
}
