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
	"io"
	"io/fs"
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
	// Size an un-rasterisable PDF placeholder to the figure's TRUE aspect (not a blind
	// square) whenever GOTEX_PDFASPECT is set OR the page is set in two columns. In
	// two-column mode the relative-width floats reserve real space (see
	// scanGraphicsOpts) and a square over-reserves a wide figure's height, so the true
	// aspect is what keeps a figure-heavy two-column page's count right.
	data, format, iw, ih, dpiX, dpiY, err := loadImage(name, pdfAspectOptIn() || e.twoColumn)
	if err != nil {
		if e.tolerant() {
			// Best-effort preview: a real document's figure files are not shipped
			// with its .tex (or use a format we can't decode). Reserve the box with
			// a framed placeholder sized from the requested dimensions so the
			// surrounding text still flows, instead of aborting the whole compile.
			// For a PDF figure loadImage still recovers the page box (iw/ih), so the
			// placeholder keeps the figure's real aspect instead of a blind square.
			e.recordFigureDrop(err)
			e.placeholderImage(wReq, hReq, iw, ih, scale, dpiX, dpiY, name)
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
func (e *Engine) placeholderImage(wReq, hReq, iw, ih int, scale, dpiX, dpiY float64, name string) {
	// iw/ih carry the figure's intrinsic size when it was recoverable (a PDF page
	// box); with them, a width-only \includegraphics keeps the figure's true aspect
	// instead of collapsing to a square.
	//
	// When they are absent, the file itself may still SAY how big it is — an EPS in
	// its %%BoundingBox, a PDF in its /MediaBox — and reading that costs no rendering
	// at all. It is what makes the reserved box the right SHAPE: with only [width=…]
	// given, the height then follows the figure's own aspect instead of a fixed
	// default, and the text around it breaks where it should.
	if iw <= 0 || ih <= 0 {
		if dw, dh := figureDeclaredSize(name); dw > 0 && dh > 0 {
			iw, ih, dpiX, dpiY = dw, dh, 72, 72 // a declared box is in PostScript points
		}
	}
	w, h := graphicsSize(iw, ih, wReq, hReq, scale, dpiX, dpiY)
	if w <= 0 {
		w = 120 * unity // a default figure width when the source gave none
	}
	if h <= 0 {
		h = 90 * unity
	}
	e.startImage()
	inner := &boxNode{kind: hbox, width: w, height: h}
	e.parList = append(e.parList, frameNode{inner: inner, sep: fboxSep, rule: fboxRule})
}

// recordFigureDrop tallies a figure the engine could not load, by REASON.
//
// It used to be tallied as a skipped "includegraphics", which said the opposite of
// what happened: the command is defined and did its job — it reserved the box — and
// what failed was the FILE. A report that names the command sends a reader looking
// for a missing macro, and the three real causes (no rasteriser, absent file,
// format we cannot decode) are worth telling apart, since only the first is a
// feature the engine could grow.
func (e *Engine) recordFigureDrop(err error) {
	if e.figuresDropped == nil {
		e.figuresDropped = map[string]int{}
	}
	e.figuresDropped[figureDropReason(err)]++
}

// figureDropReason buckets a load failure into a cause a reader can act on. The
// filename is deliberately left out so the counts aggregate over a corpus.
func figureDropReason(err error) string {
	switch {
	case errors.Is(err, errNoPDFRasterizer):
		return "PDF figure, no rasteriser wired"
	case errors.Is(err, fs.ErrNotExist):
		return "file not found"
	default:
		return "unreadable or unsupported format"
	}
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
// graphicsSearchExts is the order an extension-less \includegraphics reference is
// searched — pdf first, matching pdftex's \Gin@extensions — with both cases so a
// case-sensitive filesystem finds fig.PNG for {fig} too.
var graphicsSearchExts = []string{
	".pdf", ".png", ".jpg", ".jpeg", ".svg", ".gif",
	".PDF", ".PNG", ".JPG", ".JPEG", ".SVG", ".GIF",
}

// hasKnownGraphicsExt reports whether name already ends in a graphics extension,
// which graphicx honours verbatim rather than searching past.
func hasKnownGraphicsExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".pdf", ".png", ".jpg", ".jpeg", ".svg", ".gif", ".eps", ".ps", ".bmp", ".tif", ".tiff":
		return true
	}
	return false
}

// resolveGraphicsPath resolves an \includegraphics reference to a file on disk the
// way graphicx does: an exact hit wins; otherwise, when the reference carries no
// known graphics extension, each extension is appended in turn and the first that
// exists is used. This lets a figure written \includegraphics{fig} — the
// extension-less form real papers use — resolve to fig.pdf / fig.png / … instead
// of failing to a placeholder. ok is false when nothing matches, leaving the
// caller to read the name as given (and report the miss as before).
func resolveGraphicsPath(name string) (string, bool) {
	if _, err := os.Stat(name); err == nil {
		return name, true
	}
	if hasKnownGraphicsExt(name) {
		return "", false
	}
	for _, ext := range graphicsSearchExts {
		if cand := name + ext; fileExists(cand) {
			return cand, true
		}
	}
	return "", false
}

// fileExists reports whether path is an existing regular file (not a directory).
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func loadImage(name string, aspect bool) (data []byte, format imgFormat, iw, ih int, dpiX, dpiY float64, err error) {
	svgHint := false
	if strings.HasPrefix(name, "data:") {
		svgHint = svgDataURI(name)
		data, err = decodeDataURI(name)
	} else {
		path := name
		if p, ok := resolveGraphicsPath(name); ok {
			path = p
		}
		svgHint = strings.EqualFold(filepath.Ext(path), ".svg")
		data, err = os.ReadFile(path)
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
		// The figure's natural size, read from its page box, lets a placeholder keep
		// the right aspect (and reserve the right vertical space) when no renderer is
		// wired or rasterisation fails — treated as pixels at 72dpi (1 PDF point = 1
		// bp = 1/72 inch), so natSP converts them straight back to the box's points.
		boxIW, boxIH, haveBox := 0, 0, false
		if aspect {
			if w, h, ok := pdfIntrinsicPoints(data); ok {
				boxIW, boxIH, haveBox = int(w+0.5), int(h+0.5), true
			}
		}
		if RasterizePDF == nil {
			if haveBox {
				return nil, imgPNG, boxIW, boxIH, 72, 72, errNoPDFRasterizer
			}
			return nil, 0, 0, 0, 0, 0, errNoPDFRasterizer
		}
		img, err := RasterizePDF(data, pdfFigureDPI)
		if err != nil {
			if haveBox {
				return nil, imgPNG, boxIW, boxIH, 72, 72, err
			}
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
	// Collect the bracket content as a TOKEN list (control sequences KEPT) up to the
	// closing ']' at brace level 0. Keeping the tokens is what lets a value like
	// width=0.48\textwidth be evaluated as a real TeX dimension: a raw-text scan
	// dropped the \textwidth control sequence and read "0.48" as 0.48pt, collapsing
	// every \columnwidth/\textwidth/\linewidth-relative figure — the overwhelming
	// majority in real papers — to a fraction of a point, so it reserved almost no
	// vertical space.
	var toks []tok
	depth := 0
	for {
		u, ok := e.getNext()
		if !ok {
			break
		}
		if !u.cs_ {
			switch u.ch {
			case '{':
				depth++
			case '}':
				depth--
			case ']':
				if depth <= 0 {
					goto scan
				}
			}
		}
		toks = append(toks, u)
	}
scan:
	// The correct token evaluation is the default in TWO-COLUMN mode. There, figures set
	// to a fraction of \columnwidth/\textwidth are the norm and their reserved height is
	// the binding constraint on pagination (a figure-heavy reprint under-paginates
	// badly when they collapse — 2601.22272: 51pp vs 62).
	//
	// In ONE-COLUMN mode it is additionally applied under the GOTEX_FLOATS faithful
	// figure mode: a reader who opted into real float placement (and, with it, real
	// figure rasters) needs figures to reserve their true \linewidth-relative height
	// rather than collapse. With the flag OFF, one-column keeps the legacy raw-text read
	// (control sequences dropped) byte-for-byte — evaluating it correctly there shifts
	// every single-column class-emulation baseline (revtex preprint, IEEEtran, acmart,
	// amsart) at once, which were tuned against the collapse — so that broad re-tuning
	// stays behind the opt-in flag instead of moving the default.
	evalDimen := func(val []tok) int {
		if e.twoColumn || floatsEnabled() {
			return e.evalDimenTokens(val)
		}
		return parseDimenStr(tokensToText(val))
	}
	for _, seg := range splitTokensAtComma(toks) {
		key, val, ok := splitKeyValueTokens(seg)
		if !ok {
			continue
		}
		switch key {
		case "width":
			w = evalDimen(val)
		case "height":
			h = evalDimen(val)
		case "scale":
			scale, _ = strconv.ParseFloat(strings.TrimSpace(tokensToText(val)), 64)
		}
	}
	return w, h, scale
}

// splitTokensAtComma splits a token list on comma tokens at brace level 0.
func splitTokensAtComma(toks []tok) [][]tok {
	var out [][]tok
	var cur []tok
	depth := 0
	for _, t := range toks {
		if !t.cs_ {
			switch t.ch {
			case '{':
				depth++
			case '}':
				depth--
			case ',':
				if depth <= 0 {
					out = append(out, cur)
					cur = nil
					continue
				}
			}
		}
		cur = append(cur, t)
	}
	return append(out, cur)
}

// splitKeyValueTokens splits one key=value segment at its first '=' (brace level 0),
// returning the trimmed key text and the value tokens. A segment with no '=' is not a
// key/value pair (ok=false).
func splitKeyValueTokens(seg []tok) (key string, val []tok, ok bool) {
	depth := 0
	for i, t := range seg {
		if !t.cs_ {
			switch t.ch {
			case '{':
				depth++
			case '}':
				depth--
			case '=':
				if depth <= 0 {
					return strings.TrimSpace(tokensToText(seg[:i])), seg[i+1:], true
				}
			}
		}
	}
	return "", nil, false
}

// tokensToText renders a token list as its literal characters (control sequences
// contribute nothing), for a key name or a plain numeric value like scale=0.7.
func tokensToText(toks []tok) string {
	var b strings.Builder
	for _, t := range toks {
		if !t.cs_ {
			b.WriteRune(t.ch)
		}
	}
	return b.String()
}

// evalDimenTokens evaluates a token list as a TeX <dimen>, in isolation from the
// surrounding input so a value like 0.48\textwidth or \columnwidth is resolved
// through the engine's real dimension scanner (coercing \textwidth to the full text
// width, \columnwidth/\linewidth to the column measure). The token list is made the
// only input for the scan and the base string is fenced off (noBase), so nothing the
// scanner reads past the value can leak into or out of the graphics option list.
func (e *Engine) evalDimenTokens(toks []tok) int {
	if len(toks) == 0 {
		return 0
	}
	savedLists, savedNoBase := e.lists, e.noBase
	e.lists = [][]tok{append([]tok(nil), toks...)}
	e.noBase = true
	v := e.scanDimen()
	e.lists, e.noBase = savedLists, savedNoBase
	return v
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

// figureDeclaredSize returns the size a figure file states for itself, in whole
// PostScript points (bp), or (0, 0) when it states none or cannot be read. Two
// formats state one and neither can be decoded here:
//
//   - EPS, in its %%BoundingBox (or the more precise %%HiResBoundingBox) comment,
//     which is what dvips and the graphics package read;
//   - PDF, in the page's /MediaBox — used when no rasteriser is wired (the browser
//     build), since a rasterised figure carries its size in its pixels.
//
// Only the head of the file is read: both declarations are in it, and a figure can
// be tens of megabytes.
func figureDeclaredSize(name string) (w, h int) {
	f, err := os.Open(name)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	head := make([]byte, 64<<10)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	switch {
	case bytes.HasPrefix(head, []byte("%!PS")), bytes.Contains(head, []byte("%%BoundingBox")):
		return epsBoundingBox(head)
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return pdfMediaBox(head)
	}
	return 0, 0
}

// epsBoundingBox reads %%HiResBoundingBox in preference to %%BoundingBox (the
// first is in real numbers, the second rounded to integers), skipping the
// "(atend)" form an EPS may use to defer the box to its trailer — which is past
// the head this reads.
func epsBoundingBox(head []byte) (w, h int) {
	for _, key := range []string{"%%HiResBoundingBox:", "%%BoundingBox:"} {
		i := bytes.Index(head, []byte(key))
		if i < 0 {
			continue
		}
		line := head[i+len(key):]
		if j := bytes.IndexAny(line, "\r\n"); j >= 0 {
			line = line[:j]
		}
		if x, y, ok := boxExtent(strings.Fields(string(line))); ok {
			return x, y
		}
	}
	return 0, 0
}

// pdfMediaBox reads the first /MediaBox [llx lly urx ury] in the file's head. A
// figure PDF holds one page, so the first box is that page's.
func pdfMediaBox(head []byte) (w, h int) {
	i := bytes.Index(head, []byte("/MediaBox"))
	if i < 0 {
		return 0, 0
	}
	rest := head[i+len("/MediaBox"):]
	open := bytes.IndexByte(rest, '[')
	close := bytes.IndexByte(rest, ']')
	if open < 0 || close < open {
		return 0, 0
	}
	x, y, ok := boxExtent(strings.Fields(string(rest[open+1 : close])))
	if !ok {
		return 0, 0
	}
	return x, y
}

// boxExtent turns four numbers "llx lly urx ury" into the box's width and height,
// rounded to whole points. A degenerate or reversed box yields ok=false, so the
// caller keeps its own default rather than reserving nothing.
func boxExtent(f []string) (w, h int, ok bool) {
	if len(f) < 4 {
		return 0, 0, false
	}
	var v [4]float64
	for i := range v {
		x, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			return 0, 0, false
		}
		v[i] = x
	}
	dx, dy := v[2]-v[0], v[3]-v[1]
	if dx <= 0 || dy <= 0 {
		return 0, 0, false
	}
	return int(dx + 0.5), int(dy + 0.5), true
}
