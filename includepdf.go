// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"image"
	"image/png"
	"strconv"
	"strings"
)

// \includepdf (pdfpages) appends pages of an existing PDF to the document. It is
// how a paper ships its supplement and how a thesis binds scanned front matter,
// and undefined it drops every one of those pages in silence: 2312.05895 ends with
// twelve \includepdf calls for a 12-page SI.pdf and came out 16 pages against the
// reference's 27.
//
// What this does: one page per requested page, each holding that page RASTERISED
// through the same seam \includegraphics uses for a PDF figure, scaled to the paper
// and centred. What it deliberately does NOT do is insert blank pages of the right
// count — that would move Sigma by ten while rendering nothing, which is the
// "a truncated document scores better" trap this corpus already knows.
//
// A raster is not a PDF import: text in an imported page stops being selectable and
// does not scale beyond pdfFigureDPI. A true import (copying content streams and
// their resources) is the better answer and a much larger one; this renders the
// content, today, through machinery that already exists.
//
// With no rasteriser wired (the browser build, or the CLI without GOTEX_PDFRENDER)
// the pages are still COUNTED — an empty page each, reported as a dropped figure —
// because a missing renderer must not silently shorten the document.
func (e *Engine) doIncludepdf() {
	pages, _ := e.scanIncludepdfOpts()
	name := e.readBraceName()
	if name == "" {
		return
	}
	data, _, ok := e.findTeXFile(name, []string{".pdf"})
	if !ok {
		if e.tolerant() {
			e.recordFigureDrop(errNoPDFRasterizer)
			return
		}
		e.fail("includepdf: cannot find " + name)
		return
	}
	// How many pages the file has, for a range like pages={1-3} or pages=-. The
	// renderer knows; the byte heuristic below only works on a PDF whose page tree
	// is not in an object stream, which most modern ones are. UNKNOWN (0) must not
	// clamp: bounding a list against a count of zero silently drops every page, and
	// this whole command exists because pages were being dropped silently.
	n := 0
	if PDFPageCount != nil {
		n = PDFPageCount(data)
	}
	if n == 0 {
		n = pdfPageCount(data)
	}
	list := resolvePageList(pages, n)
	if len(list) == 0 && n == 0 {
		list = resolvePageList(pages, 1<<20) // unbounded: let each page's render decide
	}
	for _, p := range list {
		e.includeOnePDFPage(data, p)
	}
}

// includeOnePDFPage ships one page holding the rasterised page p, full width.
func (e *Engine) includeOnePDFPage(data []byte, p int) {
	e.pageBreak()
	w := e.fullWidth()
	h := e.effectiveVsize()
	if RasterizePDFPage == nil {
		// No renderer: the page exists and is empty, and the drop is reported.
		e.recordFigureDrop(errNoPDFRasterizer)
		e.pageBreak()
		return
	}
	img, err := RasterizePDFPage(data, p, pdfFigureDPI)
	if err != nil || img == nil {
		e.recordFigureDrop(err)
		e.pageBreak()
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		e.recordFigureDrop(err)
		e.pageBreak()
		return
	}
	// Fit the page box, keeping the aspect.
	b := img.Bounds()
	iw, ih := b.Dx(), b.Dy()
	if iw > 0 && ih > 0 {
		if hh := w * ih / iw; hh <= h {
			h = hh
		} else {
			w = h * iw / ih
		}
	}
	e.startImage()
	e.parList = append(e.parList, imageNode{
		data: buf.Bytes(), format: imgPNG, width: w, height: h, srcLine: e.curSrcLine,
	})
	e.pageBreak()
}

// pageBreak is \clearpage from Go: end the paragraph, then a forced break.
func (e *Engine) pageBreak() {
	e.endParagraph()
	e.contribute(penaltyNode{penalty: -10000})
}

// RasterizePDFPage rasterises one page of a PDF, the way RasterizePDF does the
// first. A consumer wires both together (cmd/gotex/pdffigures.go); the engine core
// leaves them nil so the browser build carries no renderer.
var RasterizePDFPage func(data []byte, page int, dpi float64) (image.Image, error)

// PDFPageCount reports how many pages a PDF holds. Wired by the same consumer that
// wires the rasteriser; nil leaves \includepdf to its byte heuristic.
var PDFPageCount func(data []byte) int

// scanIncludepdfOpts reads \includepdf's [key=value,…] and returns the pages= spec.
// Every other option (angle, nup, landscape, fitpaper, pagecommand) is accepted and
// ignored: pages= is what the corpus uses, and an option silently changing the page
// COUNT is the only kind that could mislead a page-count measurement.
func (e *Engine) scanIncludepdfOpts() (pages string, ok bool) {
	e.skipOptSpace()
	t, got := e.getNext()
	if !got {
		return "", false
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return "", false
	}
	// Collect the option text with its braces intact: the value that matters is a
	// LIST, so the commas inside pages={1,2} must not be read as option separators.
	// (Splitting the whole text on commas first is exactly how this lost every page
	// but the first.) splitOptsTopLevel — geometry's, for papersize={w,h} — is the
	// same rule and is reused rather than written twice.
	var sb strings.Builder
	depth := 0
	for {
		u, got := e.getNext()
		if !got {
			break
		}
		if !u.cs_ && u.ch == '{' {
			depth++
			sb.WriteRune('{')
			continue
		}
		if !u.cs_ && u.ch == '}' {
			depth--
			sb.WriteRune('}')
			continue
		}
		if !u.cs_ && u.ch == ']' && depth <= 0 {
			break
		}
		if !u.cs_ {
			sb.WriteRune(u.ch)
		}
	}
	for _, kv := range splitOptsTopLevel(sb.String()) {
		k, v, found := strings.Cut(kv, "=")
		if found && strings.TrimSpace(k) == "pages" {
			v = strings.TrimSpace(v)
			v = strings.TrimPrefix(v, "{")
			v = strings.TrimSuffix(v, "}")
			return v, true
		}
	}
	return "", false
}

// resolvePageList turns pdfpages' pages= spec into page numbers. "-" and "" mean
// every page; "2-5" a range; "1,3,7" a list; an empty entry ("1,,3") is pdfpages'
// EMPTY page and is skipped here rather than counted, since we would have nothing
// to put on it.
func resolvePageList(spec string, n int) []int {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "-" {
		if n <= 0 || n > 1<<16 {
			return nil // "every page" of an unknown count is not a number we can guess
		}
		out := make([]int, 0, n)
		for i := 1; i <= n; i++ {
			out = append(out, i)
		}
		return out
	}
	var out []int
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if lo, hi, found := strings.Cut(part, "-"); found {
			a, err1 := strconv.Atoi(strings.TrimSpace(lo))
			b, err2 := strconv.Atoi(strings.TrimSpace(hi))
			if strings.TrimSpace(lo) == "" {
				a, err1 = 1, nil
			}
			if strings.TrimSpace(hi) == "" {
				b, err2 = n, nil
			}
			if err1 != nil || err2 != nil {
				continue
			}
			for i := a; i <= b && i <= n; i++ {
				if i >= 1 {
					out = append(out, i)
				}
			}
			continue
		}
		if i, err := strconv.Atoi(part); err == nil && i >= 1 && i <= n {
			out = append(out, i)
		}
	}
	return out
}

// pdfPageCount counts /Type /Page objects, which is enough to bound a pages= range
// without a full parse. A PDF whose page tree is in an object stream reads zero and
// the caller falls back to one page.
func pdfPageCount(data []byte) int {
	n := 0
	for _, m := range [][]byte{[]byte("/Type/Page"), []byte("/Type /Page")} {
		i := 0
		for {
			j := bytes.Index(data[i:], m)
			if j < 0 {
				break
			}
			k := i + j + len(m)
			// "/Type /Pages" is the tree node, not a page.
			if k >= len(data) || data[k] != 's' {
				n++
			}
			i = k
		}
	}
	return n
}
