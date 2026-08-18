// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex-coverage is a corpus content-coverage detector: it renders a
// directory of real LaTeX papers through the pure-Go engine and flags the SILENT
// swallows — papers that typeset far less than their source implies. Undefined-
// command reporting cannot see this failure mode: a runaway, an unclosed group, or
// a dropped body macro loses content while the page still looks plausible and no
// unknown \cs is reported.
//
// For each paper it picks the top-level .tex (00README.json's "toplevel" source,
// else the .tex carrying \begin{document}), measures the source body size (letters,
// with % comments and the preamble before \begin{document} excluded, following
// \input/\include), renders it, counts the glyphs actually drawn (the number of
// `<path` elements across all page SVGs), and computes a coverage ratio of glyphs
// per source-KB. It then reports the low-coverage outliers — papers whose ratio is
// far below the corpus median — worst first, surfacing the Diagnostics swallow
// flags (Runaway / OpenGroups / PageCapHit) and the undefined-command count so a
// true silent swallow (low coverage, NO undefined command) stands out.
//
// Usage:
//
//	gotex-coverage [flags] <corpus-root>
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	engine "github.com/go-tex/engine"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable entry point; it returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotex-coverage", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("n", 300, "sample at most this many papers (0 = all)")
	top := fs.Int("top", 20, "how many worst suspects to list")
	frac := fs.Float64("frac", 0.4, "outlier cutoff: coverage at or below frac×median is flagged")
	timeout := fs.Duration("timeout", 60*time.Second, "per-paper render timeout")
	silentOnly := fs.Bool("silent-only", false, "list only suspects with a swallow flag (Runaway/OpenGroups/PageCap) and NO undefined command")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root := fs.Arg(0)
	if root == "" {
		fmt.Fprintln(stderr, "usage: gotex-coverage [flags] <corpus-root>")
		fs.PrintDefaults()
		return 2
	}

	dirs, err := paperDirs(root, *limit)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-coverage: %v\n", err)
		return 1
	}
	if len(dirs) == 0 {
		fmt.Fprintf(stderr, "gotex-coverage: no paper directories under %s\n", root)
		return 1
	}

	render := renderWithTimeout(*timeout, renderPaper)
	results := make([]Result, 0, len(dirs))
	for i, d := range dirs {
		r := analyzePaper(d, render)
		results = append(results, r)
		fmt.Fprintf(stderr, "\r[%d/%d] %s          ", i+1, len(dirs), r.ID)
	}
	fmt.Fprintln(stderr)

	report(stdout, results, *frac, *top, *silentOnly)
	return 0
}

// paperDirs returns up to limit immediate sub-directories of root that hold at
// least one .tex file, sorted by name. A limit of 0 means all.
func paperDirs(root string, limit int) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		d := filepath.Join(root, e.Name())
		if texs, _ := filepath.Glob(filepath.Join(d, "*.tex")); len(texs) > 0 {
			dirs = append(dirs, d)
		}
	}
	sort.Strings(dirs)
	if limit > 0 && len(dirs) > limit {
		dirs = dirs[:limit]
	}
	return dirs, nil
}

// renderFunc renders a paper's top-level file (found in dir) and returns its SVG
// pages and Diagnostics.
type renderFunc func(dir, top string) (pages []string, diag engine.Diagnostics, err error)

// analyzePaper measures one paper: it picks the top-level .tex, sizes the source
// body, renders with render, and assembles the coverage Result. render is injected
// so the measurement logic is testable without the engine.
func analyzePaper(dir string, render renderFunc) Result {
	id := filepath.Base(dir)
	top := pickToplevel(dir)
	if top == "" {
		return Result{ID: id, Err: "no top-level .tex (none with \\begin{document})"}
	}
	r := Result{ID: id, Toplevel: top}
	if data, err := os.ReadFile(filepath.Join(dir, top)); err == nil {
		r.Class = documentClass(string(data))
	}
	r.BodyLetters = bodyLetters(dir, top)
	pages, diag, err := render(dir, top)
	if err != nil {
		r.Err = err.Error()
		return r
	}
	r.Pages = len(pages)
	r.Glyphs = countGlyphs(pages)
	r.Runaway = diag.Runaway
	r.OpenGroups = diag.OpenGroups
	r.PageCapHit = diag.PageCapHit
	for _, n := range diag.Skipped {
		r.Undefined += n
	}
	r.Coverage = coverageOf(r.Glyphs, r.BodyLetters)
	return r
}

// renderWithTimeout wraps inner so a render exceeding d is abandoned. The engine's
// own runaway and page-cap guards bound normal renders; the timeout is a backstop
// for a pathological document. Renders run one at a time (the engine resolves
// \input against the process working directory, which renderPaper sets), so a rare
// abandoned render's leaked goroutine cannot corrupt another paper's result.
func renderWithTimeout(d time.Duration, inner renderFunc) renderFunc {
	return func(dir, top string) (pages []string, diag engine.Diagnostics, err error) {
		type out struct {
			pages []string
			diag  engine.Diagnostics
			err   error
		}
		ch := make(chan out, 1)
		go func() {
			p, dg, e := inner(dir, top)
			ch <- out{p, dg, e}
		}()
		select {
		case o := <-ch:
			return o.pages, o.diag, o.err
		case <-time.After(d):
			return nil, engine.Diagnostics{}, fmt.Errorf("render timed out after %s", d)
		}
	}
}

// renderPaper reads dir/top and compiles it to SVG pages in lenient mode. It
// chdirs into dir so the engine resolves the document's \input/\include files, and
// recovers a panic into an error so one bad paper cannot abort a corpus sweep.
func renderPaper(dir, top string) (pages []string, diag engine.Diagnostics, err error) {
	defer func() {
		if r := recover(); r != nil {
			pages, diag, err = nil, engine.Diagnostics{}, fmt.Errorf("panic: %v", r)
		}
	}()
	cwd, err := os.Getwd()
	if err != nil {
		return nil, engine.Diagnostics{}, err
	}
	if err := os.Chdir(dir); err != nil {
		return nil, engine.Diagnostics{}, err
	}
	defer os.Chdir(cwd)
	src, err := os.ReadFile(top)
	if err != nil {
		return nil, engine.Diagnostics{}, err
	}
	pages, diag, err = engine.CompileToSVGPagesDiag(src, engine.Options{Lenient: true})
	return pages, diag, err
}

// report prints the coverage summary and the ranked silent-swallow suspects.
func report(w io.Writer, results []Result, frac float64, top int, silentOnly bool) {
	outliers, med := rankOutliers(results, frac)

	var failed, rendered int
	for _, r := range results {
		if r.Err != "" {
			failed++
		} else {
			rendered++
		}
	}

	fmt.Fprintf(w, "gotex-coverage: %d papers (%d rendered, %d failed)\n", len(results), rendered, failed)
	fmt.Fprintf(w, "median coverage: %.1f glyphs/source-KB   outlier cutoff (<=%.2f×median): %.1f\n",
		med, frac, med*frac)
	fmt.Fprintf(w, "low-coverage outliers: %d\n\n", len(outliers))

	shown := outliers
	if silentOnly {
		shown = shown[:0]
		for _, r := range outliers {
			// A silent swallow is content loss that undefined-command reporting
			// cannot account for: either a swallow flag is set (a runaway / an
			// unclosed group / a page-cap hit lost body with no unknown \cs), or
			// no undefined command was reported at all — so the missing content is
			// not explained by a feature gap gotex already knows about.
			if r.Silent() || r.Undefined == 0 {
				shown = append(shown, r)
			}
		}
		fmt.Fprintf(w, "SILENT-SWALLOW SUSPECTS (swallow flag set, or NO undefined command): %d\n\n", len(shown))
	}
	if len(shown) > top {
		shown = shown[:top]
	}

	fmt.Fprintf(w, "%-14s %-14s %6s %8s %8s %5s  %s\n",
		"id", "class", "cov", "glyphs", "bodyKB", "undef", "flags")
	for _, r := range shown {
		fmt.Fprintf(w, "%-14s %-14s %6.1f %8d %8.1f %5d  %s\n",
			r.ID, truncate(r.Class, 14), r.Coverage, r.Glyphs,
			float64(r.BodyLetters)/1024, r.Undefined, flagString(r))
	}

	if failed > 0 {
		fmt.Fprintf(w, "\nfailed to render (%d):\n", failed)
		n := 0
		for _, r := range results {
			if r.Err == "" {
				continue
			}
			fmt.Fprintf(w, "  %-14s %s\n", r.ID, r.Err)
			if n++; n >= top {
				fmt.Fprintf(w, "  … and %d more\n", failed-n)
				break
			}
		}
	}
}

// flagString renders the swallow flags set on a result ("-" when clear).
func flagString(r Result) string {
	var f []string
	if r.Runaway {
		f = append(f, "RUNAWAY")
	}
	if r.OpenGroups > 0 {
		f = append(f, fmt.Sprintf("OPENGROUPS=%d", r.OpenGroups))
	}
	if r.PageCapHit {
		f = append(f, "PAGECAP")
	}
	if len(f) == 0 {
		return "-"
	}
	s := f[0]
	for _, x := range f[1:] {
		s += "," + x
	}
	return s
}

// truncate shortens s to n runes, appending an ellipsis when it was cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
