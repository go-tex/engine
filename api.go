// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
	"io"

	texmath "github.com/go-tex/math"
)

// This file is the high-level compile API: one call turns TeX source into a PDF
// or SVG pages. It is the integration seam for an editor like loom (both are Go,
// so loom imports this package and calls Compile directly — no subprocess, no
// TeXLive). The gotex command is a thin wrapper over the same functions.

// Options configures a compile. The zero value is valid: a built-in font at 10pt
// with a 72pt (1 inch) margin.
type Options struct {
	Font    []byte  // text font (.ttf/.otf); nil ⇒ the built-in default
	Size    int     // font size in points (0 ⇒ 10)
	Margin  float64 // page margin in points (0 ⇒ 72)
	Plain   bool    // load the Plain structural macros (default true via NewDocument)
	NoPlain bool    // set to omit the Plain macros even in NewDocument
}

func (o Options) size() int {
	if o.Size <= 0 {
		return 10
	}
	return o.Size
}

func (o Options) margin() float64 {
	if o.Margin <= 0 {
		return 72
	}
	return o.Margin
}

// NewDocument builds an engine configured from opts: the Plain macros loaded
// (unless NoPlain) and the current text font set. It is the starting point for
// programmatic use when callers want to Run source incrementally before
// rendering.
func NewDocument(opt Options) (*Engine, error) {
	e := New()
	if !opt.NoPlain {
		if err := e.LoadPlain(); err != nil {
			return nil, fmt.Errorf("texengine: loading macros: %w", err)
		}
	}
	fontBytes := opt.Font
	if fontBytes == nil {
		fontBytes = texmath.DefaultFont()
	}
	f, err := NewOpenTypeFont(fontBytes, opt.size())
	if err != nil {
		return nil, fmt.Errorf("texengine: font: %w", err)
	}
	e.SetFont(f)
	return e, nil
}

// CompileToPDF processes TeX source and writes a PDF to w, returning the page
// count. A document's own \font/\hsize/… override the option defaults.
func CompileToPDF(src []byte, opt Options, w io.Writer) (int, error) {
	e, err := NewDocument(opt)
	if err != nil {
		return 0, err
	}
	if _, err := e.Run(string(src)); err != nil {
		return 0, err
	}
	if err := e.RenderPDF(w, opt.margin()); err != nil {
		return 0, err
	}
	return len(e.Pages()), nil
}

// CompileToSVGPages processes TeX source and returns one SVG string per page —
// the form an editor preview pane consumes directly.
func CompileToSVGPages(src []byte, opt Options) ([]string, error) {
	e, err := NewDocument(opt)
	if err != nil {
		return nil, err
	}
	if _, err := e.Run(string(src)); err != nil {
		return nil, err
	}
	return e.RenderPages(opt.margin()), nil
}
