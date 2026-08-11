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
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable entry point: it returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotex", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output file (default: input with .pdf/.svg)")
	outdir := fs.String("outdir", "", "output directory (latexmk-style; PDF named after the input)")
	format := fs.String("format", "pdf", "output format: pdf or svg")
	fs.Bool("pdf", false, "produce PDF (accepted for latexmk compatibility; PDF is the default)")
	fontPath := fs.String("font", "", "roman text font file (.ttf/.otf); default is built in")
	boldPath := fs.String("boldfont", "", "bold font file, bound to \\bf (so \\textbf bolds)")
	italicPath := fs.String("italicfont", "", "italic font file, bound to \\it (so \\emph slants)")
	monoPath := fs.String("monofont", "", "monospace font file, bound to \\tt (so \\texttt is fixed-width)")
	sansPath := fs.String("sansfont", "", "sans-serif font file, bound to \\sf (so \\textsf is sans)")
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

	fontBytes, err := readIf(*fontPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	boldBytes, err := readIf(*boldPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	italicBytes, err := readIf(*italicPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	monoBytes, err := readIf(*monoPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	sansBytes, err := readIf(*sansPath)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	src, err := os.ReadFile(input)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	// Resolve relative \input paths against the document's directory.
	if dir := filepath.Dir(input); dir != "." {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(stderr, "gotex: %v\n", err)
			return 1
		}
	}
	opt := engine.Options{Font: fontBytes, BoldFont: boldBytes, ItalicFont: italicBytes, MonoFont: monoBytes, SansFont: sansBytes, Size: *size, Margin: *margin}

	outName := *out
	if outName == "" {
		outName = replaceExt(filepath.Base(input), "."+*format)
		if *outdir != "" {
			outName = filepath.Join(*outdir, outName)
		}
	}
	pages, err := writeOutput(src, outName, *format, opt)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "gotex: wrote %s (%d page%s)\n", outName, pages, plural(pages))
	return 0
}

// writeOutput compiles src and emits it in the requested format, returning the
// page count.
func writeOutput(src []byte, name, format string, opt engine.Options) (int, error) {
	if format == "svg" {
		pages, err := engine.CompileToSVGPages(src, opt)
		if err != nil {
			return 0, err
		}
		if len(pages) <= 1 {
			return len(pages), os.WriteFile(name, []byte(firstOr(pages, "")), 0644)
		}
		base := strings.TrimSuffix(name, ".svg")
		for i, svg := range pages {
			if err := os.WriteFile(fmt.Sprintf("%s-%d.svg", base, i+1), []byte(svg), 0644); err != nil {
				return 0, err
			}
		}
		return len(pages), nil
	}
	f, err := os.Create(name)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return engine.CompileToPDF(src, opt, f)
}

// readIf reads a file, returning nil bytes (no error) when the path is empty.
func readIf(path string) ([]byte, error) {
	if path == "" {
		return nil, nil
	}
	return os.ReadFile(path)
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
