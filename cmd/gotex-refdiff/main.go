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
// pdftotext are taken from PATH when present, else from pkgx (with the poppler
// project pinned so `pkgx pdftotext` does not stall on MultipleProjects). A real
// PDF that the extractor fails to read is marked extract-error and excluded from
// the score — never scored as lost content — and surfaced loudly on stderr, so a
// broken extractor cannot silently corrupt a measurement. Example:
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
		warnExtractErrors(stderr, layoutDiagnostics(results))
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
	warnExtractErrors(stderr, paperDiagnostics(results))
	report(results, stdout)
	return 0
}

// paperDiagnostics collects the "id: diag" lines of every extract-error paper in
// recall mode, so a silently-failed extraction is surfaced loudly rather than
// buried in the report.
func paperDiagnostics(results []paperResult) []string {
	var out []string
	for _, r := range results {
		if r.status == statusExtractError {
			out = append(out, r.id+": "+r.diag)
		}
	}
	return out
}

// layoutDiagnostics is paperDiagnostics for layout mode.
func layoutDiagnostics(results []layoutResult) []string {
	var out []string
	for _, r := range results {
		if r.status == statusExtractError {
			out = append(out, r.id+": "+r.diag)
		}
	}
	return out
}

// warnExtractErrors prints a loud, unmissable summary to stderr when any paper
// hit an extract-error, so a broken text extractor cannot corrupt a run in
// silence. It prints nothing when diags is empty.
func warnExtractErrors(stderr io.Writer, diags []string) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintf(stderr, "gotex-refdiff: WARNING: %d paper(s) hit extract-error and were EXCLUDED from the score — the text extractor failed on a real PDF (check pdftotext):\n", len(diags))
	for _, d := range diags {
		fmt.Fprintf(stderr, "    %s\n", d)
	}
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
// diag carries a human-readable reason when status is statusExtractError.
type paperResult struct {
	id       string
	class    string
	status   string
	ratio    float64
	refWords int
	common   int
	diag     string
}

const (
	statusOK           = "ok"              // both engines produced text; ratio is meaningful
	statusNoToplevel   = "no-toplevel"     // could not resolve a .tex to compile
	statusRefMissing   = "ref-unavailable" // tectonic produced no text — nothing to score against
	statusGotexFailed  = "gotex-failed"    // gotex produced no text — it lost everything (ratio 0)
	statusExtractError = "extract-error"   // a real PDF compiled but the text extractor failed on it — NOT a scorable result
)

// nonTrivialPDFBytes is the size above which a compiled PDF is taken to carry
// real content. A genuine multi-page arXiv PDF is tens to hundreds of KB; a
// failed or empty compile is a few hundred bytes to ~1 KB. When a PDF this large
// extracts to zero content words the extractor — not the compile — is at fault,
// so the paper is marked statusExtractError and excluded from the score rather
// than masquerading as a total content loss (recall 0 / total layout divergence).
const nonTrivialPDFBytes = 4096

// extraction is one PDF's text-extraction outcome, carrying enough context to
// tell a genuine empty result (no PDF, or a failed compile) apart from a SILENT
// extractor failure (a real PDF the tool could not read). text is the extracted
// text; nonTrivial is set when the source PDF was large enough to hold real
// content; toolErr is set when the extractor errored on a present PDF.
type extraction struct {
	text       string
	nonTrivial bool
	toolErr    bool
}

// classifyExtraction decides, for an extraction that yielded tokens content
// tokens, whether scoring can proceed and — when it cannot — why. It returns an
// empty status when the text is usable (tokens > 0). Otherwise an empty
// extraction is split by cause: a tool error, or a non-trivial PDF that extracted
// to nothing, is a SILENT extractor failure (statusExtractError, excluded from
// the score) with a human diagnostic; a genuinely absent or tiny PDF is the
// caller's genuineEmpty status (statusRefMissing / statusGotexFailed), a real
// compile result. side ("reference"/"gotex") names the affected engine in the
// diagnostic.
func classifyExtraction(toolErr, nonTrivial bool, tokens int, side, genuineEmpty string) (status, diag string) {
	if tokens > 0 {
		return "", "" // usable text
	}
	switch {
	case toolErr:
		return statusExtractError, fmt.Sprintf("%s: the text extractor failed on a present PDF (a tool error, e.g. a missing or ambiguous pdftotext) — extraction produced no text", side)
	case nonTrivial:
		return statusExtractError, fmt.Sprintf("%s: a non-trivial PDF (>=%d bytes) extracted to zero content words — a silent extractor failure, not a real content loss", side, nonTrivialPDFBytes)
	default:
		return genuineEmpty, "" // no content PDF to read: a real compile result
	}
}

// popplerProject is the pkgx project id that unambiguously provides poppler's
// command-line tools. On some machines a bare `pkgx pdftotext` fails with
// MultipleProjects (ambiguous between poppler.freedesktop.org and
// freedesktop.org/poppler-qt5); pinning the project resolves it so the pkgx
// fallback runs the extractor instead of silently producing no text.
const popplerProject = "poppler.freedesktop.org"

// pkgxArgs builds the argument list after `pkgx` to run tool name with toolArgs,
// pinning the poppler project for poppler's ambiguous tools (pdftotext,
// pdftoppm) so pkgx resolves a single package rather than erroring. Other tools
// are passed through unpinned.
func pkgxArgs(name string, toolArgs ...string) []string {
	args := make([]string, 0, len(toolArgs)+2)
	if name == "pdftotext" || name == "pdftoppm" {
		args = append(args, "+"+popplerProject)
	}
	args = append(args, name)
	return append(args, toolArgs...)
}

// pipeline is the set of external actions analyzePaper needs, injected so the
// per-paper logic is testable without tectonic, gotex or a network.
type pipeline struct {
	// compileRef and compileGot compile texPath to outPDF (reference / gotex).
	compileRef func(texPath, outPDF string) error
	compileGot func(texPath, outPDF string) error
	// extract returns a PDF's text extraction, distinguishing a genuine empty
	// result from a silent extractor failure (see extraction).
	extract func(pdf string) extraction
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
	// score, so gotex's own success or failure is not reported as a pass. An
	// empty extraction from a real reference PDF is a tool failure, not a missing
	// ground truth — classifyExtraction keeps the two apart so a broken extractor
	// cannot silently corrupt the measurement.
	_ = pipe.compileRef(tex, refPDF)
	refX := pipe.extract(refPDF)
	refWords := contentWords(refX.text)
	if s, diag := classifyExtraction(refX.toolErr, refX.nonTrivial, len(refWords), "reference", statusRefMissing); s != "" {
		res.status = s
		res.diag = diag
		return res
	}

	_ = pipe.compileGot(tex, gotPDF)
	gotX := pipe.extract(gotPDF)
	gotWords := contentWords(gotX.text)
	if s, diag := classifyExtraction(gotX.toolErr, gotX.nonTrivial, len(gotWords), "gotex", statusGotexFailed); s != "" {
		res.status = s
		res.diag = diag
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
		fmt.Fprintf(w, "\nnot scored (no ground truth, no top-level .tex, or a failed extractor):\n")
		for _, r := range unscored {
			fmt.Fprintf(w, "  %-12s %-16s %s%s\n", r.id, truncate(r.class, 16), r.status, diagSuffix(r.diag))
		}
	}
}

// diagSuffix renders a status's diagnostic as a trailing "  — <diag>" when one
// is present, and nothing otherwise, so extract-error rows explain themselves.
func diagSuffix(diag string) string {
	if diag == "" {
		return ""
	}
	return "  — " + diag
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
		extract: func(pdf string) extraction {
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

// extractText returns a PDF's text extraction via the given capture runner. An
// absent or zero-byte PDF is a genuine empty result (a failed compile). A present
// PDF on which the runner ERRORS is a tool failure (toolErr) — not "no text" —
// so a broken extractor is not mistaken for lost content. nonTrivial records
// whether the source PDF was large enough to hold real content, so the caller
// can tell a legitimately empty tiny PDF from a big one the tool failed to read.
func extractText(run func(time.Duration, string) (string, error), timeout time.Duration, pdf string) extraction {
	fi, err := os.Stat(pdf)
	if err != nil || fi.Size() == 0 {
		return extraction{}
	}
	nonTrivial := fi.Size() >= nonTrivialPDFBytes
	out, err := run(timeout, pdf)
	if err != nil {
		return extraction{nonTrivial: nonTrivial, toolErr: true}
	}
	return extraction{text: out, nonTrivial: nonTrivial}
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
		return runCmd(timeout, "pkgx", pkgxArgs(name, args...)...)
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
			cmd = exec.CommandContext(ctx, "pkgx", pkgxArgs(name, pdf, "-")...)
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
