// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"math"

	"github.com/go-pdfkit/reader"
)

// A Run is one text-showing operator's worth of glyphs: where it starts on the
// page, how wide it is, the face and size it was shown in, and what it says.
// Positions are in PDF user space (origin bottom-left), already through the
// text matrix and the CTM.
type Run struct {
	X, Y  float64 // the run's origin, on its baseline
	W     float64 // the advance the glyphs take, 0 when the font's widths are unknown
	Font  string  // the BaseFont name, with any subset prefix stripped
	Size  float64 // the effective size, text font size times the matrix scale
	Text  string  // decoded through the font's ToUnicode, "" when it has none
	Bytes int     // how many bytes the operand held, whatever decoding produced
}

// matrix is a PDF transformation [a b c d e f].
type matrix struct{ a, b, c, d, e, f float64 }

var identity = matrix{1, 0, 0, 1, 0, 0}

// mul returns m × n, the order PDF composes in: m applied first, then n.
func (m matrix) mul(n matrix) matrix {
	return matrix{
		a: m.a*n.a + m.b*n.c,
		b: m.a*n.b + m.b*n.d,
		c: m.c*n.a + m.d*n.c,
		d: m.c*n.b + m.d*n.d,
		e: m.e*n.a + m.f*n.c + n.e,
		f: m.e*n.b + m.f*n.d + n.f,
	}
}

// apply maps the point (x, y).
func (m matrix) apply(x, y float64) (float64, float64) {
	return m.a*x + m.c*y + m.e, m.b*x + m.d*y + m.f
}

// scale is the matrix's horizontal magnification, which is what a font size has
// to be multiplied by to give the size a reader sees.
func (m matrix) scale() float64 {
	if m.b == 0 {
		return abs(m.a)
	}
	return hypot(m.a, m.b)
}

// textState is the part of the graphics state that positions text.
type textState struct {
	ctm        matrix
	tm, tlm    matrix
	font       *fontInfo
	fontSize   float64
	leading    float64
	charSp     float64
	wordSp     float64
	horizScale float64 // Tz, as a fraction (100 Tz is 1)
	rise       float64
}

func newTextState() textState {
	return textState{ctm: identity, tm: identity, tlm: identity, horizScale: 1}
}

// PageRuns walks one page's content stream and returns its text runs in the
// order they were drawn. A page whose fonts cannot be read still yields runs:
// their Text is empty and their W is zero, because a position measured without
// a width is still a position, and saying so beats guessing one.
func PageRuns(doc *reader.Document, page int) ([]Run, error) {
	ops, err := doc.PageOperations(page)
	if err != nil {
		return nil, err
	}
	fonts := pageFonts(doc, page)
	st := newTextState()
	var stack []textState
	var runs []Run
	for _, op := range ops {
		switch op.Operator {
		case "q":
			// q/Q save the whole graphics state, and the character, word and
			// horizontal spacing are part of it — not only the matrix. Saving the
			// matrix alone leaks a Tc set inside a q out past its Q, which widens
			// every run after it by that spacing.
			stack = append(stack, st)
		case "Q":
			if n := len(stack); n > 0 {
				tm, tlm := st.tm, st.tlm // the text matrices are NOT part of it
				st, stack = stack[n-1], stack[:n-1]
				st.tm, st.tlm = tm, tlm
			}
		case "cm":
			if m, ok := matrixOf(op.Operands); ok {
				st.ctm = m.mul(st.ctm)
			}
		case "BT":
			st.tm, st.tlm = identity, identity
		case "Tf":
			if len(op.Operands) == 2 {
				if n, ok := reader.ToName(op.Operands[0]); ok {
					st.font = fonts[string(n)]
				}
				st.fontSize, _ = reader.ToFloat(op.Operands[1])
			}
		case "TL":
			st.leading = floatArg(op.Operands, 0)
		case "Tc":
			st.charSp = floatArg(op.Operands, 0)
		case "Tw":
			st.wordSp = floatArg(op.Operands, 0)
		case "Tz":
			st.horizScale = floatArg(op.Operands, 0) / 100
		case "Ts":
			st.rise = floatArg(op.Operands, 0)
		case "Td":
			st.nextLine(floatArg(op.Operands, 0), floatArg(op.Operands, 1))
		case "TD":
			ty := floatArg(op.Operands, 1)
			st.leading = -ty
			st.nextLine(floatArg(op.Operands, 0), ty)
		case "Tm":
			if m, ok := matrixOf(op.Operands); ok {
				st.tm, st.tlm = m, m
			}
		case "T*":
			st.nextLine(0, -st.leading)
		case "Tj":
			runs = st.show(runs, op.Operands, 0)
		case "'":
			st.nextLine(0, -st.leading)
			runs = st.show(runs, op.Operands, 0)
		case "\"":
			if len(op.Operands) == 3 {
				st.wordSp = floatArg(op.Operands, 0)
				st.charSp = floatArg(op.Operands, 1)
			}
			st.nextLine(0, -st.leading)
			runs = st.show(runs, op.Operands, len(op.Operands)-1)
		case "TJ":
			arr, ok := reader.ToArray(firstOperand(op.Operands))
			if !ok {
				continue
			}
			for _, el := range arr {
				if n, ok := reader.ToFloat(el); ok {
					st.advance(-n / 1000 * st.fontSize * st.horizScale)
					continue
				}
				runs = st.showString(runs, el)
			}
		}
	}
	return runs, nil
}

// nextLine moves the line matrix, as Td does, and resets the text matrix to it.
func (st *textState) nextLine(tx, ty float64) {
	st.tlm = matrix{1, 0, 0, 1, tx, ty}.mul(st.tlm)
	st.tm = st.tlm
}

// advance moves the text matrix along its own x axis by w text-space units.
func (st *textState) advance(w float64) {
	st.tm = matrix{1, 0, 0, 1, w, 0}.mul(st.tm)
}

// show emits one run for the operand at index i (the last operand for ' and ").
func (st *textState) show(runs []Run, operands []reader.Object, i int) []Run {
	if i >= len(operands) {
		return runs
	}
	return st.showString(runs, operands[i])
}

// showString emits the run for one PDF string and advances past it.
func (st *textState) showString(runs []Run, o reader.Object) []Run {
	b, ok := reader.ToString(o)
	if !ok {
		return runs
	}
	trm := matrix{st.fontSize * st.horizScale, 0, 0, st.fontSize, 0, st.rise}.mul(st.tm).mul(st.ctm)
	x, y := trm.apply(0, 0)
	r := Run{X: x, Y: y, Font: "?", Size: st.fontSize * st.tm.mul(st.ctm).scale(), Bytes: len(b)}
	var w float64
	if st.font != nil {
		r.Font = st.font.Name
		r.Text = st.font.Decode(b)
		w = st.font.Width(b, st.fontSize, st.charSp, st.wordSp) * st.horizScale
		r.W = w * st.tm.mul(st.ctm).scale()
	}
	st.advance(w)
	return append(runs, r)
}

func firstOperand(ops []reader.Object) reader.Object {
	if len(ops) == 0 {
		return nil
	}
	return ops[len(ops)-1]
}

func floatArg(ops []reader.Object, i int) float64 {
	if i >= len(ops) {
		return 0
	}
	v, _ := reader.ToFloat(ops[i])
	return v
}

func matrixOf(ops []reader.Object) (matrix, bool) {
	if len(ops) < 6 {
		return identity, false
	}
	var v [6]float64
	for i := 0; i < 6; i++ {
		f, ok := reader.ToFloat(ops[len(ops)-6+i])
		if !ok {
			return identity, false
		}
		v[i] = f
	}
	return matrix{v[0], v[1], v[2], v[3], v[4], v[5]}, true
}

func abs(x float64) float64      { return math.Abs(x) }
func hypot(a, b float64) float64 { return math.Hypot(a, b) }
