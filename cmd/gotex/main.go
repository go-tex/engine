// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex is a pure-Go TeX compiler: it processes a .tex document and
// writes a PDF (or SVG pages), a drop-in for pdftex/xetex in a loom-style
// preview/build pipeline. With no -font it uses a built-in font so it runs with
// no external assets; a document's own \font wins over the default.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	engine "github.com/go-tex/engine"
	texmath "github.com/go-tex/math"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable entry point: it returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output file (default: input with .pdf/.svg)")
	format := fs.String("format", "pdf", "output format: pdf or svg")
	fontPath := fs.String("font", "", "text font file (.ttf/.otf); default is built in")
	size := fs.Int("size", 10, "default text size in points")
	margin := fs.Float64("margin", 72, "page margin in points")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	input := fs.Arg(0)
	if input == "" {
		fmt.Fprintln(stderr, "usage: gotex [flags] file.tex")
		fs.PrintDefaults()
		return 2
	}
	if *format != "pdf" && *format != "svg" {
		fmt.Fprintf(stderr, "gotex: unknown -format %q (want pdf or svg)\n", *format)
		return 2
	}

	fontBytes, err := loadFont(*fontPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	e := engine.New()
	if err := e.LoadPlain(); err != nil {
		fmt.Fprintf(stderr, "gotex: loading macros: %v\n", err)
		return 1
	}
	f, err := engine.NewOpenTypeFont(fontBytes, *size)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: font: %v\n", err)
		return 1
	}
	e.SetFont(f)

	// Process the document by \input (its own \font/\hsize/… take effect).
	if _, err := e.Run(`\input ` + input + ` `); err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}

	outName := *out
	if outName == "" {
		outName = replaceExt(input, "."+*format)
	}
	if err := writeOutput(e, outName, *format, *margin); err != nil {
		fmt.Fprintf(stderr, "gotex: writing %s: %v\n", outName, err)
		return 1
	}
	pages := len(e.Pages())
	fmt.Fprintf(stdout, "gotex: wrote %s (%d page%s)\n", outName, pages, plural(pages))
	return 0
}

// loadFont returns the font bytes: the file at path, or the built-in default.
func loadFont(path string) ([]byte, error) {
	if path == "" {
		return texmath.DefaultFont(), nil // built-in (STIX Two Math, OFL)
	}
	return os.ReadFile(path)
}

// writeOutput emits the compiled document in the requested format.
func writeOutput(e *engine.Engine, name, format string, margin float64) error {
	if format == "svg" {
		pages := e.RenderPages(margin)
		if len(pages) <= 1 {
			return os.WriteFile(name, []byte(firstOr(pages, "")), 0644)
		}
		// Multiple pages: write name-1.svg, name-2.svg, …
		base := strings.TrimSuffix(name, ".svg")
		for i, svg := range pages {
			if err := os.WriteFile(fmt.Sprintf("%s-%d.svg", base, i+1), []byte(svg), 0644); err != nil {
				return err
			}
		}
		return nil
	}
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return e.RenderPDF(f, margin)
}

func replaceExt(name, ext string) string {
	return strings.TrimSuffix(name, filepath.Ext(name)) + ext
}

func firstOr(s []string, d string) string {
	if len(s) > 0 {
		return s[0]
	}
	return d
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
