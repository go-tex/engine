// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file draws XY-pic's \xymatrix — the commutative diagram notation, which is
// how a category-theory or algebra paper draws a square of objects and the maps
// between them:
//
//	\[\xymatrix{
//	  A \ar@{^(->}[r]\ar[d]_f & B \ar@{=}[d] \\
//	  C \ar[r]                & D}\]
//
// # Why it is here and not loaded
//
// XY-pic draws with its OWN FONTS: xyatip10 and xybtip10 hold arrow tips, xycmat10
// holds line segments at each supported slope, and an arrow is assembled by
// setting those characters end to end. Loading the real package would mean
// bringing that whole Type1 font path with it. What a diagram actually needs is a
// grid of typeset cells and some straight lines between them, and this engine has
// both already: makeMath renders a cell, and the composed SVG a formula returns is
// exactly where lines can be drawn beside it.
//
// # The geometry, measured rather than assumed
//
// Every number here was read off tectonic's own output rather than taken from the
// manual, with `pdftotext -bbox` at the cell level and a 600dpi rasterisation for
// the arrow:
//
//	\xymatrix@C=0pc{A & B}   column pitch 13.44bp for a 7.47bp letter
//	\xymatrix@C=1pc{A & B}   25.39      (+11.95 = 1pc, in bp)
//	\xymatrix@C=2pc{A & B}   37.35      and this is also the DEFAULT
//	\xymatrix@C=3pc{A & B}   49.32
//
// so a cell is boxed with a 3pt margin on each side and the boxes are separated by
// @C — the model gives 13.47 and 37.38 against 13.44 and 37.35 measured. The
// arrow in `A \ar[r] & B` runs 24.12bp between those boxes, against 23.88 the
// model predicts.
//
// # Filled, never stroked
//
// A formula's SVG is drawn by both drivers with a fill-only vocabulary — rect and
// path, each filled, and no stroke (mathpdf.go). So every line here is a filled
// quadrilateral and every head a filled triangle. That keeps a diagram working in
// the PDF driver and the SVG driver alike without either of them learning
// anything new.

import (
	"strconv"
	"strings"
)

// xyMargin is the space a cell is boxed with on each side, in points. Arrows
// start and stop at that box, which is what keeps them clear of the glyphs.
const xyMargin = 3.0

// xyDefaultSep is XY-pic's default @C and @R: 2pc.
const xyDefaultSep = 24.0

// xyArrow is one \ar in a cell: where it points, how it is drawn, and what is
// written beside it.
type xyArrow struct {
	dr, dc int    // target offset, in rows and columns
	style  string // the @{…} body, "" for a plain ->
	curve  string // the @/…/ body: "^", "_", "^1pc" …; "" for a straight arrow
	labels []xyLabel
}

// xyLabel is a _below or ^above annotation on an arrow.
type xyLabel struct {
	text string
	side byte    // '_' below/right of travel, '^' above/left, '|' on the line
	pos  float64 // where along the arrow, 0 at the tail and 1 at the head
}

// xyCell is one entry of the matrix.
type xyCell struct {
	math   string
	arrows []xyArrow
}

// xymatrixSource recognises a formula that is nothing but one \xymatrix, and
// returns its options and body.
//
// Only the whole-formula case is taken. A \xymatrix embedded in a larger formula
// is left to the maths layer to refuse, because the surrounding material would
// have to be laid out around it and this does not do that — saying so plainly is
// better than drawing half of it.
func xymatrixSource(src string) (opts, body string, ok bool) {
	s := strings.TrimSpace(src)
	const cs = `\xymatrix`
	if !strings.HasPrefix(s, cs) {
		return "", "", false
	}
	s = s[len(cs):]
	// Options run up to the opening brace: @C=1pc, @R=2pc, @!0, @1 …
	i := strings.IndexByte(s, '{')
	if i < 0 {
		return "", "", false
	}
	opts = s[:i]
	body, rest := splitBrace(s[i:])
	if strings.TrimSpace(rest) != "" {
		return "", "", false // something follows the matrix; not ours
	}
	return opts, body, true
}

// splitBrace takes a string starting at '{' and returns the group's contents and
// whatever follows it.
func splitBrace(s string) (inner, rest string) {
	depth := 0
	for i, r := range s {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[1:i], s[i+1:]
			}
		}
	}
	return s[1:], ""
}

// xySep reads @C= / @R= out of the option string, in points.
func xySep(opts string, key byte) float64 {
	tag := "@" + string(key) + "="
	i := strings.Index(opts, tag)
	if i < 0 {
		return xyDefaultSep
	}
	rest := opts[i+len(tag):]
	// A dimension: a number then a unit.
	j := 0
	for j < len(rest) && (rest[j] == '-' || rest[j] == '.' || (rest[j] >= '0' && rest[j] <= '9')) {
		j++
	}
	v := parseFloat(rest[:j])
	switch {
	case strings.HasPrefix(rest[j:], "pc"):
		return v * 12
	case strings.HasPrefix(rest[j:], "pt"):
		return v
	case strings.HasPrefix(rest[j:], "mm"):
		return v * 72.27 / 25.4
	case strings.HasPrefix(rest[j:], "cm"):
		return v * 72.27 / 2.54
	case strings.HasPrefix(rest[j:], "in"):
		return v * 72.27
	case strings.HasPrefix(rest[j:], "ex"):
		return v * 4.3
	case strings.HasPrefix(rest[j:], "em"):
		return v * 10
	}
	return xyDefaultSep
}

// parseXymatrix splits a body into rows and cells, pulling each \ar out of the
// cell it sits in.
func parseXymatrix(body string) [][]xyCell {
	var out [][]xyCell
	for _, row := range splitTop(body, `\\`) {
		var cells []xyCell
		for _, c := range splitTopByte(row, '&') {
			cells = append(cells, parseXyCell(c))
		}
		out = append(out, cells)
	}
	return out
}

// splitTop splits on a separator that appears at brace depth zero.
func splitTop(s, sep string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '\\':
			if depth == 0 && strings.HasPrefix(s[i:], sep) {
				out = append(out, s[start:i])
				i += len(sep) - 1
				start = i + 1
			} else if i+1 < len(s) {
				i++ // an escaped character is not a delimiter
			}
		}
	}
	return append(out, s[start:])
}

// splitTopByte splits on a single byte at brace depth zero.
func splitTopByte(s string, sep byte) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '\\':
			i++ // skip the escaped character
		case sep:
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

// parseXyCell separates a cell's maths from the \ar declarations in it.
func parseXyCell(s string) xyCell {
	var c xyCell
	var math strings.Builder
	for {
		i := strings.Index(s, `\ar`)
		if i < 0 {
			math.WriteString(s)
			break
		}
		math.WriteString(s[:i])
		rest := s[i+3:]
		a, after, ok := parseXyArrow(rest)
		if !ok {
			math.WriteString(`\ar`)
			s = rest
			continue
		}
		c.arrows = append(c.arrows, a)
		s = after
	}
	c.math = strings.TrimSpace(math.String())
	return c
}

// parseXyArrow reads what follows \ar: any number of @modifiers, the
// [direction], and any _below / ^above / |on-the-line labels.
//
// A single \ar can carry SEVERAL @ groups — \ar@/^/@{.>}[r] is a dotted arrow
// that also curves — so they are read in a loop rather than one at a time.
func parseXyArrow(s string) (xyArrow, string, bool) {
	var a xyArrow
	for {
		s = strings.TrimLeft(s, " \t\n")
		if !strings.HasPrefix(s, "@") {
			break
		}
		rest := s[1:]
		switch {
		case strings.HasPrefix(rest, "{"):
			a.style, s = splitBrace(rest)
		case strings.HasPrefix(rest, "/"):
			// @/…/ curves the arrow. The body is a direction and an optional
			// distance: ^, _, ^1pc, _-.5pc …
			j := strings.IndexByte(rest[1:], '/')
			if j < 0 {
				return a, s, false
			}
			a.curve, s = rest[1:1+j], rest[j+2:]
		default:
			// @^, @_, @2, @3, @(…): a variant this does not draw differently.
			// Skip it, stopping at whatever comes next rather than swallowing it.
			j := strings.IndexAny(rest, "[@")
			if j < 0 {
				return a, s, false
			}
			if a.style == "" {
				a.style = rest[:j]
			}
			s = rest[j:]
		}
	}
	if !strings.HasPrefix(s, "[") {
		return a, s, false
	}
	j := strings.IndexByte(s, ']')
	if j < 0 {
		return a, s, false
	}
	for _, r := range s[1:j] {
		switch r {
		case 'r':
			a.dc++
		case 'l':
			a.dc--
		case 'd':
			a.dr++
		case 'u':
			a.dr--
		}
	}
	s = s[j+1:]
	// Labels: _x, ^x, |x — a single token or a braced group, each optionally
	// preceded by a PLACE saying where along the arrow it goes.
	for {
		if s == "" {
			break
		}
		side := s[0]
		if side != '_' && side != '^' && side != '|' {
			break
		}
		rest := s[1:]
		pos, rest := xyPlace(rest)
		var text string
		if strings.HasPrefix(rest, "{") {
			text, rest = splitBrace(rest)
		} else {
			text, rest = firstToken(rest)
		}
		if text == "" {
			break
		}
		a.labels = append(a.labels, xyLabel{text: text, side: side, pos: pos})
		s = rest
	}
	return a, s, true
}

// xyPlace reads the optional <place> between a label's ^_| and the label itself,
// and returns where along the arrow the label goes.
//
// XY-pic's grammar (xyarrow.tex, \PATHanchor@i) treats a bare - as the place
// <>(.5) — the middle of the connection — and (f) as the fraction f along it. The
// middle is also where a label with no place at all goes, so - changes nothing
// here; what matters is that it is CONSUMED. Read as a label it became the text
// of \ar[dr]|-{(x,y)}, and the (x,y) that followed fell through into the cell.
func xyPlace(s string) (float64, string) {
	const middle = 0.5
	pos := middle
	for {
		switch {
		case strings.HasPrefix(s, "-"):
			s = s[1:]
		case strings.HasPrefix(s, "<") || strings.HasPrefix(s, ">"):
			s = s[1:]
		case strings.HasPrefix(s, "("):
			j := strings.IndexByte(s, ')')
			if j < 0 {
				return pos, s
			}
			v, err := strconv.ParseFloat(strings.TrimSpace(s[1:j]), 64)
			if err != nil || v < 0 || v > 1 {
				return pos, s // not a place: leave it to be read as the label
			}
			pos = v
			s = s[j+1:]
		default:
			return pos, s
		}
	}
}

// firstToken takes one TeX token: a control sequence or a single character.
func firstToken(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	if s[0] != '\\' {
		return s[:1], s[1:]
	}
	i := 1
	for i < len(s) && isLetterByte(s[i]) {
		i++
	}
	if i == 1 && i < len(s) {
		i = 2 // a control symbol
	}
	return s[:i], s[i:]
}

func isLetterByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}
