// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// This file holds the pure content-coverage analysis: pick a paper's top-level
// .tex, measure how much source body it carries (letters, comments and preamble
// excluded), and — paired with the glyph count a render produces — compute a
// coverage ratio that flags a document rendering far less than its source implies.
// A silent swallow (a runaway, an unclosed group, a dropped body macro) shows up
// here as an outlier low ratio even when no undefined command was reported.

// Result is one paper's coverage measurement.
type Result struct {
	ID          string  // arXiv id (the paper directory name)
	Class       string  // \documentclass, e.g. "article", "revtex4-2" ("" if none/unknown)
	Toplevel    string  // the top-level .tex file chosen
	Pages       int     // rendered SVG page count
	Glyphs      int     // total `<path` occurrences across all page SVGs (glyph count)
	BodyLetters int     // letters of source body (comments + preamble excluded)
	Coverage    float64 // glyphs per source-KB of body letters (Glyphs / (BodyLetters/1024))
	Runaway     bool    // Diagnostics.Runaway — an expansion/argument runaway tripped
	OpenGroups  int     // Diagnostics.OpenGroups — groups left open at end of document
	PageCapHit  bool    // Diagnostics.PageCapHit — pagination hit the maxPages backstop
	Undefined   int     // total count of undefined control sequences skipped (lenient)
	Err         string  // non-empty if the paper could not be analysed
}

// Silent reports whether this result is a silent-swallow suspect: a document with
// a genuine swallow signal — a runaway or a left-open group — since those lose body
// content without any undefined command being reported. (Low coverage alone is
// surfaced by the ranking; this marks the results a swallow flag corroborates.)
func (r Result) Silent() bool {
	return r.Err == "" && (r.Runaway || r.OpenGroups > 0 || r.PageCapHit)
}

// countGlyphs counts `<path` occurrences across all page SVGs — the correct glyph
// tally (each drawn glyph is one <path> element). It deliberately does not use a
// line count: the engine emits a whole page as a single line, so counting lines
// would report ~1 regardless of content.
func countGlyphs(pages []string) int {
	n := 0
	for _, p := range pages {
		n += strings.Count(p, "<path")
	}
	return n
}

// classRE extracts the class name from \documentclass[opts]{class}.
var classRE = regexp.MustCompile(`\\documentclass\s*(?:\[[^\]]*\])?\s*\{([^}]*)\}`)

// documentClass returns the \documentclass name in src, or "" if there is none.
func documentClass(src string) string {
	if m := classRE.FindStringSubmatch(src); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// stripLineComment removes a TeX end-of-line comment: an unescaped % starts a
// comment that runs to the end of the line. A % preceded by an odd number of
// backslashes is a literal \% and is kept.
func stripLineComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] != '%' {
			continue
		}
		bs := 0
		for j := i - 1; j >= 0 && line[j] == '\\'; j-- {
			bs++
		}
		if bs%2 == 0 { // even backslashes ⇒ the % is a real comment start
			return line[:i]
		}
	}
	return line
}

// stripComments removes every line comment from src.
func stripComments(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = stripLineComment(l)
	}
	return strings.Join(lines, "\n")
}

// countLetters counts Unicode letters in s.
func countLetters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// bodyRegion returns the document body of a .tex source: the text after
// \begin{document} (up to \end{document} if present). A file with no
// \begin{document} is an \input fragment — its whole content is body.
func bodyRegion(src string) string {
	const begin = `\begin{document}`
	i := strings.Index(src, begin)
	if i < 0 {
		return src
	}
	body := src[i+len(begin):]
	if j := strings.Index(body, `\end{document}`); j >= 0 {
		body = body[:j]
	}
	return body
}

// includeRE matches \input{file}, \input file, \include{file} and \subfile{file}
// — the ways a LaTeX document splices a body fragment in from another file.
var includeRE = regexp.MustCompile(`\\(?:input|include|subfile)\b\s*(?:\{([^}]*)\}|([^\s{}\\]+))`)

// includedFiles returns the file targets \input/\include/\subfile in src reference,
// in order.
func includedFiles(src string) []string {
	var out []string
	for _, m := range includeRE.FindAllStringSubmatch(src, -1) {
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// resolveTeX resolves an \input/\include name against dir, appending .tex when the
// name has no .tex extension. It returns the path and true if a readable file
// exists.
func resolveTeX(dir, name string) (string, bool) {
	cands := []string{name}
	if !strings.HasSuffix(name, ".tex") {
		cands = append([]string{name + ".tex"}, cands...)
	}
	for _, c := range cands {
		p := c
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, c)
		}
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, true
		}
	}
	return "", false
}

// bodyLetters measures a document's source body size: the letters of the top-level
// file's body plus, recursively, the letters of every file it \inputs/\includes —
// exactly the source that the render consumes. Comments and the preamble before
// \begin{document} are excluded. Following includes (rather than summing every
// stray .tex in the directory) keeps an independent supplement or leftover file
// from inflating the size and faking a low-coverage outlier.
func bodyLetters(dir, top string) int {
	visited := map[string]bool{}
	return gatherLetters(dir, top, visited)
}

func gatherLetters(dir, file string, visited map[string]bool) int {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, file)
	}
	abs = filepath.Clean(abs)
	if visited[abs] {
		return 0
	}
	visited[abs] = true
	raw, err := os.ReadFile(abs)
	if err != nil {
		return 0
	}
	src := stripComments(string(raw))
	region := bodyRegion(src)
	total := countLetters(region)
	for _, name := range includedFiles(region) {
		if p, ok := resolveTeX(dir, name); ok {
			total += gatherLetters(dir, p, visited)
		}
	}
	return total
}

// readme is the arXiv 00README.json shape we read: the source flagged
// "usage":"toplevel".
type readme struct {
	Sources []struct {
		Usage    string `json:"usage"`
		Filename string `json:"filename"`
	} `json:"sources"`
}

// toplevelFromReadme returns the filename 00README.json marks "usage":"toplevel",
// or "" if there is no readme or no such entry.
func toplevelFromReadme(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "00README.json"))
	if err != nil {
		return ""
	}
	var r readme
	if json.Unmarshal(data, &r) != nil {
		return ""
	}
	for _, s := range r.Sources {
		if s.Usage == "toplevel" && s.Filename != "" {
			return s.Filename
		}
	}
	return ""
}

// preferredNames are common top-level base names, tried before falling back to the
// largest candidate when several files carry a \begin{document}.
var preferredNames = []string{"main", "ms", "paper", "root", "manuscript", "article", "template"}

// pickToplevel chooses a paper's top-level .tex. It honours 00README.json's
// "toplevel" source when present and readable; otherwise it falls back to the .tex
// containing \begin{document}, preferring a conventional name (main/ms/paper/…) or
// the paper's own id, and finally the largest such file. It returns "" if the
// directory holds no usable top-level.
func pickToplevel(dir string) string {
	if name := toplevelFromReadme(dir); name != "" {
		if _, ok := resolveTeX(dir, name); ok {
			return name
		}
	}
	texs, _ := filepath.Glob(filepath.Join(dir, "*.tex"))
	var candidates []string
	for _, t := range texs {
		data, err := os.ReadFile(t)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), `\begin{document}`) {
			candidates = append(candidates, filepath.Base(t))
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	id := filepath.Base(dir)
	for _, pref := range append([]string{id}, preferredNames...) {
		for _, c := range candidates {
			if strings.EqualFold(strings.TrimSuffix(c, ".tex"), pref) {
				return c
			}
		}
	}
	// Fall back to the largest candidate (the manuscript, not a short front file).
	sort.Slice(candidates, func(i, j int) bool {
		return fileSize(filepath.Join(dir, candidates[i])) > fileSize(filepath.Join(dir, candidates[j]))
	})
	return candidates[0]
}

func fileSize(p string) int64 {
	if fi, err := os.Stat(p); err == nil {
		return fi.Size()
	}
	return 0
}

// median returns the median of xs (0 for an empty slice). xs is sorted in place.
func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sort.Float64s(xs)
	n := len(xs)
	if n%2 == 1 {
		return xs[n/2]
	}
	return (xs[n/2-1] + xs[n/2]) / 2
}

// coverageOf computes glyphs per source-KB (1024 body letters). It is 0 when there
// is no body to divide by.
func coverageOf(glyphs, bodyLetters int) float64 {
	if bodyLetters <= 0 {
		return 0
	}
	return float64(glyphs) / (float64(bodyLetters) / 1024)
}

// rankOutliers returns the results whose coverage is at or below frac×median,
// among those that rendered a non-empty body, sorted worst (lowest coverage)
// first. Papers that failed to render (Err set) or had no measurable body are
// excluded — they are reported separately.
func rankOutliers(results []Result, frac float64) (outliers []Result, med float64) {
	var covs []float64
	for _, r := range results {
		if r.Err == "" && r.BodyLetters > 0 {
			covs = append(covs, r.Coverage)
		}
	}
	med = median(covs)
	threshold := med * frac
	for _, r := range results {
		if r.Err == "" && r.BodyLetters > 0 && r.Coverage <= threshold {
			outliers = append(outliers, r)
		}
	}
	sort.SliceStable(outliers, func(i, j int) bool {
		return outliers[i].Coverage < outliers[j].Coverage
	})
	return outliers, med
}
