// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex-refdiff is a reference-diff corpus sampler: the highest-signal
// fidelity detector for gotex. Over a random SAMPLE of a directory of real
// arXiv papers it compiles each paper twice — once with a real LaTeX engine
// (tectonic, the ground truth) and once with gotex — extracts the text of both
// PDFs with pdftotext, and reports a per-paper word-recall ratio: the fraction
// of the reference's extractable content words that gotex reproduced. The report
// is ranked worst-first, so the papers where gotex loses the most REAL content
// against a ground-truth engine surface directly as fix targets.
//
// It is a developer tool, not a CI gate: it needs network (tectonic downloads
// its package bundle on first use) and poppler's pdftotext. tectonic and
// pdftotext are taken from PATH when present, else from pkgx. Example:
//
//	GOWORK=off go run ./cmd/gotex-refdiff -corpus /path/to/arxiv/work -n 40
//
// The recall ratio is robust to the three known-expected engine differences
// documented in scripts/fidelity.sh (page numbers, math rendered as vector
// paths, and the fi/fl ligature ToUnicode gap); see contentWords in analyze.go.
//
// The -layout flag switches to the geometric complement of the recall measure.
// Recall says WHAT text gotex reproduced; the layout diff says WHERE it landed.
// It reuses the same corpus sampler, top-level resolution and compile wiring, but
// extracts per-word bounding boxes with `pdftotext -bbox`, aligns the two word
// streams by content, and ranks papers worst-first by a layout-divergence score
// combining matched-word displacement, page-count agreement, and line-break
// agreement. It masks the same three known differences, so the score reflects
// REAL layout divergence. See layout.go. Example:
//
//	GOWORK=off go run ./cmd/gotex-refdiff -corpus /path/to/arxiv/work -n 40 -layout
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is the testable entry point; it returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gotex-refdiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	corpus := fs.String("corpus", "", "directory of arXiv paper sub-directories (required)")
	n := fs.Int("n", 40, "number of papers to sample")
	seed := fs.Int64("seed", 1, "random seed for the sample (reproducible)")
	timeout := fs.Duration("timeout", 90*time.Second, "per-engine compile timeout for one paper")
	layout := fs.Bool("layout", false, "geometric layout-diff mode: rank papers by how far matched words drift plus page-count and line-break divergence (needs pdftotext -bbox), instead of the default word-recall")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpus == "" {
		fmt.Fprintln(stderr, "gotex-refdiff: -corpus is required")
		fs.PrintDefaults()
		return 2
	}

	papers, err := discoverPapers(*corpus)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-refdiff: %v\n", err)
		return 1
	}
	if len(papers) == 0 {
		fmt.Fprintf(stderr, "gotex-refdiff: no paper directories under %s\n", *corpus)
		return 1
	}
	sample := samplePapers(papers, *n, *seed)
	fmt.Fprintf(stderr, "gotex-refdiff: sampling %d of %d papers (seed %d)…\n", len(sample), len(papers), *seed)

	work, err := os.MkdirTemp("", "gotex-refdiff-")
	if err != nil {
		fmt.Fprintf(stderr, "gotex-refdiff: %v\n", err)
		return 1
	}
	defer os.RemoveAll(work)

	// Build gotex once into the work dir and drive it as a subprocess: a real
	// arXiv paper can make the engine fail hard, and a subprocess isolates that
	// from the sampler (an in-process call would take the whole run down).
	gotexBin := filepath.Join(work, "gotex")
	if out, err := buildGotex(gotexBin); err != nil {
		fmt.Fprintf(stderr, "gotex-refdiff: building gotex failed: %v\n%s", err, out)
		return 1
	}

	if *layout {
		pipe := realLayoutPipeline(gotexBin, *timeout)
		results := make([]layoutResult, 0, len(sample))
		for i, dir := range sample {
			id := filepath.Base(dir)
			fmt.Fprintf(stderr, "  [%d/%d] %s\n", i+1, len(sample), id)
			results = append(results, analyzeLayout(dir, id, filepath.Join(work, id), pipe))
		}
		reportLayout(results, stdout)
		return 0
	}

	pipe := realPipeline(gotexBin, *timeout)
	results := make([]paperResult, 0, len(sample))
	for i, dir := range sample {
		id := filepath.Base(dir)
		fmt.Fprintf(stderr, "  [%d/%d] %s\n", i+1, len(sample), id)
		results = append(results, analyzePaper(dir, id, filepath.Join(work, id), pipe))
	}
	report(results, stdout)
	return 0
}

// discoverPapers lists the immediate sub-directories of the corpus root; each is
// one paper (named by its arXiv id).
func discoverPapers(corpus string) ([]string, error) {
	entries, err := os.ReadDir(corpus)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(corpus, e.Name()))
		}
	}
	sort.Strings(dirs)
	return dirs, nil
}

// samplePapers picks n papers deterministically from dirs using seed. When n is
// zero or negative, or at least len(dirs), every paper is returned (still
// shuffled for a representative processing order).
func samplePapers(dirs []string, n int, seed int64) []string {
	shuffled := make([]string, len(dirs))
	copy(shuffled, dirs)
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	if n > 0 && n < len(shuffled) {
		shuffled = shuffled[:n]
	}
	return shuffled
}

// paperResult is one paper's outcome. status is one of the statusXxx constants.
type paperResult struct {
	id       string
	class    string
	status   string
	ratio    float64
	refWords int
	common   int
}

const (
	statusOK          = "ok"              // both engines produced text; ratio is meaningful
	statusNoToplevel  = "no-toplevel"     // could not resolve a .tex to compile
	statusRefMissing  = "ref-unavailable" // tectonic produced no text — nothing to score against
	statusGotexFailed = "gotex-failed"    // gotex produced no text — it lost everything (ratio 0)
)

// pipeline is the set of external actions analyzePaper needs, injected so the
// per-paper logic is testable without tectonic, gotex or a network.
type pipeline struct {
	// compileRef and compileGot compile texPath to outPDF (reference / gotex).
	compileRef func(texPath, outPDF string) error
	compileGot func(texPath, outPDF string) error
	// extract returns the text of a PDF (empty string when it has none).
	extract func(pdf string) string
}

// analyzePaper runs the full per-paper pipeline: resolve the top-level .tex,
// compile it with both engines, extract and normalise both texts, and score the
// word recall. workDir is a scratch directory unique to this paper.
func analyzePaper(dir, id, workDir string, pipe pipeline) paperResult {
	res := paperResult{id: id, class: "?"}
	tex, err := resolveToplevel(dir)
	if err != nil {
		res.status = statusNoToplevel
		return res
	}
	if b, err := os.ReadFile(tex); err == nil {
		res.class = documentClass(string(b))
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		res.status = statusNoToplevel
		return res
	}
	refPDF := filepath.Join(workDir, "ref.pdf")
	gotPDF := filepath.Join(workDir, "got.pdf")

	// Reference first: if the ground truth is unavailable there is nothing to
	// score, so gotex's own success or failure is not reported as a pass.
	_ = pipe.compileRef(tex, refPDF)
	refWords := contentWords(pipe.extract(refPDF))
	if len(refWords) == 0 {
		res.status = statusRefMissing
		return res
	}

	_ = pipe.compileGot(tex, gotPDF)
	gotWords := contentWords(pipe.extract(gotPDF))
	if len(gotWords) == 0 {
		res.status = statusGotexFailed
		res.refWords = len(refWords)
		return res
	}
	res.ratio, res.refWords, res.common = recall(refWords, gotWords)
	res.status = statusOK
	return res
}

// report prints the results ranked worst-fidelity first. Scored papers (ok or
// gotex-failed) are ranked by ratio ascending; papers with no reference to
// score against are listed afterwards, so the ranking is grounded in real
// content loss.
func report(results []paperResult, w io.Writer) {
	var scored, unscored []paperResult
	for _, r := range results {
		if r.status == statusOK || r.status == statusGotexFailed {
			scored = append(scored, r)
		} else {
			unscored = append(unscored, r)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].ratio != scored[j].ratio {
			return scored[i].ratio < scored[j].ratio
		}
		return scored[i].id < scored[j].id
	})

	fmt.Fprintf(w, "reference-diff fidelity — %d paper(s) scored, worst first\n\n", len(scored))
	fmt.Fprintf(w, "%-12s %-16s %8s %8s %8s   %s\n", "id", "class", "ratio", "ref", "common", "status")
	var sum float64
	for _, r := range scored {
		fmt.Fprintf(w, "%-12s %-16s %7.1f%% %8d %8d   %s\n",
			r.id, truncate(r.class, 16), r.ratio*100, r.refWords, r.common, r.status)
		sum += r.ratio
	}
	if len(scored) > 0 {
		fmt.Fprintf(w, "\nmean recall over scored papers: %.1f%%\n", sum/float64(len(scored))*100)
	}
	if len(unscored) > 0 {
		fmt.Fprintf(w, "\nnot scored (no ground truth or no top-level .tex):\n")
		for _, r := range unscored {
			fmt.Fprintf(w, "  %-12s %-16s %s\n", r.id, truncate(r.class, 16), r.status)
		}
	}
}

// truncate shortens s to at most n runes, appending an ellipsis marker when cut.
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

// buildGotex compiles cmd/gotex to bin, returning any combined tool output on
// failure. GOWORK=off matches how the repo's own lanes build.
func buildGotex(bin string) ([]byte, error) {
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/gotex")
	cmd.Env = append(os.Environ(), "GOWORK=off")
	return cmd.CombinedOutput()
}

// realPipeline wires the external binaries. tectonic and pdftotext are taken
// from PATH when present, else run through pkgx (mirroring scripts/fidelity.sh).
func realPipeline(gotexBin string, timeout time.Duration) pipeline {
	tectonic := toolRunner("tectonic")
	pdftotext := captureRunner("pdftotext")
	return pipeline{
		compileRef: func(texPath, outPDF string) error {
			return runTectonic(tectonic, timeout, texPath, outPDF)
		},
		compileGot: func(texPath, outPDF string) error {
			return runCmd(timeout, gotexBin, "-lenient", "-o", outPDF, texPath)
		},
		extract: func(pdf string) string {
			return extractText(pdftotext, timeout, pdf)
		},
	}
}

// runTectonic compiles texPath to outPDF via the given runner. tectonic writes
// <base>.pdf into the output directory; when that produced name differs from the
// requested outPDF it is moved into place.
func runTectonic(run func(time.Duration, ...string) error, timeout time.Duration, texPath, outPDF string) error {
	outDir := filepath.Dir(outPDF)
	if err := run(timeout, "-o", outDir, "--outfmt", "pdf", texPath); err != nil {
		return err
	}
	produced := filepath.Join(outDir, replaceExt(filepath.Base(texPath), ".pdf"))
	if produced != outPDF {
		return os.Rename(produced, outPDF)
	}
	return nil
}

// extractText returns a PDF's text via the given capture runner, or "" when the
// file is absent, empty, or the extractor fails — the sampler treats all three
// the same (no comparable text).
func extractText(run func(time.Duration, string) (string, error), timeout time.Duration, pdf string) string {
	if fi, err := os.Stat(pdf); err != nil || fi.Size() == 0 {
		return ""
	}
	out, err := run(timeout, pdf)
	if err != nil {
		return ""
	}
	return out
}

// toolRunner returns a function that runs a tool with args, preferring a binary
// on PATH and falling back to `pkgx <tool>`. The decision is made ONCE (not per
// call) so a genuinely failing tool is not silently retried through pkgx —
// mirroring the reasoning in scripts/fidelity.sh.
func toolRunner(name string) func(timeout time.Duration, args ...string) error {
	if _, err := exec.LookPath(name); err == nil {
		return func(timeout time.Duration, args ...string) error {
			return runCmd(timeout, name, args...)
		}
	}
	return func(timeout time.Duration, args ...string) error {
		return runCmd(timeout, "pkgx", append([]string{name}, args...)...)
	}
}

// captureRunner returns a function that runs a text-producing tool (pdftotext)
// on a PDF, writing to stdout ("-"), and returns the captured text. Like
// toolRunner it decides PATH-vs-pkgx once.
func captureRunner(name string) func(timeout time.Duration, pdf string) (string, error) {
	usePkgx := false
	if _, err := exec.LookPath(name); err != nil {
		usePkgx = true
	}
	return func(timeout time.Duration, pdf string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var cmd *exec.Cmd
		if usePkgx {
			cmd = exec.CommandContext(ctx, "pkgx", name, pdf, "-")
		} else {
			cmd = exec.CommandContext(ctx, name, pdf, "-")
		}
		out, err := cmd.Output()
		return string(out), err
	}
}

// runCmd runs a command with a timeout, discarding its output; only its
// success/failure matters to the sampler.
func runCmd(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Run()
}

// replaceExt replaces the extension of name with ext (e.g. "a.tex" -> "a.pdf").
func replaceExt(name, ext string) string {
	return name[:len(name)-len(filepath.Ext(name))] + ext
}
