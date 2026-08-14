// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file adds SVG support to \includegraphics (see image.go). An SVG image is
// carried verbatim in an imageNode of format "svg"; the two drivers realise it
// differently:
//
//   - SVG driver (boxrender.go): the image is emitted as a nested data-URI
//     <image href="data:image/svg+xml;base64,…"> at the box position and size, so a
//     browser or rsvg-convert renders the vector artwork crisply (imageNode.mime /
//     dataURI already produce the correct media type — no driver change needed).
//
//   - PDF driver (pdfdriver.go → drawSVGImage below): the SVG is drawn as native PDF
//     vector paths, scaled from its viewBox/intrinsic size into the target box, so
//     the PDF stays resolution-independent.
//
// Supported SVG subset for the PDF driver:
//   - Root sizing/coordinate system: width/height and viewBox (min-x, min-y, w, h).
//   - Elements: <svg>, <g> (transform translate()/scale()), <path> (M/L/H/V/C/Q/Z,
//     absolute and relative — via drawSVGPath in mathpdf.go), <rect>, <circle>,
//     <ellipse>, <polygon>.
//   - Solid fills via the fill attribute, inherited through <g>: #rgb / #rrggbb,
//     rgb(r,g,b), a table of common named colours, and "none"/"transparent" (skip).
//     The default fill is black, as in SVG.
//
// Out of scope (skipped without error, so a rich SVG degrades rather than crashes):
//   strokes and stroke-only shapes (<line>, <polyline>), gradients, patterns,
//   filters, clip paths, opacity, <text>, embedded raster <image>, CSS <style>/
//   class selectors, rounded-rect corners, and transform rotate/matrix/skew (only
//   translate/scale are honoured, matching go-tex/math's parseTransform).

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/go-pdfkit/pdfkit"
)

// svgDefaultSize is the fallback intrinsic size (points) for an SVG whose root has
// neither width/height nor a viewBox.
const svgDefaultSize = 72

// svgKappa is the control-point ratio for approximating a quarter ellipse with a
// cubic Bézier (4/3·(√2−1)).
const svgKappa = 0.5522847498307936

// errSVG marks an SVG that could not be parsed (no <svg> root); doIncludegraphics
// turns it into a SourceError at the \includegraphics line.
var errSVG = &graphicsError{"malformed SVG image"}

// svgDataURI reports whether a data: URI carries an SVG payload (its media type
// mentions "svg"), so loadImage can route it to the SVG loader.
func svgDataURI(uri string) bool {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 {
		return false
	}
	return strings.Contains(strings.ToLower(uri[:comma]), "svg")
}

// looksLikeSVG sniffs decoded bytes for an SVG root, covering files with an XML
// prolog or DOCTYPE ahead of <svg …>. Binary PNG/JPEG never contains "<svg".
func looksLikeSVG(data []byte) bool {
	head := data
	if len(head) > 512 {
		head = head[:512]
	}
	return bytes.Contains(head, []byte("<svg"))
}

// loadSVGImage validates the SVG root and returns the verbatim bytes, the "svg"
// format tag and the intrinsic size (points). A missing/broken root is errSVG.
func loadSVGImage(data []byte) (out []byte, format imgFormat, iw, ih int, err error) {
	attrs, ok := svgRootAttrs(data)
	if !ok {
		return nil, 0, 0, 0, errSVG
	}
	iw, ih = svgIntrinsic(attrs)
	return data, imgSVG, iw, ih, nil
}

// svgRootAttrs returns the attributes of the root <svg> element. It fails (ok=false)
// when the document does not parse or its first element is not <svg>.
func svgRootAttrs(data []byte) (map[string]string, bool) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, false
		}
		if se, ok := tok.(xml.StartElement); ok {
			if se.Name.Local != "svg" {
				return nil, false
			}
			m := make(map[string]string, len(se.Attr))
			for _, a := range se.Attr {
				m[a.Name.Local] = a.Value
			}
			return m, true
		}
	}
}

// svgIntrinsic derives the natural size (points, rounded) from the root attributes:
// width/height when both are present, else the viewBox extent, else a default. A
// single present dimension falls back to a square of that size.
func svgIntrinsic(attrs map[string]string) (iw, ih int) {
	w := parseSVGLen(attrs["width"])
	h := parseSVGLen(attrs["height"])
	if w > 0 && h > 0 {
		return roundf(w), roundf(h)
	}
	if vb := parseFloats(attrs["viewBox"]); len(vb) == 4 && vb[2] > 0 && vb[3] > 0 {
		return roundf(vb[2]), roundf(vb[3])
	}
	if w > 0 {
		return roundf(w), roundf(w)
	}
	if h > 0 {
		return roundf(h), roundf(h)
	}
	return svgDefaultSize, svgDefaultSize
}

// svgViewport returns the source coordinate system for PDF drawing: the viewBox
// (min-x, min-y, width, height) when present, else 0,0,width,height, else a default
// square. Paths are authored in this system and mapped into the target box.
func svgViewport(attrs map[string]string) (x, y, w, h float64) {
	if vb := parseFloats(attrs["viewBox"]); len(vb) == 4 {
		return vb[0], vb[1], vb[2], vb[3]
	}
	w = parseSVGLen(attrs["width"])
	h = parseSVGLen(attrs["height"])
	if w > 0 && h > 0 {
		return 0, 0, w, h
	}
	return 0, 0, svgDefaultSize, svgDefaultSize
}

// parseSVGLen reads a leading number from an SVG length ("40", "40px", "2.5pt"),
// ignoring the unit (treated as a user unit / point). Empty or non-numeric → 0.
func parseSVGLen(s string) float64 {
	s = strings.TrimSpace(s)
	i := 0
	for i < len(s) && (s[i] == '.' || s[i] == '-' || s[i] == '+' || s[i] == 'e' || s[i] == 'E' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	v, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0
	}
	return v
}

// roundf rounds a non-negative float to the nearest int.
func roundf(v float64) int { return int(v + 0.5) }

// parseSVGFill parses a fill value. It returns ok=false when the value is empty or
// unrecognised (so the caller keeps the inherited fill); none=true for
// "none"/"transparent" (draw nothing). Recognised: #rgb, #rrggbb, rgb(r,g,b) and a
// table of common named colours.
func parseSVGFill(s string) (color uint32, none, ok bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return 0, false, false
	case "none", "transparent":
		return 0, true, true
	case "currentcolor", "inherit":
		return 0, false, false
	}
	if strings.HasPrefix(s, "#") {
		if c, ok := parseHexColor(s[1:]); ok {
			return c, false, true
		}
		return 0, false, false
	}
	if strings.HasPrefix(s, "rgb(") && strings.HasSuffix(s, ")") {
		nums := parseFloats(s[4 : len(s)-1])
		if len(nums) == 3 {
			return clamp255(nums[0])<<16 | clamp255(nums[1])<<8 | clamp255(nums[2]), false, true
		}
		return 0, false, false
	}
	if c, ok := svgNamedColors[s]; ok {
		return c, false, true
	}
	return 0, false, false
}

// parseHexColor parses a 3- or 6-digit hex colour (no leading '#').
func parseHexColor(s string) (uint32, bool) {
	switch len(s) {
	case 3:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0, false
		}
		r := (v >> 8) & 0xf
		g := (v >> 4) & 0xf
		b := v & 0xf
		return uint32(r<<20 | r<<16 | g<<12 | g<<8 | b<<4 | b), true
	case 6:
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0, false
		}
		return uint32(v), true
	}
	return 0, false
}

// clamp255 clamps a float channel to the 0..255 byte range as a uint32.
func clamp255(v float64) uint32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint32(v + 0.5)
}

// svgNamedColors is the subset of SVG/CSS named colours the PDF driver resolves.
var svgNamedColors = map[string]uint32{
	"black": 0x000000, "white": 0xffffff, "red": 0xff0000, "lime": 0x00ff00,
	"green": 0x008000, "blue": 0x0000ff, "yellow": 0xffff00, "cyan": 0x00ffff,
	"aqua": 0x00ffff, "magenta": 0xff00ff, "fuchsia": 0xff00ff, "gray": 0x808080,
	"grey": 0x808080, "silver": 0xc0c0c0, "maroon": 0x800000, "olive": 0x808000,
	"navy": 0x000080, "teal": 0x008080, "purple": 0x800080, "orange": 0xffa500,
	"pink": 0xffc0cb, "brown": 0xa52a2a, "gold": 0xffd700,
}

// svgColor converts an 0xRRGGBB value to a pdfkit colour.
func svgColor(c uint32) pdfkit.Color {
	return pdfkit.RGB8(uint8(c>>16), uint8(c>>8), uint8(c))
}

// drawSVGImage draws an SVG image into a PDF page, scaled into rect r (PDF space,
// origin lower-left). The SVG's viewBox/intrinsic box is mapped onto r, and the
// supported elements are drawn as vector fills. Unsupported constructs are skipped.
// The page fill colour is changed as fills are selected; the caller re-selects its
// tracked colour afterwards (see pdfDraw.drawImage).
func drawSVGImage(p *pdfkit.Page, svg []byte, r pdfkit.Rect) {
	attrs, ok := svgRootAttrs(svg)
	if !ok {
		return
	}
	vx, vy, vw, vh := svgViewport(attrs)
	if vw <= 0 || vh <= 0 {
		return
	}
	sx := r.Width / vw
	sy := r.Height / vh
	left := r.X
	pdfTop := r.Y + r.Height // top edge in PDF (y-up) space
	// Map a source point (x,y) to viewport space: scale about the viewBox origin.
	// drawSVGPath/drawSVGRect apply this affine then place the result at
	// (left+vx, pdfTop-vy), flipping SVG's y-down into PDF's y-up.
	base := affine{a: sx, b: 0, c: 0, d: sy, e: -sx * vx, f: -sy * vy}

	dec := xml.NewDecoder(bytes.NewReader(svg))
	dec.Strict = false
	type svgFrame struct {
		m    affine
		fill uint32
		none bool
	}
	stack := []svgFrame{{m: base, fill: 0, none: false}}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			fr := stack[len(stack)-1]
			fr.m = fr.m.mul(parseTransform(attr(t, "transform")))
			if col, none, okc := parseSVGFill(attr(t, "fill")); okc {
				fr.fill, fr.none = col, none
			}
			drawSVGShape(p, t, fr.m, fr.fill, fr.none, left, pdfTop)
			stack = append(stack, fr)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		}
	}
}

// drawSVGShape fills one shape element in the current colour, honouring fill="none".
// Non-shape elements (svg, g, and anything unsupported) draw nothing.
func drawSVGShape(p *pdfkit.Page, t xml.StartElement, m affine, fill uint32, none bool, left, pdfTop float64) {
	if none {
		return
	}
	switch t.Name.Local {
	case "path":
		p.SetFillColor(svgColor(fill))
		drawSVGPath(p, attr(t, "d"), m, left, pdfTop)
	case "rect":
		p.SetFillColor(svgColor(fill))
		drawSVGRect(p, t, m, left, pdfTop)
	case "circle":
		p.SetFillColor(svgColor(fill))
		r := parseFloat(attr(t, "r"))
		drawSVGEllipse(p, parseFloat(attr(t, "cx")), parseFloat(attr(t, "cy")), r, r, m, left, pdfTop)
	case "ellipse":
		p.SetFillColor(svgColor(fill))
		drawSVGEllipse(p, parseFloat(attr(t, "cx")), parseFloat(attr(t, "cy")),
			parseFloat(attr(t, "rx")), parseFloat(attr(t, "ry")), m, left, pdfTop)
	case "polygon":
		p.SetFillColor(svgColor(fill))
		drawSVGPolygon(p, attr(t, "points"), m, left, pdfTop)
	}
}

// drawSVGEllipse fills an axis-aligned ellipse centred at (cx,cy) with radii rx,ry,
// approximated by four cubic Béziers, mapped through m and the box placement.
func drawSVGEllipse(p *pdfkit.Page, cx, cy, rx, ry float64, m affine, left, pdfTop float64) {
	if rx <= 0 || ry <= 0 {
		return
	}
	k := svgKappa
	move := func(x, y float64) {
		vx, vy := m.apply(x, y)
		p.MoveTo(left+vx, pdfTop-vy)
	}
	curve := func(x1, y1, x2, y2, x3, y3 float64) {
		a1, b1 := m.apply(x1, y1)
		a2, b2 := m.apply(x2, y2)
		a3, b3 := m.apply(x3, y3)
		p.CurveTo(left+a1, pdfTop-b1, left+a2, pdfTop-b2, left+a3, pdfTop-b3)
	}
	move(cx+rx, cy)
	curve(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry)
	curve(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy)
	curve(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry)
	curve(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy)
	p.ClosePath()
	p.Fill()
}

// drawSVGPolygon fills a closed polygon from a "points" list (x0,y0 x1,y1 …),
// mapped through m and the box placement. Fewer than three points draws nothing.
func drawSVGPolygon(p *pdfkit.Page, points string, m affine, left, pdfTop float64) {
	nums := parseFloats(points)
	if len(nums) < 6 {
		return
	}
	for i := 0; i+1 < len(nums); i += 2 {
		vx, vy := m.apply(nums[i], nums[i+1])
		if i == 0 {
			p.MoveTo(left+vx, pdfTop-vy)
		} else {
			p.LineTo(left+vx, pdfTop-vy)
		}
	}
	p.ClosePath()
	p.Fill()
}
