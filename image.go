// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's \includegraphics (the graphicx package): it loads a
// PNG, JPEG or SVG — from disk, or straight from a data: URI so the browser build
// needs no filesystem — sizes it from the [width=…, height=…, scale=…] options (or
// its intrinsic pixels), and places it as an image box. Both drivers embed a raster
// image verbatim (SVG driver: a data-URI <image>; PDF driver: an image XObject). An
// SVG image is embedded verbatim by the SVG driver (nested data-URI <image>) and
// drawn as vector paths by the PDF driver — see svgimage.go.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"image"
	"image/png" // re-encode exotic formats for embedding
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gfx/gfx/codec"
)

// RasterizePDF, when set by a consumer, rasterises a PDF figure (the bytes of an
// included .pdf) to an image at the given DPI. It is the seam for \includegraphics
// of vector PDFs: a pure-Go PDF renderer is a heavy dependency, so the engine core
// stays free of it — the CLI and loom inject one (see go-tex/pdfrender), while the
// browser/wasm build leaves it nil and shows a placeholder. EPS is not handled here.
var RasterizePDF func(data []byte, dpi float64) (image.Image, error)

// pdfFigureDPI is the resolution at which an injected renderer rasterises a PDF
// figure; the resulting pixels are the figure's intrinsic size, then scaled to the
// \includegraphics box like any raster.
const pdfFigureDPI = 200

// errNoPDFRasterizer is returned for a PDF figure when no renderer is wired, so the
// caller frames a placeholder (the browser build's behaviour).
var errNoPDFRasterizer = errors.New("texengine: PDF figure needs a rasteriser (none wired)")

// imgFormat is the embedded-image format an imageNode carries.
type imgFormat uint8

const (
	imgPNG  imgFormat = iota // raster, embedded/re-encoded as PNG
	imgJPEG                  // raster, embedded verbatim (DCT)
	imgSVG                   // vector: data-URI in the SVG driver, paths in the PDF driver
)

// mime returns the data-URI media type for the format.
func (f imgFormat) mime() string {
	switch f {
	case imgJPEG:
		return "image/jpeg"
	case imgSVG:
		return "image/svg+xml"
	}
	return "image/png"
}

// imageNode is a placed image: the original file bytes (PNG/JPEG embedded verbatim;
// SVG embedded verbatim by the SVG driver, vector-drawn by the PDF driver) and the
// box it occupies (sp), sitting on the baseline.
type imageNode struct {
	data          []byte
	format        imgFormat
	width, height int
	depth         int
	srcLine       int
}

func (imageNode) isNode() {}

// mime returns the data-URI media type for the image format.
func (n imageNode) mime() string { return n.format.mime() }

// dataURI returns the image as a base64 data: URI for the SVG <image> href.
func (n imageNode) dataURI() string {
	return "data:" + n.mime() + ";base64," + base64.StdEncoding.EncodeToString(n.data)
}

// doIncludegraphics implements \includegraphics[opts]{name}. The image sits on the
// baseline (height above, no depth); a load failure is a SourceError at this line.
func (e *Engine) doIncludegraphics() {
	wReq, hReq, scale := e.scanGraphicsOpts()
	name := e.readBraceName()
	if name == "" {
		return
	}
	data, format, iw, ih, dpiX, dpiY, err := loadImage(name)
	if err != nil {
		if e.tolerant() {
			// Best-effort preview: a real document's figure files are not shipped
			// with its .tex (or use a format we can't decode). Reserve the box with
			// a framed placeholder sized from the requested dimensions so the
			// surrounding text still flows, instead of aborting the whole compile.
			e.placeholderImage(wReq, hReq, scale, name)
			return
		}
		e.fail("includegraphics " + name + ": " + err.Error())
		return
	}
	w, h := graphicsSize(iw, ih, wReq, hReq, scale, dpiX, dpiY)
	e.startImage()
	e.parList = append(e.parList, imageNode{
		data: data, format: format, width: w, height: h, srcLine: e.curSrcLine,
	})
}

// placeholderImage appends a framed empty box standing in for an image that could
// not be loaded (lenient mode). It sizes the box from the requested width/height
// (or scale), falling back to a default figure size when none was given, so the
// page keeps a sensibly-sized gap where the graphic would sit.
func (e *Engine) placeholderImage(wReq, hReq int, scale float64, name string) {
	w, h := graphicsSize(0, 0, wReq, hReq, scale, 0, 0)
	if w <= 0 {
		w = 120 * unity // a default figure width when the source gave none
	}
	if h <= 0 {
		h = 90 * unity
	}
	if e.skippedCS == nil {
		e.skippedCS = map[string]int{}
	}
	e.skippedCS["includegraphics"]++
	e.startImage()
	inner := &boxNode{kind: hbox, width: w, height: h}
	e.parList = append(e.parList, frameNode{inner: inner, sep: fboxSep, rule: fboxRule})
}

// startImage ensures a paragraph is open so the image joins horizontal mode.
func (e *Engine) startImage() {
	if !e.inPar {
		e.beginParagraph(true)
	}
}

// natSP converts an intrinsic pixel count to scaled points at resolution dpi.
// A raster figure's natural TeX size is its pixels divided by its resolution
// (px / dpi = inches), times 72.27pt per inch. When dpi is 0 the image declared
// no resolution, and the historical fallback — one pixel to one TeX point (~72.27
// dpi) — is kept, so only images that actually carry a pHYs/JFIF resolution move.
func natSP(px int, dpi float64) int {
	if dpi > 0 {
		return int(float64(px)*72.27/dpi*unity + 0.5)
	}
	return px * unity
}

// graphicsSize resolves the image box from its intrinsic pixels (converted to a
// TeX length through the image's own resolution dpiX/dpiY, see natSP) and the
// width/height/scale options, preserving aspect ratio when only one of
// width/height is given.
func graphicsSize(iw, ih, wReq, hReq int, scale float64, dpiX, dpiY float64) (w, h int) {
	natW, natH := natSP(iw, dpiX), natSP(ih, dpiY)
	switch {
	case scale > 0:
		return int(float64(natW)*scale + 0.5), int(float64(natH)*scale + 0.5)
	case wReq > 0 && hReq > 0:
		return wReq, hReq
	case wReq > 0:
		if iw == 0 {
			return wReq, wReq
		}
		if dpiX > 0 && dpiY > 0 {
			return wReq, int(float64(wReq)*float64(ih)*dpiX/(float64(iw)*dpiY) + 0.5)
		}
		return wReq, wReq * ih / iw
	case hReq > 0:
		if ih == 0 {
			return hReq, hReq
		}
		if dpiX > 0 && dpiY > 0 {
			return int(float64(hReq)*float64(iw)*dpiY/(float64(ih)*dpiX) + 0.5), hReq
		}
		return hReq * iw / ih, hReq
	default:
		return natW, natH
	}
}

// loadImage reads an image from a data: URI or a file path and returns its bytes,
// format ("png"/"jpeg"/"svg"), intrinsic pixel dimensions, and the resolution
// (dpiX/dpiY) it declares — 0 when it declares none, so graphicsSize keeps the
// pixels-as-points fallback. An SVG source (a ".svg" file, a data:image/svg+xml
// URI, or content sniffed as SVG) is handled by loadSVGImage; everything else
// goes through the go-gfx codec (PNG/JPEG/…).
func loadImage(name string) (data []byte, format imgFormat, iw, ih int, dpiX, dpiY float64, err error) {
	svgHint := false
	if strings.HasPrefix(name, "data:") {
		svgHint = svgDataURI(name)
		data, err = decodeDataURI(name)
	} else {
		svgHint = strings.EqualFold(filepath.Ext(name), ".svg")
		data, err = os.ReadFile(name)
	}
	if err != nil {
		return nil, 0, 0, 0, 0, 0, err
	}
	if svgHint || looksLikeSVG(data) {
		// An SVG carries its size in TeX-compatible units, not pixels; loadSVGImage
		// already returns intrinsic points, so it needs no resolution correction.
		out, f, w, h, e := loadSVGImage(data)
		return out, f, w, h, 0, 0, e
	}
	// A PDF figure (the common vector figure on arXiv) is rasterised by an injected
	// renderer, then embedded as PNG like any other raster. With no renderer wired
	// (e.g. the browser build) it becomes a framed placeholder. The raster is made
	// at pdfFigureDPI, so that is its resolution — without it the figure would be
	// pdfFigureDPI/72.27 times oversize.
	if bytes.HasPrefix(data, []byte("%PDF-")) {
		if RasterizePDF == nil {
			return nil, 0, 0, 0, 0, 0, errNoPDFRasterizer
		}
		img, err := RasterizePDF(data, pdfFigureDPI)
		if err != nil {
			return nil, 0, 0, 0, 0, 0, err
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, 0, 0, 0, 0, 0, err
		}
		b := img.Bounds()
		return buf.Bytes(), imgPNG, b.Dx(), b.Dy(), pdfFigureDPI, pdfFigureDPI, nil
	}
	// Raster image: decode through the go-gfx codec, which handles PNG, JPEG, GIF,
	// WEBP, TIFF, BMP, ICNS and ICO — well beyond the standard library — so more real
	// figure formats render. PNG and JPEG are embedded verbatim (both drivers handle
	// them natively, keeping the output small); any other format is re-encoded to PNG.
	img, err := codec.Decode(data)
	if err != nil {
		return nil, 0, 0, 0, 0, 0, err
	}
	dx, dy := rasterResolution(data)
	switch codec.Sniff(data) {
	case codec.PNG:
		return data, imgPNG, img.W, img.H, dx, dy, nil
	case codec.JPEG:
		return data, imgJPEG, img.W, img.H, dx, dy, nil
	default:
		var buf bytes.Buffer
		if err := png.Encode(&buf, img.ToNRGBA()); err != nil {
			return nil, 0, 0, 0, 0, 0, err
		}
		// The bytes were re-encoded to a bare PNG, dropping any resolution chunk, so
		// report the resolution parsed from the ORIGINAL source instead.
		return buf.Bytes(), imgPNG, img.W, img.H, dx, dy, nil
	}
}

// rasterResolution parses the pixel resolution a PNG or JPEG declares, returning
// (dpiX, dpiY) in dots per inch, or (0, 0) when the file states none (or states a
// non-positive/aspect-only resolution the drivers ignore). A figure's TeX size is
// its pixels at this resolution; without it the pixels are taken as points.
func rasterResolution(data []byte) (dpiX, dpiY float64) {
	switch {
	case bytes.HasPrefix(data, pngMagic):
		return pngResolution(data)
	case bytes.HasPrefix(data, []byte{0xFF, 0xD8}):
		return jpegResolution(data)
	}
	return 0, 0
}

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// pngResolution reads a PNG's pHYs chunk (pixels-per-unit X, Y, unit specifier).
// Only unit 1 (metre) gives a physical resolution; it is converted to dpi. Unit 0
// (aspect ratio only) and a chunk-less file return 0.
func pngResolution(data []byte) (dpiX, dpiY float64) {
	p := len(pngMagic)
	for p+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[p:]))
		typ := data[p+4 : p+8]
		body := p + 8
		if length < 0 || body+length > len(data) {
			return 0, 0
		}
		if string(typ) == "pHYs" {
			if length < 9 {
				return 0, 0
			}
			ppuX := binary.BigEndian.Uint32(data[body:])
			ppuY := binary.BigEndian.Uint32(data[body+4:])
			if data[body+8] != 1 { // unit 1 == metre; 0 == aspect only
				return 0, 0
			}
			// dpi = pixels-per-metre × 0.0254 m/inch.
			return float64(ppuX) * 0.0254, float64(ppuY) * 0.0254
		}
		if string(typ) == "IDAT" {
			return 0, 0 // pHYs, if present, precedes the pixel data
		}
		p = body + length + 4 // skip body and the trailing CRC
	}
	return 0, 0
}

// jpegResolution reads the JFIF APP0 density (units 1 = dpi, 2 = dots/cm). Units 0
// is a pixel aspect ratio with no physical size and returns 0.
func jpegResolution(data []byte) (dpiX, dpiY float64) {
	p := 2 // past the SOI marker
	for p+4 <= len(data) {
		if data[p] != 0xFF {
			return 0, 0
		}
		marker := data[p+1]
		if marker == 0xD9 || (marker >= 0xD0 && marker <= 0xD7) { // EOI / RSTn: no length
			return 0, 0
		}
		segLen := int(binary.BigEndian.Uint16(data[p+2:]))
		if segLen < 2 || p+2+segLen > len(data) {
			return 0, 0
		}
		seg := data[p+4 : p+2+segLen]
		if marker == 0xE0 && len(seg) >= 12 && bytes.HasPrefix(seg, []byte("JFIF\x00")) {
			units := seg[7]
			xd := binary.BigEndian.Uint16(seg[8:])
			yd := binary.BigEndian.Uint16(seg[10:])
			if xd == 0 || yd == 0 {
				return 0, 0
			}
			switch units {
			case 1: // dots per inch
				return float64(xd), float64(yd)
			case 2: // dots per centimetre
				return float64(xd) * 2.54, float64(yd) * 2.54
			}
			return 0, 0 // units 0: aspect ratio only
		}
		if marker == 0xDA { // start of scan: headers are done
			return 0, 0
		}
		p += 2 + segLen
	}
	return 0, 0
}

// decodeDataURI extracts the bytes from a data: URI. A ";base64" payload is base64-
// decoded; any other payload (e.g. data:image/svg+xml,<svg…>) is treated as
// percent-encoded text, so both encodings of an inline SVG are accepted.
func decodeDataURI(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, errDataURI
	}
	meta, payload := uri[:comma], uri[comma+1:]
	if strings.Contains(meta, ";base64") {
		return base64.StdEncoding.DecodeString(payload)
	}
	if s, err := url.PathUnescape(payload); err == nil {
		return []byte(s), nil
	}
	return []byte(payload), nil
}

var errDataURI = &graphicsError{"malformed data: URI"}

type graphicsError struct{ msg string }

func (e *graphicsError) Error() string { return e.msg }

// scanGraphicsOpts parses the optional [width=…,height=…,scale=…] argument.
// Unknown keys are ignored. Missing bracket → all zero.
func (e *Engine) scanGraphicsOpts() (w, h int, scale float64) {
	e.skipOptSpace()
	t, ok := e.getXToken()
	if !ok {
		return 0, 0, 0
	}
	if t.cs_ || t.ch != '[' {
		e.back(t)
		return 0, 0, 0
	}
	// Collect the bracket content as raw text up to ']'.
	var b strings.Builder
	for {
		u, ok := e.getNext()
		if !ok || (!u.cs_ && u.ch == ']') {
			break
		}
		if !u.cs_ {
			b.WriteRune(u.ch)
		}
	}
	for _, part := range strings.Split(b.String(), ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.TrimSpace(kv[0])
		val := strings.TrimSpace(kv[1])
		switch key {
		case "width":
			w = parseDimenStr(val)
		case "height":
			h = parseDimenStr(val)
		case "scale":
			scale, _ = strconv.ParseFloat(val, 64)
		}
	}
	return w, h, scale
}

// parseDimenStr converts a TeX dimension string ("5cm", "100pt", "1.5in") to scaled
// points. An unknown or missing unit is treated as points.
func parseDimenStr(s string) int {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	unit := strings.TrimSpace(s[i:])
	ptPerUnit := map[string]float64{
		"pt": 1, "bp": 7227.0 / 7200, "pc": 12,
		"in": 72.27, "cm": 72.27 / 2.54, "mm": 72.27 / 25.4,
		"em": 10, "ex": 4.3,
	}
	f, ok := ptPerUnit[unit]
	if !ok {
		f = 1 // default: points
	}
	return int(num*f*float64(unity) + 0.5)
}
