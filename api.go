// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
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
	Font       []byte  // roman text font (.ttf/.otf); nil ⇒ the built-in default
	BoldFont   []byte  // optional bold face, bound to \bf (so \textbf really bolds)
	ItalicFont []byte  // optional italic face, bound to \it (so \textit/\emph slant)
	MonoFont   []byte  // optional monospace face, bound to \tt (so \texttt is fixed-width)
	SansFont   []byte  // optional sans-serif face, bound to \sf (so \textsf is sans)
	Size       int     // font size in points (0 ⇒ 10)
	Margin     float64 // page margin in points (0 ⇒ 72)
	NoPlain    bool    // set to omit the Plain macros
	Date       string  // \today text (a pure-Go wasm build has no clock; supply it here)

	// Lenient turns an undefined control sequence from a fatal error into a
	// skipped command: the engine drops the unknown \cs (and its likely
	// [optional]/{mandatory} argument block) and carries on, recording the name
	// for reporting. This is best-effort "produce something" behaviour for real
	// third-party documents that pull in packages/classes gotex does not load —
	// an editor preview shows the typesettable content instead of one hard error.
	// The default (false) is strict: an undefined cs aborts, as TeX does.
	Lenient bool
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
// (unless NoPlain) and the text font family set. It is the starting point for
// programmatic use when callers want to Run source incrementally before
// rendering.
func NewDocument(opt Options) (*Engine, error) {
	return buildEngine(opt, false)
}

// buildEngine builds an engine with macros and the font family. Fonts are bound
// after the macro layers so \rm/\bf/\it win over the LaTeX kernel's no-op stubs.
func buildEngine(opt Options, latex bool) (*Engine, error) {
	e := New()
	if !opt.NoPlain {
		if err := e.LoadPlain(); err != nil {
			return nil, fmt.Errorf("texengine: loading macros: %w", err)
		}
	}
	if latex {
		if err := e.LoadLaTeX(); err != nil {
			return nil, fmt.Errorf("texengine: loading LaTeX: %w", err)
		}
	}
	fontBytes := opt.Font
	if fontBytes == nil {
		fontBytes = texmath.DefaultFont()
	}
	reg, err := NewOpenTypeFont(fontBytes, opt.size())
	if err != nil {
		return nil, fmt.Errorf("texengine: font: %w", err)
	}
	e.SetFont(reg)
	e.today = opt.Date
	e.lenient = opt.Lenient
	e.bindFont("rm", reg)
	if err := e.bindOptionalFont("bf", opt.BoldFont, opt.size()); err != nil {
		return nil, err
	}
	if err := e.bindOptionalFont("it", opt.ItalicFont, opt.size()); err != nil {
		return nil, err
	}
	if err := e.bindOptionalFont("tt", opt.MonoFont, opt.size()); err != nil {
		return nil, err
	}
	if err := e.bindOptionalFont("sf", opt.SansFont, opt.size()); err != nil {
		return nil, err
	}
	e.aliasFontSwitches()
	return e, nil
}

// aliasFontSwitches makes the NFSS declarations (\bfseries, \itshape, …) direct
// copies of the low-level font switches (\bf, \it, …) rather than macros that
// call them by name. LaTeX's \textbf/\textit/… go through the NFSS names, so a
// document that "modernises" a deprecated switch with \renewcommand{\bf}{\textbf}
// (common in older sources) must not turn \textbf into an infinite self-reference
// (\textbf→\bfseries→\bf→\textbf…), which otherwise consumes the whole document.
func (e *Engine) aliasFontSwitches() {
	for _, p := range [][2]string{
		{"bfseries", "bf"}, {"itshape", "it"}, {"slshape", "sl"},
		{"ttfamily", "tt"}, {"sffamily", "sf"}, {"rmfamily", "rm"},
		{"normalfont", "rm"}, {"upshape", "rm"}, {"mdseries", "rm"},
	} {
		if m := e.eq[p[1]]; m != nil {
			cp := *m
			cp.name = p[0]
			e.eq[p[0]] = &cp
		}
	}
}

// bindFont defines a control sequence that selects the given font.
func (e *Engine) bindFont(name string, f fontFace) {
	e.eq[name] = &meaning{kind: mFont, font: f, name: name}
}

func (e *Engine) bindOptionalFont(name string, data []byte, size int) error {
	if data == nil {
		return nil
	}
	f, err := NewOpenTypeFont(data, size)
	if err != nil {
		return fmt.Errorf("texengine: %s font: %w", name, err)
	}
	e.bindFont(name, f)
	return nil
}

// CompileToPDF processes TeX source and writes a PDF to w, returning the page
// count. A document's own \font/\hsize/… override the option defaults.
func CompileToPDF(src []byte, opt Options, w io.Writer) (int, error) {
	e, err := compile(src, opt)
	if err != nil {
		return 0, err
	}
	if err := e.RenderPDF(w, e.renderMargin(opt.margin())); err != nil {
		return 0, err
	}
	return len(e.Pages()), nil
}

// CompileToSVGPages processes TeX source and returns one SVG string per page —
// the form an editor preview pane consumes directly.
func CompileToSVGPages(src []byte, opt Options) ([]string, error) {
	e, err := compile(src, opt)
	if err != nil {
		return nil, err
	}
	return e.RenderPages(e.renderMargin(opt.margin())), nil
}

// CompileToSVGPagesReport is CompileToSVGPages that also returns the tally of
// undefined control sequences the (lenient) compile skipped — see SkippedCommands.
// It lets a caller surface the feature gaps a best-effort render would otherwise
// hide: an unimplemented command silently dropped can take a document's whole body
// with it (a class's frontmatter macro, \subfile, …) while the page still looks
// plausible. Ranked over a corpus, the tally points straight at what is worth
// implementing.
func CompileToSVGPagesReport(src []byte, opt Options) ([]string, map[string]int, error) {
	e, err := compile(src, opt)
	if err != nil {
		return nil, nil, err
	}
	return e.RenderPages(e.renderMargin(opt.margin())), e.SkippedCommands(), nil
}

// CompileToPDFReport is CompileToPDF that also returns the skipped-command tally
// (see CompileToSVGPagesReport / SkippedCommands).
func CompileToPDFReport(src []byte, opt Options, w io.Writer) (int, map[string]int, error) {
	e, err := compile(src, opt)
	if err != nil {
		return 0, nil, err
	}
	if err := e.RenderPDF(w, e.renderMargin(opt.margin())); err != nil {
		return 0, e.SkippedCommands(), err
	}
	return len(e.Pages()), e.SkippedCommands(), nil
}

// compile builds an engine and runs the source, ready for rendering. When the
// document defines cross-reference \labels it compiles twice — the first pass
// gathers the label table (so forward \refs resolve), exactly as LaTeX uses its
// .aux file — carrying the labels into the second, rendered pass.
func compile(src []byte, opt Options) (*Engine, error) {
	src = []byte(normalizeEOL(string(src))) // tolerate a CRLF document (e.g. from Windows)
	latex := isLaTeX(src)
	if latex && needsTwoPass(src) {
		aux, err := buildEngine(opt, true)
		if err != nil {
			return nil, err
		}
		if _, err := aux.Run(string(src)); err != nil {
			return nil, err
		}
		aux.finalizeTOCPages()
		aux.finalizeIndexPages()
		e, err := buildEngine(opt, true)
		if err != nil {
			return nil, err
		}
		e.labels = aux.labels
		e.refTypes = aux.refTypes
		e.refNames = aux.refNames
		e.bibAuthor = aux.bibAuthor // \citet author labels, gathered by the aux \bibliography
		e.tocSource = aux.tocEntries
		e.indexSource = aux.indexEntries
		if _, err := e.Run(string(src)); err != nil {
			return nil, err
		}
		return e, nil
	}
	e, err := buildEngine(opt, latex)
	if err != nil {
		return nil, err
	}
	if _, err := e.Run(string(src)); err != nil {
		return nil, err
	}
	return e, nil
}

// isLaTeX reports whether the source looks like a LaTeX document.
func isLaTeX(src []byte) bool {
	return bytes.Contains(src, []byte(`\documentclass`))
}
