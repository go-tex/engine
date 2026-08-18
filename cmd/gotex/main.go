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
	"sort"
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
	date := fs.String("date", "", "date text bound to \\today")
	lenient := fs.Bool("lenient", false, "skip undefined commands instead of aborting (best-effort preview of third-party documents)")
	reportSkipped := fs.Bool("report-skipped", false, "after a lenient render, print (to stderr) the undefined commands that were skipped, most frequent first — surfaces the feature gaps a best-effort render hides")
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
	// Resolve relative \input paths against the document's directory. Restore
	// the working directory before returning: os.Chdir mutates the process-wide
	// cwd, and leaving it inside a caller's temp dir makes that dir impossible
	// to remove on Windows ("The process cannot access the file because it is
	// being used by another process") when the caller (e.g. a test) cleans up.
	if dir := filepath.Dir(input); dir != "." {
		orig, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "gotex: %v\n", err)
			return 1
		}
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(stderr, "gotex: %v\n", err)
			return 1
		}
		defer os.Chdir(orig)
	}
	opt := engine.Options{Font: fontBytes, BoldFont: boldBytes, ItalicFont: italicBytes, MonoFont: monoBytes, SansFont: sansBytes, Size: *size, Margin: *margin, Date: *date, Lenient: *lenient}

	outName := *out
	if outName == "" {
		outName = replaceExt(filepath.Base(input), "."+*format)
		if *outdir != "" {
			outName = filepath.Join(*outdir, outName)
		}
	}
	pages, diag, err := writeOutput(src, outName, *format, opt)
	if err != nil {
		fmt.Fprintf(stderr, "gotex: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "gotex: wrote %s (%d page%s)\n", outName, pages, plural(pages))
	if *reportSkipped {
		reportDiagnostics(stderr, diag)
	}
	return 0
}

// reportDiagnostics prints what a lenient render may have quietly lost: the
// undefined commands it skipped (most frequent first — a high count is where
// material is being dropped) and the silent-swallow alarms (a runaway that tripped,
// groups left open at the end, a page-count explosion) that no skipped command
// reveals.
func reportDiagnostics(w io.Writer, d engine.Diagnostics) {
	if d.Runaway {
		fmt.Fprintln(w, "gotex: WARNING runaway guard tripped — a loop or exponential scan was aborted (content may be lost)")
	}
	if d.OpenGroups > 0 {
		fmt.Fprintf(w, "gotex: WARNING %d group(s) still open at end of document — an unbalanced { or \\begingroup (a likely swallow)\n", d.OpenGroups)
	}
	if d.PageCapHit {
		fmt.Fprintln(w, "gotex: WARNING pagination hit the page cap — a page-count explosion")
	}
	// Undefined environments never show up as skipped commands: \begin{env} routes
	// through \csname, which turns a missing \env into a silent \relax. Report them
	// on their own so an unimplemented environment (whose body was then typeset in
	// the wrong mode) is not mistaken for "nothing skipped".
	if len(d.UndefinedEnvs) > 0 {
		list := sortedCounts(d.UndefinedEnvs)
		fmt.Fprintf(w, "gotex: %d undefined environment(s) — \\begin{…} of an environment the engine lacks (most frequent first):\n", len(list))
		for _, e := range list {
			fmt.Fprintf(w, "  %6d  %s\n", e.count, e.name)
		}
	}
	if len(d.Skipped) == 0 {
		fmt.Fprintln(w, "gotex: no undefined commands skipped")
		return
	}
	list := sortedCounts(d.Skipped)
	fmt.Fprintf(w, "gotex: %d undefined command(s) skipped (most frequent first):\n", len(list))
	for _, e := range list {
		fmt.Fprintf(w, "  %6d  \\%s\n", e.count, e.name)
	}
}

// nameCount pairs a name with how many times it occurred, for sorted reporting.
type nameCount struct {
	name  string
	count int
}

// sortedCounts flattens a name→count map into a slice ordered by descending count,
// ties broken by name, so the report leads with where the most material was lost.
func sortedCounts(m map[string]int) []nameCount {
	list := make([]nameCount, 0, len(m))
	for name, count := range m {
		list = append(list, nameCount{name, count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].count != list[j].count {
			return list[i].count > list[j].count
		}
		return list[i].name < list[j].name
	})
	return list
}

// writeOutput compiles src and emits it in the requested format, returning the
// page count and the compile's Diagnostics.
func writeOutput(src []byte, name, format string, opt engine.Options) (int, engine.Diagnostics, error) {
	if format == "svg" {
		pages, diag, err := engine.CompileToSVGPagesDiag(src, opt)
		if err != nil {
			return 0, diag, err
		}
		if len(pages) <= 1 {
			return len(pages), diag, os.WriteFile(name, []byte(firstOr(pages, "")), 0644)
		}
		base := strings.TrimSuffix(name, ".svg")
		for i, svg := range pages {
			if err := os.WriteFile(fmt.Sprintf("%s-%d.svg", base, i+1), []byte(svg), 0644); err != nil {
				return 0, diag, err
			}
		}
		return len(pages), diag, nil
	}
	f, err := os.Create(name)
	if err != nil {
		return 0, engine.Diagnostics{}, err
	}
	// Close deterministically and surface a write error in preference to the
	// close error. Relying on a deferred Close alone would ignore its error and
	// (on Windows) could keep the handle open past the caller's temp-dir
	// cleanup; closing here releases the handle before writeOutput returns.
	pages, diag, err := engine.CompileToPDFDiag(src, opt, f)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return pages, diag, err
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
