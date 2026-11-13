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
// broken extractor cannot silently corrupt a measurement. A compile that merely
// exhausts -timeout is marked timed-out and excluded on EITHER side, for the same
// reason and with the same banner: a clock running out is not a fidelity result.
// Example:
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
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-tex/engine/internal/pdftext"
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
	gotexPath := fs.String("gotex", "", "a gotex binary to measure instead of building this checkout's — run it twice, once per side of a change, with the same -seed and -n")
	list := fs.String("list", "", "a file of paper paths, one per line, to sample from instead of walking -corpus (the same list gotex-abdiff takes)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *corpus == "" && *list == "" {
		fmt.Fprintln(stderr, "gotex-refdiff: one of -corpus or -list is required")
		fs.PrintDefaults()
		return 2
	}

	papers, err := choosePapers(*corpus, *list)
	if err != nil {
		fmt.Fprintf(stderr, "gotex-refdiff: %v\n", err)
		return 1
	}
	if len(papers) == 0 {
		where := *corpus
		if *list != "" {
			where = *list
		}
		fmt.Fprintf(stderr, "gotex-refdiff: no paper directories from %s\n", where)
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
	//
	// -gotex names a binary to use instead. That is what makes this an A/B
	// instrument rather than a snapshot: the two sides of a change are two builds,
	// and the sample is the same one as long as -seed and -n are. Without it the
	// only way to compare was to run the tool from two checkouts, which also
	// changes the sample when the corpus directory listing differs.
	gotexBin := *gotexPath
	if gotexBin == "" {
		gotexBin = filepath.Join(work, "gotex")
		if out, err := buildGotex(gotexBin); err != nil {
			fmt.Fprintf(stderr, "gotex-refdiff: building gotex failed: %v\n%s", err, out)
			return 1
		}
	} else if _, err := os.Stat(gotexBin); err != nil {
		fmt.Fprintf(stderr, "gotex-refdiff: -gotex %s: %v\n", gotexBin, err)
		return 1
	}
	fmt.Fprintf(stderr, "engine under test: %s\n", gotexBin)

	if *layout {
		pipe := realLayoutPipeline(gotexBin, *timeout)
		results := make([]layoutResult, 0, len(sample))
		for i, dir := range sample {
			id := filepath.Base(dir)
			fmt.Fprintf(stderr, "  [%d/%d] %s\n", i+1, len(sample), id)
			results = append(results, analyzeLayout(dir, id, filepath.Join(work, id), pipe))
		}
		warnExtractErrors(stderr, layoutDiagnostics(results))
		warnTimedOut(stderr, layoutTimeoutDiagnostics(results))
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
	warnTimedOut(stderr, timeoutDiagnostics(results))
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

// timeoutDiagnostics collects the "id: diag" lines of every paper whose compile
// ran out of the per-engine budget, in recall mode.
func timeoutDiagnostics(results []paperResult) []string {
	var out []string
	for _, r := range results {
		if r.status == statusTimedOut {
			out = append(out, r.id+": "+r.diag)
		}
	}
	return out
}

// layoutTimeoutDiagnostics is timeoutDiagnostics for layout mode.
func layoutTimeoutDiagnostics(results []layoutResult) []string {
	var out []string
	for _, r := range results {
		if r.status == statusTimedOut {
			out = append(out, r.id+": "+r.diag)
		}
	}
	return out
}

// warnTimedOut prints a loud, unmissable summary to stderr when any paper's
// compile ran out of its budget, so a mean resting on fewer papers than were
// sampled cannot be read as though it rested on all of them. It prints nothing
// when diags is empty.
func warnTimedOut(stderr io.Writer, diags []string) {
	if len(diags) == 0 {
		return
	}
	fmt.Fprintf(stderr, "gotex-refdiff: WARNING: %d paper(s) ran out of the per-engine compile budget and were EXCLUDED from the score — a clock, not a fidelity result. The mean below rests on fewer papers than were sampled; raise -timeout, or measure on a quieter machine:\n", len(diags))
	for _, d := range diags {
		fmt.Fprintf(stderr, "    %s\n", d)
	}
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
// diag carries a human-readable reason when status is statusExtractError or
// statusTimedOut.
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
	statusTimedOut     = "timed-out"       // a compile exhausted the per-engine budget — a clock, NOT a fidelity result
)

// errTimedOut marks a compile that the per-engine budget cut short, as opposed
// to one that ran to completion and produced nothing. Wrapped by runCmd and
// tested for with errors.Is, so an injected pipeline can report a timeout the
// same way the real one does.
//
// ⛔ The two are indistinguishable by their RESULT — neither leaves a PDF — and
// the sampler used to have no other evidence, because both call sites discarded
// the compile error. The difference matters because the sampler does not treat
// the two sides alike: an empty reference EXCLUDES the paper, an empty gotex
// SCORES it zero (see report). So a slow paper, or a loaded machine, moved the
// mean in one direction only. See #751.
var errTimedOut = errors.New("compile timed out")

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

// The extractor itself — which binary, how it is found, the poppler pin — now
// lives in internal/pdftext, so that gotex-worddiff reads a PDF through exactly
// the same path this scorer does. Two extractors would report their own
// difference in these units. What stays here is what this tool MAKES of the
// text: word recall, and the genuine-empty versus tool-error distinction below.
const popplerProject = pdftext.PopplerProject

func pkgxArgs(name string, toolArgs ...string) []string {
	return pdftext.PkgxArgs(name, toolArgs...)
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
	//
	// ⛔ THE CLOCK IS ASKED FIRST, ON BOTH SIDES. A compile that merely ran out
	// of its budget leaves no PDF, exactly like one that gave up, and the paper
	// is removed from the measurement either way it happened — not scored zero
	// on one side and dropped on the other. See [errTimedOut] and #751.
	if err := pipe.compileRef(tex, refPDF); errors.Is(err, errTimedOut) {
		res.status = statusTimedOut
		res.diag = "reference: " + err.Error()
		return res
	}
	refX := pipe.extract(refPDF)
	refWords := contentWords(refX.text)
	if s, diag := classifyExtraction(refX.toolErr, refX.nonTrivial, len(refWords), "reference", statusRefMissing); s != "" {
		res.status = s
		res.diag = diag
		return res
	}

	if err := pipe.compileGot(tex, gotPDF); errors.Is(err, errTimedOut) {
		res.status = statusTimedOut
		res.diag = "gotex: " + err.Error()
		res.refWords = len(refWords)
		return res
	}
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
		fmt.Fprintf(w, "\nnot scored (no ground truth, no top-level .tex, a failed extractor, or a compile that ran out of time):\n")
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

// captureRunner runs a text-producing tool (pdftotext) on a PDF and returns the
// captured text. It is internal/pdftext's, shared with gotex-worddiff.
func captureRunner(name string) func(timeout time.Duration, pdf string) (string, error) {
	return pdftext.CaptureRunner(name)
}

// runCmd runs a command with a timeout, discarding its output; only its
// success/failure — and, since #751, WHY it failed — matters to the sampler.
//
// CommandContext kills the child when the deadline passes, so cmd.Run reports a
// signal and nothing in that error says "the budget ran out". The context does
// know, so it is asked, and the answer is wrapped as [errTimedOut].
func runCmd(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("%s: %w after %s", name, errTimedOut, timeout)
	}
	return err
}

// replaceExt replaces the extension of name with ext (e.g. "a.tex" -> "a.pdf").
func replaceExt(name, ext string) string {
	return name[:len(name)-len(filepath.Ext(name))] + ext
}

// choosePapers decides which papers the sample is drawn from: a named list when
// one is given, otherwise every directory under the corpus root.
//
// ⛔ The list matters more than it looks. Walking a raw arXiv corpus draws papers
// the REFERENCE cannot compile — a paper whose preamble calls \pdfglyphtounicode
// stops XeTeX dead — and every one of those scores "ref-unavailable", which is
// not a measurement. A 30-paper layout run over the raw directory came back with
// 30 of 30 unavailable on BOTH sides of a change, which reads exactly like "no
// difference" and says nothing at all. The curated list is the set that compiles
// under both engines, and it is the same file gotex-abdiff is given, so the two
// instruments speak about the same population.
func choosePapers(corpus, list string) ([]string, error) {
	if list == "" {
		return discoverPapers(corpus)
	}
	b, err := os.ReadFile(list)
	if err != nil {
		return nil, err
	}
	var dirs []string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// The list names a paper's top-level .tex; the sampler works in directories.
		d := filepath.Clean(filepath.Dir(line))
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("%s names no paper", list)
	}
	return dirs, nil
}
