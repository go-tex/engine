// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// A PDF figure that cannot be rasterised (no renderer wired, or the renderer
// fails) is stood in for by a framed placeholder. To reserve the RIGHT amount of
// vertical space — so the surrounding text paginates as it would with the real
// figure — the placeholder must have the figure's aspect ratio, not a blind
// square. This file recovers that aspect (and the natural size) from the PDF's
// own page box with a dependency-free scan, keeping the engine core free of a PDF
// parser.
//
// Getting it right takes three things that a single regexp over the file's bytes
// does not give, each measured against poppler over the 1401 figure PDFs of the
// arXiv corpus:
//
//   - the box may be an INDIRECT REFERENCE, `/CropBox 60 0 R`, with the array in
//     a separate object (11 figures);
//   - it may live only inside a COMPRESSED object stream, as PDF 1.5+ producers
//     write it, with no literal box in the file at all (137 figures);
//   - CropBox must be INTERSECTED with MediaBox, not preferred outright: pdfcrop
//     leaves files whose CropBox is larger, and the spec clamps it (10 figures).
//
// With all three, 1399 of 1400 agree with poppler — the one that does not is a
// file poppler itself reports as 0x0. Before them, 269 figures (19%) were reserved
// at a DEFAULT size while the file stated theirs, and since the recovered boxes
// have a median aspect of 0.503 the placeholder was reserving about twice the
// height needed. On 2401.13343 that put a lone caption on a page of its own.

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
	"strconv"
)

// pdfBoxArrayRE matches a CropBox or MediaBox holding a literal four-number array,
// tolerating the whitespace variants real producers emit (`/MediaBox[ 0 0 470.16
// 442.08]` and `/MediaBox [ 0 0 423.56 217.39 ]`).
var pdfBoxArrayRE = regexp.MustCompile(
	`/(CropBox|MediaBox)\s*\[\s*(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s*\]`)

// pdfBoxRefRE matches the indirect form, `/CropBox 60 0 R`, whose array is a
// separate object elsewhere in the file.
var pdfBoxRefRE = regexp.MustCompile(`/(CropBox|MediaBox)\s+([0-9]+)\s+([0-9]+)\s+R`)

// pdfRotateRE matches a page's /Rotate entry. The value is a multiple of 90, and
// a producer may write it negative (/Rotate -90 is what /Rotate 270 means).
var pdfRotateRE = regexp.MustCompile(`/Rotate\s+(-?[0-9]+)`)

// pdfStreamRE finds the start of each stream body: the keyword, then a line break
// (CR LF or LF alone). The body runs to the next "endstream".
var pdfStreamRE = regexp.MustCompile(`stream\r?\n`)

// pdfRect is a box as the file states it, in PDF points (1 point = 1 bp = 1/72
// inch), normalised so x0<=x1 and y0<=y1 — a producer may write the corners either
// way round.
type pdfRect struct{ x0, y0, x1, y1 float64 }

func (r pdfRect) w() float64 { return r.x1 - r.x0 }
func (r pdfRect) h() float64 { return r.y1 - r.y0 }
func (r pdfRect) ok() bool   { return r.w() > 0 && r.h() > 0 }

// pdfIntrinsicPoints returns a PDF figure's natural width and height in PDF points,
// read from its page box. ok is false when no box is found or every one is
// degenerate.
func pdfIntrinsicPoints(data []byte) (wPt, hPt float64, ok bool) {
	if r, found := pdfEffectiveBox(data, data); found {
		return pdfTurned(r, pdfPageRotation(data))
	}
	// PDF 1.5 and later may pack the page dictionary into a compressed object
	// stream, where no literal box appears in the file's bytes. Inflating costs
	// nothing when the first pass already succeeded. An indirect reference found
	// inside a stream still resolves against the WHOLE file, which is where the
	// referenced object lives.
	for _, u := range pdfInflatedStreams(data) {
		if r, found := pdfEffectiveBox(u, data); found {
			// The rotation lives in the same page dictionary as the box, so it is
			// read from the same blob; a page dictionary in an object stream does
			// not state its rotation anywhere else.
			return pdfTurned(r, pdfPageRotation(u))
		}
	}
	return 0, 0, false
}

// pdfPageRotation reports the page's /Rotate in degrees clockwise, normalised to
// 0, 90, 180 or 270. A page that states nothing is not turned.
func pdfPageRotation(blob []byte) int {
	m := pdfRotateRE.FindSubmatch(blob)
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0
	}
	n %= 360
	if n < 0 {
		n += 360
	}
	if n%90 != 0 {
		return 0 // not a rotation the spec allows; treat the page as upright
	}
	return n
}

// pdfTurned gives a box's size as the page is SHOWN, which is the box turned by
// the page's /Rotate. A quarter turn swaps width and height.
//
// A figure is included at the size a viewer displays it, so a placeholder that
// reserves the box as written reserves a portrait space for a landscape picture.
// Two of the 770 figures in the go-tex corpus are turned this way, and they are
// both in papers whose pagination the placeholder got wrong: 2307.08085's
// web_manage2.pdf and 2408.03452's ufront-a100.pdf each state a portrait
// MediaBox and /Rotate 90, and each is shown landscape.
func pdfTurned(r pdfRect, rotation int) (wPt, hPt float64, ok bool) {
	if rotation == 90 || rotation == 270 {
		return r.h(), r.w(), true
	}
	return r.w(), r.h(), true
}

// pdfEffectiveBox finds the box a viewer would show in one blob — the file itself,
// or one inflated stream out of it — resolving indirect references against whole.
//
// The effective box is CropBox ∩ MediaBox (PDF 32000-1 §14.11.2: the crop box is
// clamped to the media box). Preferring CropBox outright was wrong for the files
// pdfcrop leaves behind, whose CropBox is LARGER than the page.
func pdfEffectiveBox(blob, whole []byte) (pdfRect, bool) {
	var crop, media pdfRect
	var haveCrop, haveMedia bool
	put := func(kind string, r pdfRect) {
		switch {
		case kind == "CropBox" && !haveCrop:
			crop, haveCrop = r, true
		case kind == "MediaBox" && !haveMedia:
			media, haveMedia = r, true
		}
	}
	for _, m := range pdfBoxArrayRE.FindAllSubmatch(blob, -1) {
		if r, ok := pdfRectOf(m[2], m[3], m[4], m[5]); ok {
			put(string(m[1]), r)
		}
	}
	for _, m := range pdfBoxRefRE.FindAllSubmatch(blob, -1) {
		if r, ok := pdfResolveRect(whole, m[2], m[3]); ok {
			put(string(m[1]), r)
		}
	}
	switch {
	case haveCrop && haveMedia:
		clamped := pdfRect{
			x0: maxF(crop.x0, media.x0), y0: maxF(crop.y0, media.y0),
			x1: minF(crop.x1, media.x1), y1: minF(crop.y1, media.y1),
		}
		if clamped.ok() {
			return clamped, true
		}
		return media, true // a crop box that does not meet the page: the page wins
	case haveCrop:
		return crop, true
	case haveMedia:
		return media, true
	}
	return pdfRect{}, false
}

// pdfResolveRect finds `<num> <gen> obj [ a b c d ]` and reads the array.
func pdfResolveRect(whole, num, gen []byte) (pdfRect, bool) {
	re, err := regexp.Compile(`(?:^|[^0-9])` + string(num) + `\s+` + string(gen) +
		`\s+obj\s*\[\s*(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s+(-?[0-9.]+)\s*\]`)
	if err != nil {
		return pdfRect{}, false
	}
	m := re.FindSubmatch(whole)
	if m == nil {
		return pdfRect{}, false
	}
	return pdfRectOf(m[1], m[2], m[3], m[4])
}

func pdfRectOf(a, b, c, d []byte) (pdfRect, bool) {
	x0, e0 := strconv.ParseFloat(string(a), 64)
	y0, e1 := strconv.ParseFloat(string(b), 64)
	x1, e2 := strconv.ParseFloat(string(c), 64)
	y1, e3 := strconv.ParseFloat(string(d), 64)
	if e0 != nil || e1 != nil || e2 != nil || e3 != nil {
		return pdfRect{}, false
	}
	r := pdfRect{minF(x0, x1), minF(y0, y1), maxF(x0, x1), maxF(y0, y1)}
	return r, r.ok()
}

// pdfInflatedStreams returns the zlib-inflated body of every stream that inflates,
// skipping the ones that do not (images, fonts, anything not FlateDecode). It reads
// no filter from the dictionary: attempting the decompression IS the test, and it
// costs one failed header check on a stream that is not deflate.
//
// The caps keep a hostile or malformed file from expanding without bound; a page
// dictionary is a few hundred bytes, so neither is ever near.
func pdfInflatedStreams(data []byte) [][]byte {
	const (
		maxStreams  = 512
		maxInflated = 1 << 20
	)
	var out [][]byte
	for _, loc := range pdfStreamRE.FindAllIndex(data, maxStreams) {
		end := bytes.Index(data[loc[1]:], []byte("endstream"))
		if end < 0 {
			continue
		}
		body := bytes.TrimRight(data[loc[1]:loc[1]+end], "\r\n")
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			continue
		}
		u, err := io.ReadAll(io.LimitReader(zr, maxInflated))
		zr.Close()
		// A truncated read still carries the dictionary, which sits at the head of
		// an object stream — so err is not a reason to drop what was read.
		if len(u) > 0 {
			out = append(out, u)
		}
	}
	return out
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func abs64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
