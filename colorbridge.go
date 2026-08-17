// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"strings"
)

// This file publishes the engine's named colours in the form the color/xcolor
// packages store theirs, so a package that asks "what colour is this, exactly?"
// gets an answer.
//
// The engine resolves colour natively (see color.go) rather than loading xcolor,
// which is fine as long as nothing looks *inside* a colour. A drawing package
// does: pgf sets a stroke colour with \colorlet and then reads the result back —
//
//	\expandafter\let\expandafter\pgf@temp\csname\string\color@pgfstrokecolor\endcsname
//	\expandafter\pgf@setstrokecolor\pgf@temp   % → #1#2#3#4#5, #4 = model, #5 = values
//
// — in order to hand the model and its values to its own driver. Without the
// stored form it reads whatever happens to follow and reports the colour model as
// unsupported, so no picture can set a colour at all.
//
// The stored form is xcolor's: the control sequence \color@<name> expands to five
// arguments, of which the fourth is the model and the fifth its comma-separated
// values. The engine keeps colours as RGB, so the model published is rgb with
// three 0–1 components — the model every driver understands.

// publishNamedColors publishes the standard colour names at start-up, so a
// package can read one that the document never defined itself. A drawing package
// treats an unrecognised option as a colour name and asks whether it is one
// (TikZ: \draw[red] works only because \color@red exists), so the built-in table
// has to be visible too, not just what \definecolor added.
func (e *Engine) publishNamedColors() {
	for name, c := range namedColors {
		e.publishColor(name, c)
	}
}

// publishColor stores a colour under the name a colour-reading package expects,
// alongside the engine's own table. It is called wherever a colour gets a name.
func (e *Engine) publishColor(name string, c uint32) {
	if name == "" {
		return
	}
	// \string\color@<name> is the control sequence's *characters*, which is the
	// name xcolor builds with \csname: a literal backslash, then "color@<name>".
	e.define(`\color@`+name, &meaning{kind: mMacro, body: xcolorValue(c)}, true)
}

// xcolorValue renders 0xRRGGBB as xcolor's stored five arguments.
func xcolorValue(c uint32) []tok {
	r := float64((c>>16)&0xff) / 255
	g := float64((c>>8)&0xff) / 255
	b := float64(c&0xff) / 255
	spec := fmt.Sprintf("%s,%s,%s", colorComponent(r), colorComponent(g), colorComponent(b))
	// The value is a token list, not text: the reader destructures it into five
	// arguments, so the groups have to be real begin/end-group tokens and the
	// leading \xcolor@ a real control sequence.
	out := []tok{csTok("xcolor@")}
	for _, group := range []string{"", "", "rgb", spec} {
		out = append(out, chTok('{', catBegin))
		for _, r := range group {
			out = append(out, chTok(r, catOther))
		}
		out = append(out, chTok('}', catEnd))
	}
	return out
}

// colorComponent formats one 0–1 colour component the way a colour specification
// spells it: at most five decimals, with trailing zeros trimmed.
func colorComponent(v float64) string {
	s := fmt.Sprintf("%.5f", v)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}
