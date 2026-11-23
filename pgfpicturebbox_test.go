// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A pgfpicture written in pgf's old syntax, \begin{pgfpicture}{x0}{y0}{x1}{y1},
// declares its bounding box, and pgf sets the picture as a box exactly x1-x0
// wide and y1-y0 tall with depth 0 (pgfcorescopes.code.tex:218-225,
// pgfsys.code.tex:1596-1611). The gobbled picture keeps that footprint instead
// of the framed 96x60pt placeholder. beamer's ball enumerate item is such a
// picture, and the placeholder put its lines 70pt apart (go-tex/engine#884).
func TestAPgfPictureKeepsItsDeclaredBoundingBox(t *testing.T) {
	for _, c := range []struct {
		name, src string
		w, h      int
	}{
		{"declared box", `\global\setbox1\hbox{\begin{pgfpicture}{-1pt}{-2pt}{3pt}{4pt}\pgfusepath{fill}\end{pgfpicture}}`, 4 * unity, 6 * unity},
		{"spaces between the arguments", `\global\setbox1\hbox{\begin{pgfpicture} {0pt} {0pt} {10pt} {5pt}\end{pgfpicture}}`, 10 * unity, 5 * unity},
		{"an inverted box is empty", `\global\setbox1\hbox{\begin{pgfpicture}{3pt}{4pt}{1pt}{2pt}\end{pgfpicture}}`, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}`+c.src+`x\end{document}`), Options{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			b := e.box[1]
			if b == nil {
				t.Fatal("box 1 is void")
			}
			if b.width != c.w || b.height != c.h || b.depth != 0 {
				t.Errorf("%s\n  wd=%d ht=%d dp=%d, want wd=%d ht=%d dp=0", c.src, b.width, b.height, b.depth, c.w, c.h)
			}
			for _, n := range b.list {
				if _, ok := n.(frameNode); ok {
					t.Errorf("%s: a framed placeholder was left; the declared box should replace it", c.src)
				}
			}
		})
	}
}

// A pgfpicture with no declared box — how TikZ and most pgf code open one, with a
// command or text right after \begin{pgfpicture} — whose size its body does not
// tell (pgfbbox.go reads only straight paths between \pgfpoint corners) still gets
// the framed placeholder every gobbled picture gets.
func TestAPgfPictureWithoutABoundingBoxKeepsThePlaceholder(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{"a command follows", `\global\setbox1\hbox{\begin{pgfpicture}\pgfpathcircle{\pgfpoint{0pt}{0pt}}{3pt}\pgfusepath{stroke}\end{pgfpicture}}`},
		{"text follows", `\global\setbox1\hbox{\begin{pgfpicture} a\end{pgfpicture}}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile([]byte(`\documentclass{article}\begin{document}`+c.src+`x\end{document}`), Options{Lenient: true})
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
				t.Errorf("%s: no framed placeholder (wd=%d ht=%d)", c.src, b.width, b.height)
			}
		})
	}
}

// The declared-size picture is still a dropped figure, counted once, and an
// unterminated one still leaves nothing, as every gobbled picture does.
func TestAPgfPictureWithABoundingBoxIsStillADroppedFigure(t *testing.T) {
	if n := runPics(t, `\begin{pgfpicture}{0pt}{0pt}{1pt}{1pt}\pgfusepath{stroke}\end{pgfpicture}\zzp `); n != 1 {
		t.Errorf("dropped pictures = %d, want 1", n)
	}
	if n := runPics(t, `before \begin{pgfpicture}{0pt}{0pt}{1pt}{1pt}\pgfusepath{stroke} no end`); n != 0 {
		t.Errorf("unterminated picture counted %d, want 0", n)
	}
	skip := runSkip(t, `\begin{pgfpicture}{0pt}{0pt}{1pt}{1pt}\pgfusepath{stroke}\end{pgfpicture}\zzp `)
	noLeak(t, skip, "pgfusepath")
	if skip["zzp"] != 1 {
		t.Errorf("content after the picture was not processed: zzp=%d", skip["zzp"])
	}
}
