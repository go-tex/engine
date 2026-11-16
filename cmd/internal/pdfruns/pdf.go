// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Package pdfruns reads the text a PDF shows — the positioned runs, their fonts
// and their widths — with nothing outside the standard library and
// github.com/go-pdfkit/reader.
//
// ⛔ WHY IT IS A PACKAGE. This code was written for gotex-linediff and lived in
// its `package main`, where PageRuns and Run were already capitalised — written
// to be shared — and yet unreachable by any other tool. The cost was visible in
// two places: gotex-worddiff reaches for internal/pdftext, which shells out to
// poppler's pdftotext through pkgx, to answer a question this package answers in
// process; and gotex-abdiff had no text access at all, so its reports carry page
// counts and nothing about ink.
//
// That mattered on 2026-11-16: an A/B said Σ|page deviation| +7 for an embedding,
// which cannot distinguish "added the ink the reference has" from "made the pages
// longer", and settling it took a second protocol built by hand — two worktrees,
// two binaries and two gotex-refdiff passes — to learn that the ink gain was +6
// words in 32 081. The xkeyval precedent in texmf/LICENSE-NOTE.md shows the
// report that was wanted all along: "Σ 328 → 335 (+7) … glyphs +19336", two
// numbers from one run.
//
// ⚠ NO EXTERNAL TOOL, ON PURPOSE. gotex-linediff's own doc states the property —
// "no qpdf, no poppler, no MuPDF" — and go-pdfkit/reader keeps it: nothing outside
// the Go standard library, so this builds for js/wasm and every architecture the
// fleet targets.
package pdfruns

import (
	"github.com/go-pdfkit/reader"
)

// Open parses PDF bytes.
func Open(b []byte) (*reader.Document, error) { return reader.Open(b) }

// PageHeight reads the page's /MediaBox so a baseline can be reported as a
// distance from the TOP of the page, which is how a reader counts down a page
// and how both engines' own logs number their lines. A page with no readable
// box reports 0, and the baselines come out as PDF user-space y.
func PageHeight(doc *reader.Document, page int) float64 {
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

// Glyphs counts the glyph codes every page of the document shows: the ink on the
// page, in the unit texmf/LICENSE-NOTE.md's xkeyval entry already reports.
//
// ⛔ IT IS NOT A WORD COUNT and must not be read as one. A glyph is one code
// passed to a text operator, so a ligature counts once where its letters would
// count two or three, and a word broken across a line counts its added hyphen.
// What it answers is narrower and is the question an A/B cannot otherwise ask:
// did this change put MORE ink on the paper, or the same ink in other places?
//
// A page that fails to decode contributes nothing and is counted in bad, because
// a damaged page is not an empty one — returning only the total would let a
// corrupt file read as a document that lost its text.
func Glyphs(doc *reader.Document) (total, bad int) {
	for n := 1; n <= doc.PageCount(); n++ {
		runs, err := PageRuns(doc, n)
		if err != nil {
			bad++
			continue
		}
		for _, r := range runs {
			total += len([]rune(r.Text))
		}
	}
	return total, bad
}
