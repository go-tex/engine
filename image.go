// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's \includegraphics (the graphicx package): it loads a
// PNG or JPEG — from disk, or straight from a data: URI so the browser build needs
// no filesystem — sizes it from the [width=…, height=…, scale=…] options (or its
// intrinsic pixels), and places it as an image box. The SVG driver embeds it as a
// data-URI <image>; the PDF driver draws it as an image XObject via go-pdfkit.

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg" // register JPEG for image.DecodeConfig
	_ "image/png"  // register PNG for image.DecodeConfig
	"os"
	"strconv"
	"strings"
)

// imageNode is a placed raster image: the original file bytes (PNG/JPEG, embedded
// verbatim by both drivers) and the box it occupies (sp), sitting on the baseline.
type imageNode struct {
	data          []byte
	format        string // "png" or "jpeg"
	width, height int
	depth         int
	srcLine       int
}

func (imageNode) isNode() {}

// mime returns the data-URI media type for the image format.
func (n imageNode) mime() string {
	if n.format == "jpeg" {
		return "image/jpeg"
	}
	return "image/png"
}

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
		e.fail("includegraphics " + name + ": " + err.Error())
		return
	}
	w, h := graphicsSize(iw, ih, wReq, hReq, scale)
	e.startImage()
	e.parList = append(e.parList, imageNode{
		data: data, format: format, width: w, height: h, srcLine: e.curSrcLine,
	})
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
// format ("png"/"jpeg") and intrinsic pixel dimensions.
func loadImage(name string) (data []byte, format string, iw, ih int, err error) {
	if strings.HasPrefix(name, "data:") {
		data, err = decodeDataURI(name)
	} else {
		data, err = os.ReadFile(name)
	}
	if err != nil {
		return nil, "", 0, 0, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", 0, 0, err
	}
	return data, format, cfg.Width, cfg.Height, nil
}

// decodeDataURI extracts the bytes from a base64 data: URI.
func decodeDataURI(uri string) ([]byte, error) {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return nil, errDataURI
	}
	return base64.StdEncoding.DecodeString(uri[comma+1:])
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
