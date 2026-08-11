// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements colour (the color / xcolor packages): \color and
// \textcolor for text, \colorbox / \fcolorbox for filled (and framed) boxes, and
// \definecolor to name new colours. Colours are 0xRRGGBB values; a set of standard
// names is built in, and \definecolor adds more (rgb / RGB / gray / HTML models).
// A colour of 0 means "default" (black text, no box fill), so the zero value needs
// no special casing. Every glyph is stamped with the current colour and both
// drivers honour it (SVG fill, PDF SetFillColor).

import (
	"strconv"
	"strings"
)

// namedColors are the standard color/xcolor names (dvips-ish values).
var namedColors = map[string]uint32{
	"black": 0x000000, "white": 0xFFFFFF, "red": 0xFF0000, "green": 0x00FF00,
	"blue": 0x0000FF, "cyan": 0x00FFFF, "magenta": 0xFF00FF, "yellow": 0xFFFF00,
	"gray": 0x808080, "grey": 0x808080, "darkgray": 0x404040, "lightgray": 0xBFBFBF,
	"brown": 0xBF8040, "orange": 0xFF8000, "pink": 0xFFBFBF, "purple": 0xBF0040,
	"violet": 0x800080, "olive": 0x808000, "teal": 0x008080, "lime": 0xBFFF00,
}

// resolveColor maps a colour name to its 0xRRGGBB value, checking user-defined
// (\definecolor) names first, then the built-ins. Unknown → black.
func (e *Engine) resolveColor(name string) uint32 {
	name = strings.TrimSpace(name)
	if e.colors != nil {
		if c, ok := e.colors[name]; ok {
			return c
		}
	}
	if c, ok := namedColors[name]; ok {
		return c
	}
	return 0
}

// selectColor makes c the current text colour, saving the previous one for
// restoration at the end of the current group (so { \color{red} … } reverts).
func (e *Engine) selectColor(c uint32) {
	if len(e.groups) > 0 {
		e.save = append(e.save, saveItem{kind: 8, oldi: int(e.curColor)})
	}
	e.curColor = c
}

// doColor implements \color{name}: switch the current colour (group-scoped).
func (e *Engine) doColor() {
	e.selectColor(e.resolveColor(e.readBraceName()))
}

// doColorbox implements \colorbox{name}{content}: content on a filled background
// (with \fboxsep padding, no border).
func (e *Engine) doColorbox() frameNode {
	bg := e.resolveColor(e.readBraceName())
	list, _ := e.grabHboxList()
	return frameNode{inner: hpackSP(list, packNatural, 0), sep: fboxSep, rule: 0, bg: bg}
}

// doFcolorbox implements \fcolorbox{frame}{bg}{content}: a filled background with a
// coloured frame.
func (e *Engine) doFcolorbox() frameNode {
	frame := e.resolveColor(e.readBraceName())
	bg := e.resolveColor(e.readBraceName())
	list, _ := e.grabHboxList()
	return frameNode{inner: hpackSP(list, packNatural, 0), sep: fboxSep, rule: fboxRule, bg: bg, ruleColor: frame}
}

// doDefineColor implements \definecolor{name}{model}{spec}, adding a named colour.
// Models: rgb (three 0–1 floats), RGB (three 0–255 ints), gray (one 0–1 float),
// HTML (six hex digits).
func (e *Engine) doDefineColor() {
	name := e.readBraceName()
	model := e.readBraceName()
	spec := e.readBraceName()
	if name == "" {
		return
	}
	if e.colors == nil {
		e.colors = map[string]uint32{}
	}
	e.colors[name] = parseColorSpec(model, spec)
}

// parseColorSpec turns a color model + spec string into 0xRRGGBB.
func parseColorSpec(model, spec string) uint32 {
	fields := strings.Split(spec, ",")
	switch model {
	case "rgb":
		if len(fields) == 3 {
			return packRGB(unitToByte(fields[0]), unitToByte(fields[1]), unitToByte(fields[2]))
		}
	case "RGB":
		if len(fields) == 3 {
			return packRGB(intToByte(fields[0]), intToByte(fields[1]), intToByte(fields[2]))
		}
	case "gray", "grey":
		g := unitToByte(spec)
		return packRGB(g, g, g)
	case "HTML":
		if v, err := strconv.ParseUint(strings.TrimSpace(spec), 16, 32); err == nil {
			return uint32(v) & 0xFFFFFF
		}
	}
	return 0
}

func packRGB(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

// unitToByte parses a 0–1 float component to a 0–255 byte.
func unitToByte(s string) uint8 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	if f < 0 {
		f = 0
	} else if f > 1 {
		f = 1
	}
	return uint8(f*255 + 0.5)
}

// intToByte parses a 0–255 integer component, clamped.
func intToByte(s string) uint8 {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	if n < 0 {
		n = 0
	} else if n > 255 {
		n = 255
	}
	return uint8(n)
}

// hexColor formats a 0xRRGGBB value as an SVG/CSS colour "#rrggbb".
func hexColor(c uint32) string {
	const hex = "0123456789abcdef"
	b := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i := 0; i < 6; i++ {
		shift := uint(20 - 4*i)
		b[1+i] = hex[(c>>shift)&0xF]
	}
	return string(b)
}
