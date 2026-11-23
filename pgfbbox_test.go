// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A pgfpicture that declares no box, but whose body is only straight path
// construction between \pgfpoint corners and the commands that paint it, gets the
// box pgf would grow from those points (pgfbbox.go): beamer's shadow headline ends
// with such a picture, and the 96x60pt placeholder made it 64.8pt tall
// (go-tex/engine#926).
func TestAPgfPathPictureKeepsThePathsBoundingBox(t *testing.T) {
	for _, c := range []struct {
		name, body string
		w, h       int
	}{
		{"rectangle: a corner and a size", `\pgfpathrectangle{\pgfpoint{0pt}{-4pt}}{\pgfpoint{20pt}{4pt}}\pgfusepath{fill}`, 20 * unity, 4 * unity},
		{"beamer's shadow, fading, colour and a register", `\pgfpathrectangle{\pgfpoint{0pt}{-4pt}}{\pgfpoint{\dimen0}{4pt}}\pgfsetfillcolor{black}\pgfsetfading{bmb@fading}{\pgftransformshift{\pgfpoint{0pt}{-2pt}}}\pgfusepath{fill}`, 33 * unity, 4 * unity},
		{"lines grow the box both ways", `\pgfsetlinewidth{2pt}\pgfpathmoveto{\pgfpoint{-3pt}{1pt}}\pgfpathlineto{\pgfpoint{5pt}{-2pt}}\pgfpathlineto{\pgfpoint{1pt}{6pt}}\pgfpathclose\pgfusepath{stroke}`, 8 * unity, 8 * unity},
		{"one point is an empty box", `\pgfpathmoveto{\pgfpoint{2pt}{2pt}}\pgfusepath{stroke}`, 0, 0},
		{"a unit and spaces", ` \pgfpathrectangle {\pgfpoint{0pt}{0pt}} {\pgfpoint{1em}{ 2pt }} \relax `, -1, 2 * unity},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}\dimen0=33pt\global\setbox1\hbox{\begin{pgfpicture}`+c.body+`\end{pgfpicture}}x\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			b := e.box[1]
			if b == nil {
				t.Fatal("box 1 is void")
			}
			for _, n := range b.list {
				if _, ok := n.(frameNode); ok {
					t.Fatalf("%s: a framed placeholder was left; the path's box should replace it", c.body)
				}
			}
			w := c.w
			if w < 0 {
				w = b.width // 1em: the font's quad, not pinned here
				if w <= 0 {
					t.Errorf("1em measured %d, want a positive width", w)
				}
			}
			if b.width != w || b.height != c.h || b.depth != 0 {
				t.Errorf("%s\n  wd=%d ht=%d dp=%d, want wd=%d ht=%d dp=0", c.body, b.width, b.height, b.depth, w, c.h)
			}
		})
	}
}

// Anything whose extent the body does not tell — a curve, a node, text, a point
// that is not a \pgfpoint, a dimension with something after it — keeps the
// placeholder, and reading the body leaves nothing behind on the page.
func TestAPgfPictureWithAnUnknownCommandKeepsThePlaceholder(t *testing.T) {
	for _, c := range []struct{ name, body string }{
		{"a curve", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}}\pgfpathcurveto{\pgfpoint{1pt}{1pt}}{\pgfpoint{2pt}{1pt}}{\pgfpoint{3pt}{0pt}}\pgfusepath{stroke}`},
		{"a polar point", `\pgfpathmoveto{\pgfpointpolar{30}{1cm}}\pgfusepath{stroke}`},
		{"text", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}} a`},
		{"a dimension with junk", `\pgfpathmoveto{\pgfpoint{0pt JUNK}{0pt}}`},
		{"a y dimension with junk", `\pgfpathmoveto{\pgfpoint{0pt}{0pt JUNK}}`},
		{"an empty coordinate", `\pgfpathmoveto{\pgfpoint{}{0pt}}\pgfusepath{stroke}`},
		{"something after the point", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}\relax}`},
		{"a paint command missing its argument", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}}\pgfusepath`},
		{"a paint command with an unbraced argument", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}}\pgfusepath x{fill}`},
		{"a group after a paint command's argument", `\pgfpathmoveto{\pgfpoint{0pt}{0pt}}\pgfusepath{fill}{TEXT}`},
		{"no point at all", `\pgfusepath{stroke}`},
		{"a missing argument", `\pgfpathrectangle{\pgfpoint{0pt}{0pt}}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}\global\setbox1\hbox{\begin{pgfpicture}`+c.body+`\end{pgfpicture}}AFTER\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			b := e.box[1]
			if b == nil {
				t.Fatal("box 1 is void")
			}
			framed := false
			for _, n := range b.list {
				if _, ok := n.(frameNode); ok {
					framed = true
				}
			}
			if !framed {
				t.Errorf("%s: no framed placeholder (wd=%d ht=%d)", c.body, b.width, b.height)
			}
			if got := pageChars(e); got != "AFTER" {
				t.Errorf("the page reads %q, want only AFTER", got)
			}
		})
	}
}

// The path picture is still a dropped figure, counted once, and an unterminated
// one still leaves nothing.
func TestAPgfPathPictureIsStillADroppedFigure(t *testing.T) {
	if n := runPics(t, `\begin{pgfpicture}\pgfpathrectangle{\pgfpoint{0pt}{0pt}}{\pgfpoint{1pt}{1pt}}\pgfusepath{fill}\end{pgfpicture}\zzp `); n != 1 {
		t.Errorf("dropped pictures = %d, want 1", n)
	}
	if n := runPics(t, `before \begin{pgfpicture}\pgfpathrectangle{\pgfpoint{0pt}{0pt}}{\pgfpoint{1pt}{1pt}} no end`); n != 0 {
		t.Errorf("unterminated picture counted %d, want 0", n)
	}
}
