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
	"image/png" // re-encode exotic formats for embedding
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-gfx/gfx/codec"
)

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
	data, format, iw, ih, err := loadImage(name)
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
	w, h := graphicsSize(iw, ih, wReq, hReq, scale)
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
	w, h := graphicsSize(0, 0, wReq, hReq, scale)
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

// graphicsSize resolves the image box from its intrinsic pixels (taken as points)
// and the width/height/scale options, preserving aspect ratio when only one of
// width/height is given.
func graphicsSize(iw, ih, wReq, hReq int, scale float64) (w, h int) {
	natW, natH := iw*unity, ih*unity
	switch {
	case scale > 0:
		return int(float64(natW)*scale + 0.5), int(float64(natH)*scale + 0.5)
	case wReq > 0 && hReq > 0:
		return wReq, hReq
	case wReq > 0:
		if iw == 0 {
			return wReq, wReq
		}
		return wReq, wReq * ih / iw
	case hReq > 0:
		if ih == 0 {
			return hReq, hReq
		}
		return hReq * iw / ih, hReq
	default:
		return natW, natH
	}
}

// loadImage reads an image from a data: URI or a file path and returns its bytes,
// format ("png"/"jpeg"/"svg") and intrinsic pixel dimensions. An SVG source (a
// ".svg" file, a data:image/svg+xml URI, or content sniffed as SVG) is handled by
// loadSVGImage; everything else goes through image.DecodeConfig (PNG/JPEG).
func loadImage(name string) (data []byte, format imgFormat, iw, ih int, err error) {
	svgHint := false
	if strings.HasPrefix(name, "data:") {
		svgHint = svgDataURI(name)
		data, err = decodeDataURI(name)
	} else {
		svgHint = strings.EqualFold(filepath.Ext(name), ".svg")
		data, err = os.ReadFile(name)
	}
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if svgHint || looksLikeSVG(data) {
		return loadSVGImage(data)
	}
	// Raster image: decode through the go-gfx codec, which handles PNG, JPEG, GIF,
	// WEBP, TIFF, BMP, ICNS and ICO — well beyond the standard library — so more real
	// figure formats render. PNG and JPEG are embedded verbatim (both drivers handle
	// them natively, keeping the output small); any other format is re-encoded to PNG.
	img, err := codec.Decode(data)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	switch codec.Sniff(data) {
	case codec.PNG:
		return data, imgPNG, img.W, img.H, nil
	case codec.JPEG:
		return data, imgJPEG, img.W, img.H, nil
	default:
		var buf bytes.Buffer
		if err := png.Encode(&buf, img.ToNRGBA()); err != nil {
			return nil, 0, 0, 0, err
		}
		return buf.Bytes(), imgPNG, img.W, img.H, nil
	}
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
