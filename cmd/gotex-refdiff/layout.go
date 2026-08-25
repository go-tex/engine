// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

// This file implements the -layout mode: the geometric complement to the
// word-recall fidelity measure. Recall says WHAT text gotex reproduced; the
// layout diff says WHERE it landed. For each sampled paper it compiles a
// reference PDF (tectonic) and a gotex PDF (reusing the same sampler, top-level
// resolution and compile wiring as the recall mode), extracts per-word bounding
// boxes from both with `pdftotext -bbox`, aligns the two word streams by content
// and measures how far matched words drift, whether the page count matches, and
// whether the same words start new lines. The result is ranked worst-first by a
// single layout-divergence score, so the biggest LAYOUT problems surface as fix
// targets.
//
// The measure is robust to the three known-expected engine differences that the
// recall mode already documents (see contentWords in analyze.go): page numbers
// are masked (the footer band plus the pure-numeric drop), math rendered as
// vector paths carries no extractable words on gotex's side and its single-glyph
// reference words are dropped with the single-character rule, and fi/fl-class
// ligatures are folded — so the score reflects REAL layout divergence, not the
// by-design engine differences.

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// footerBand is the fraction of a page's height, measured from the bottom, whose
// words are treated as the running footer and masked out. A page number sits in
// this band; the reference emits one and gotex (\pagestyle{empty}) does not, so
// leaving it in would register as a spurious extra word with no counterpart.
const footerBand = 0.06

// lineTolFrac decides when a word begins a new line: a word whose vertical
// position differs from the current line's baseline by more than this fraction
// of its own height starts a new line. pdftotext already emits words grouped by
// line, so a small tolerance absorbs sub-pixel jitter without merging real lines.
const lineTolFrac = 0.5

// lcsCap bounds the LCS dynamic-programming table (in cells). A paper whose two
// word streams multiply out to more than this falls back to the linear greedy
// aligner, so the tool never allocates an unbounded table on a pathological
// paper. 4e6 cells is ~16 MB of int32 — comfortable, and far above any real
// arXiv paper's word count product for a handful of sampled papers.
const lcsCap = 4_000_000

// wordBox is one extracted word and its bounding box on a page. Coordinates are
// in PDF points with the y axis pointing DOWN (pdftotext's convention: yMin is
// the top edge). page is 0-based. lineStart marks the first word of a text line.
type wordBox struct {
	text                   string
	page                   int
	xMin, yMin, xMax, yMax float64
	lineStart              bool
}

// pageDim is one page's width and height in PDF points.
type pageDim struct{ w, h float64 }

// pdfLayout is a parsed `pdftotext -bbox` document: the page dimensions (index =
// 0-based page number) and the words in reading order across all pages.
type pdfLayout struct {
	pages []pageDim
	words []wordBox
}

// normWord is a masked, content-normalised word reduced to the two page-relative
// coordinates the layout diff compares: hpos is the horizontal centre in [0,1]
// of the page width, and vpos is the vertical centre stacked across pages
// (float64(page) + centre/height), so a word one page later than its counterpart
// is a full 1.0 of vertical divergence. lineStart is carried through from the raw
// stream so line-break agreement can be measured on matched words.
type normWord struct {
	text      string
	hpos      float64
	vpos      float64
	lineStart bool
}

// parseBBox parses the XHTML that `pdftotext -bbox` emits into a pdfLayout,
// tagging the first word of each text line. It is tolerant of the surrounding
// html/head/body boilerplate: it reacts only to <page> and <word> elements and
// ignores everything else, so a missing or malformed wrapper does not derail it.
func parseBBox(data []byte) pdfLayout {
	var pl pdfLayout
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity

	page := -1        // current 0-based page index (-1 before the first <page>)
	lineY := 0.0      // baseline (yMin) of the current line
	haveLine := false // whether a line is open on the current page
	for {
		tok, err := dec.Token()
		if err != nil {
			break // EOF or an unrecoverable parse error: stop with what we have
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "page":
			pl.pages = append(pl.pages, pageDim{
				w: attrFloat(se, "width"),
				h: attrFloat(se, "height"),
			})
			page++
			haveLine = false
		case "word":
			wb := wordBox{
				page: page,
				xMin: attrFloat(se, "xMin"),
				yMin: attrFloat(se, "yMin"),
				xMax: attrFloat(se, "xMax"),
				yMax: attrFloat(se, "yMax"),
			}
			if text, err := dec.Token(); err == nil {
				if cd, ok := text.(xml.CharData); ok {
					wb.text = string(cd)
				}
			}
			if page < 0 {
				continue // a <word> before any <page>: no geometry to anchor it
			}
			tol := lineTolFrac * (wb.yMax - wb.yMin)
			if !haveLine || math.Abs(wb.yMin-lineY) > tol {
				wb.lineStart = true
				lineY = wb.yMin
				haveLine = true
			}
			pl.words = append(pl.words, wb)
		}
	}
	return pl
}

// normalize masks and projects a pdfLayout into the comparable word stream. It
// applies the same three known-difference normalisations as contentWords —
// ligatures folded, pure-numeric tokens dropped, single-character tokens dropped
// — and additionally drops words in the footer band, then projects each surviving
// word onto its (hpos, vpos) page-relative coordinates. Words whose page has no
// recorded dimensions, or a zero dimension, are skipped (no geometry to compare).
func normalize(pl pdfLayout) []normWord {
	out := make([]normWord, 0, len(pl.words))
	for _, wb := range pl.words {
		if wb.page < 0 || wb.page >= len(pl.pages) {
			continue
		}
		dim := pl.pages[wb.page]
		if dim.w <= 0 || dim.h <= 0 {
			continue
		}
		tok := foldToken(wb.text)
		if !isContentToken(tok) {
			continue
		}
		cy := (wb.yMin + wb.yMax) / 2
		if cy/dim.h > 1-footerBand {
			continue // running footer (page number): masked
		}
		cx := (wb.xMin + wb.xMax) / 2
		out = append(out, normWord{
			text:      tok,
			hpos:      cx / dim.w,
			vpos:      float64(wb.page) + cy/dim.h,
			lineStart: wb.lineStart,
		})
	}
	return out
}

// alignWords aligns two content-normalised word streams by their text, returning
// the matched index pairs in reading order. It uses a longest-common-subsequence
// alignment — robust to the insertions and deletions the known recall gaps
// produce (a missing math glyph, an extra footer word slipping through) — and
// falls back to a linear greedy aligner when the streams are large enough that
// the LCS table would exceed lcsCap.
func alignWords(ref, got []normWord) []alignPair {
	rt := texts(ref)
	gt := texts(got)
	if len(rt) == 0 || len(gt) == 0 {
		return nil
	}
	if len(rt)*len(gt) <= lcsCap {
		return lcsPairs(rt, gt)
	}
	return greedyPairs(rt, gt)
}

// alignPair is one matched word: index ri in the reference stream aligned to
// index gi in the gotex stream.
type alignPair struct{ ri, gi int }

// lcsPairs computes the longest common subsequence of two token slices and
// returns the matched index pairs in order. Standard O(n*m) dynamic programming
// with an int32 table (bounded by the caller via lcsCap) and a backtrack.
func lcsPairs(a, b []string) []alignPair {
	n, m := len(a), len(b)
	// dp[i][j] = LCS length of a[i:] and b[j:]; one extra row/col of zeros.
	dp := make([][]int32, n+1)
	for i := range dp {
		dp[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var pairs []alignPair
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[i] == b[j]:
			pairs = append(pairs, alignPair{i, j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			i++
		default:
			j++
		}
	}
	return pairs
}

// greedyPairs is the linear fallback aligner for streams too large for the LCS
// table. It is a two-pointer diff heuristic: on a mismatch it advances the
// pointer whose current token reappears sooner on the other side (bounded by a
// small look-ahead window), which recovers cleanly from short insertions and
// deletions without ever allocating a quadratic table.
func greedyPairs(a, b []string) []alignPair {
	const window = 64
	var pairs []alignPair
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			pairs = append(pairs, alignPair{i, j})
			i++
			j++
			continue
		}
		di := lookAhead(a[i], b, j+1, window) // distance to a[i] on b's side
		dj := lookAhead(b[j], a, i+1, window) // distance to b[j] on a's side
		if dj <= di {
			i++ // b[j] reappears sooner in a: a[i] is an insertion, skip it
		} else {
			j++
		}
	}
	return pairs
}

// lookAhead returns the offset of the first occurrence of tok in s[from:] within
// the next window elements, or window+1 when it is not found in that window.
func lookAhead(tok string, s []string, from, window int) int {
	for k := 0; k < window && from+k < len(s); k++ {
		if s[from+k] == tok {
			return k
		}
	}
	return window + 1
}

// texts extracts the token slice from a normWord stream.
func texts(ws []normWord) []string {
	out := make([]string, len(ws))
	for i, w := range ws {
		out[i] = w.text
	}
	return out
}

// layoutScore is the per-paper geometric verdict. All displacements are in
// page-relative units (see normWord): 1.0 horizontally spans the page width, and
// 1.0 vertically spans one whole page.
type layoutScore struct {
	matched    int     // number of aligned words the statistics are built on
	medianDisp float64 // median matched-word displacement
	p95Disp    float64 // 95th-percentile matched-word displacement (the tail)
	refPages   int
	gotPages   int
	lineAgree  float64 // fraction of matched words that agree on line-start
	divergence float64 // the single worst-first ranking score
}

// scoreLayout aligns the two normalised streams and reduces them to a
// layoutScore. The divergence score combines the three signals on one
// page-relative scale, so gross failures dominate the ranking:
//
//		divergence = p95Disp + |refPages-gotPages| + (1 - lineAgree)
//
//	  - p95Disp is the tail of the per-word displacement: a wholesale shift or a
//	    paragraph that reflowed onto the wrong part of the page pushes it up;
//	  - |refPages-gotPages| is a full unit per surplus/missing page, so page
//	    overflow or a dropped page immediately outranks sub-page drift;
//	  - (1 - lineAgree) adds up to one unit as the same words stop starting lines
//	    together (line breaking diverging in justified paragraphs).
//
// When alignment finds no common words at all the two documents share no
// comparable layout, which is itself a gross divergence: a full unit is added.
func scoreLayout(ref, got []normWord, refPages, gotPages int) layoutScore {
	pairs := alignWords(ref, got)
	disp := displacements(pairs, ref, got)
	med, p95 := medianP95(disp)
	agree := lineBreakAgreement(pairs, ref, got)
	s := layoutScore{
		matched:    len(pairs),
		medianDisp: med,
		p95Disp:    p95,
		refPages:   refPages,
		gotPages:   gotPages,
		lineAgree:  agree,
	}
	s.divergence = p95 + math.Abs(float64(refPages-gotPages)) + (1 - agree)
	if len(pairs) == 0 {
		s.divergence += 1 // no shared layout to measure: a gross divergence
	}
	return s
}

// displacements returns the page-relative displacement of every matched word:
// the Euclidean distance between the reference and gotex positions, with the
// vertical component spanning pages (so a word one page off contributes ~1.0).
func displacements(pairs []alignPair, ref, got []normWord) []float64 {
	out := make([]float64, len(pairs))
	for k, p := range pairs {
		dh := ref[p.ri].hpos - got[p.gi].hpos
		dv := ref[p.ri].vpos - got[p.gi].vpos
		out[k] = math.Hypot(dh, dv)
	}
	return out
}

// medianP95 returns the median and the 95th percentile of vals (nearest-rank,
// zero for an empty input). It sorts a copy, leaving the caller's slice intact.
func medianP95(vals []float64) (median, p95 float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	s := make([]float64, len(vals))
	copy(s, vals)
	sort.Float64s(s)
	median = s[len(s)/2]
	// Nearest-rank: for any len>=1 this index lies in [0, len-1], so no clamp is
	// needed (ceil(0.95*len) is in [1, len]).
	rank := int(math.Ceil(0.95*float64(len(s)))) - 1
	p95 = s[rank]
	return median, p95
}

// lineBreakAgreement returns the fraction of matched words whose line-start flag
// agrees between the two engines — a signal for whether the SAME words begin new
// lines (i.e. whether line breaking matches). With no matched words it returns 0:
// no evidence of agreement.
func lineBreakAgreement(pairs []alignPair, ref, got []normWord) float64 {
	if len(pairs) == 0 {
		return 0
	}
	agree := 0
	for _, p := range pairs {
		if ref[p.ri].lineStart == got[p.gi].lineStart {
			agree++
		}
	}
	return float64(agree) / float64(len(pairs))
}

// layoutResult is one paper's layout outcome, mirroring paperResult. status is
// one of the shared statusXxx constants (statusOK, statusNoToplevel,
// statusRefMissing, statusGotexFailed).
type layoutResult struct {
	id     string
	class  string
	status string
	score  layoutScore
}

// analyzeLayout runs the full per-paper layout pipeline: resolve the top-level
// .tex, compile it with both engines, extract and normalise both bbox streams,
// and score the geometric divergence. It mirrors analyzePaper's control flow and
// status reporting so the two modes classify papers identically.
func analyzeLayout(dir, id, workDir string, pipe layoutPipeline) layoutResult {
	res := layoutResult{id: id, class: "?"}
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

	_ = pipe.compileRef(tex, refPDF)
	refLayout := pipe.extract(refPDF)
	refWords := normalize(refLayout)
	if len(refWords) == 0 {
		res.status = statusRefMissing
		return res
	}

	_ = pipe.compileGot(tex, gotPDF)
	gotLayout := pipe.extract(gotPDF)
	gotWords := normalize(gotLayout)
	if len(gotWords) == 0 {
		res.status = statusGotexFailed
		res.score.refPages = len(refLayout.pages)
		return res
	}
	res.score = scoreLayout(refWords, gotWords, len(refLayout.pages), len(gotLayout.pages))
	res.status = statusOK
	return res
}

// reportLayout prints the layout results ranked worst-first: scored papers by
// divergence DESCENDING (the biggest layout problems on top), then the papers
// that could not be scored. The breakdown columns are the raw evidence behind
// each divergence score, so a fix target can be read straight off the row.
func reportLayout(results []layoutResult, w io.Writer) {
	var scored, unscored []layoutResult
	for _, r := range results {
		if r.status == statusOK || r.status == statusGotexFailed {
			scored = append(scored, r)
		} else {
			unscored = append(unscored, r)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score.divergence != scored[j].score.divergence {
			return scored[i].score.divergence > scored[j].score.divergence
		}
		return scored[i].id < scored[j].id
	})

	fmt.Fprintf(w, "layout-diff fidelity — %d paper(s) scored, worst first\n\n", len(scored))
	fmt.Fprintf(w, "%-12s %-14s %8s %8s %8s %5s %5s %7s %7s   %s\n",
		"id", "class", "diverge", "medDisp", "p95Disp", "refPg", "gotPg", "line%", "matched", "status")
	var sum float64
	for _, r := range scored {
		s := r.score
		fmt.Fprintf(w, "%-12s %-14s %8.3f %8.3f %8.3f %5d %5d %6.1f%% %7d   %s\n",
			r.id, truncate(r.class, 14), s.divergence, s.medianDisp, s.p95Disp,
			s.refPages, s.gotPages, s.lineAgree*100, s.matched, layoutNote(r))
		sum += s.divergence
	}
	if len(scored) > 0 {
		fmt.Fprintf(w, "\nmean divergence over scored papers: %.3f\n", sum/float64(len(scored)))
	}
	if len(unscored) > 0 {
		fmt.Fprintf(w, "\nnot scored (no ground truth or no top-level .tex):\n")
		for _, r := range unscored {
			fmt.Fprintf(w, "  %-12s %-14s %s\n", r.id, truncate(r.class, 14), r.status)
		}
	}
}

// layoutNote annotates a scored row with the dominant gross failure, if any, so
// the worst rows read as fix targets rather than bare numbers.
func layoutNote(r layoutResult) string {
	if r.status == statusGotexFailed {
		return "gotex-failed"
	}
	s := r.score
	switch {
	case s.gotPages > s.refPages:
		return fmt.Sprintf("PAGE OVERFLOW (+%d)", s.gotPages-s.refPages)
	case s.gotPages < s.refPages:
		return fmt.Sprintf("MISSING PAGES (-%d)", s.refPages-s.gotPages)
	case s.matched == 0:
		return "NO SHARED LAYOUT"
	default:
		return "ok"
	}
}

// layoutPipeline is the set of external actions analyzeLayout needs, injected so
// the per-paper logic is testable without tectonic, gotex or a network. It
// mirrors pipeline, but the extractor returns a parsed bbox layout rather than
// plain text.
type layoutPipeline struct {
	compileRef func(texPath, outPDF string) error
	compileGot func(texPath, outPDF string) error
	extract    func(pdf string) pdfLayout
}

// realLayoutPipeline wires the external binaries for layout mode: tectonic for
// the reference, gotex for the candidate (both reusing the recall mode's
// runners), and `pdftotext -bbox` for per-word bounding boxes.
func realLayoutPipeline(gotexBin string, timeout time.Duration) layoutPipeline {
	tectonic := toolRunner("tectonic")
	bbox := bboxRunner("pdftotext")
	return layoutPipeline{
		compileRef: func(texPath, outPDF string) error {
			return runTectonic(tectonic, timeout, texPath, outPDF)
		},
		compileGot: func(texPath, outPDF string) error {
			return runCmd(timeout, gotexBin, "-lenient", "-o", outPDF, texPath)
		},
		extract: func(pdf string) pdfLayout {
			return extractLayout(bbox, timeout, pdf)
		},
	}
}

// extractLayout returns a PDF's parsed bbox layout via the given runner, or an
// empty layout when the file is absent, empty, or the extractor fails — the same
// three-way handling as extractText.
func extractLayout(run func(time.Duration, string) ([]byte, error), timeout time.Duration, pdf string) pdfLayout {
	if fi, err := os.Stat(pdf); err != nil || fi.Size() == 0 {
		return pdfLayout{}
	}
	out, err := run(timeout, pdf)
	if err != nil {
		return pdfLayout{}
	}
	return parseBBox(out)
}

// foldToken normalises one extracted bbox word to its comparable content form,
// applying the same folding as contentWords so the two engines' words align: the
// fi/fl-class ligatures are folded, the word is lower-cased, and any non
// alphanumeric characters (surrounding punctuation, an internal hyphen) are
// stripped so "words." and "words" are the same token. Both streams receive the
// identical transformation, so the alignment is consistent regardless of how a
// residual punctuation mark landed.
func foldToken(word string) string {
	folded := strings.ToLower(ligatureFolder.Replace(word))
	var b strings.Builder
	for _, r := range folded {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// bboxRunner returns a function that runs `pdftotext -bbox <pdf> -` and returns
// the XHTML bbox dump on stdout. Like captureRunner it decides PATH-vs-pkgx once,
// so a genuinely failing extractor is not silently retried through pkgx.
func bboxRunner(name string) func(timeout time.Duration, pdf string) ([]byte, error) {
	usePkgx := false
	if _, err := exec.LookPath(name); err != nil {
		usePkgx = true
	}
	return func(timeout time.Duration, pdf string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		var cmd *exec.Cmd
		if usePkgx {
			cmd = exec.CommandContext(ctx, "pkgx", name, "-bbox", pdf, "-")
		} else {
			cmd = exec.CommandContext(ctx, name, "-bbox", pdf, "-")
		}
		return cmd.Output()
	}
}

// attrFloat returns the named attribute of a start element parsed as a float, or
// zero when it is absent or unparseable.
func attrFloat(se xml.StartElement, name string) float64 {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			f, _ := strconv.ParseFloat(a.Value, 64)
			return f
		}
	}
	return 0
}
