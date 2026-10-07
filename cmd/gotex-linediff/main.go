// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

// Command gotex-linediff puts one page of two PDFs side by side, LINE BY LINE,
// with the geometry of each line: where it starts, where it ends, its baseline,
// the face and size it is set in, and — when it holds a leader — how many dots
// it draws and how far apart.
//
// It exists because the whole-corpus instruments answer a different question.
// gotex-abdiff counts pages and gotex-refdiff scores a paper; neither can say
// "our contents entry starts 1.58pt to the right of the reference's and its dots
// are twice as dense", which is the sentence a fidelity defect is actually fixed
// from. Three such defects in the contents list were found and closed this way.
//
// It reads the PDFs with github.com/go-pdfkit/reader: no qpdf, no poppler, no
// MuPDF, nothing but Go, so it runs wherever the engine's own tests run. Text
// comes through each font's ToUnicode CMap and widths through /Widths or /W; a
// font that carries neither yields runs with no text and no width rather than a
// made-up one.
//
//	gotex-linediff ref.pdf ours.pdf            # both, page 1, side by side
//	gotex-linediff -page 3 -above 300 a.pdf b.pdf
//	gotex-linediff ours.pdf                    # one file, just its lines
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

func main() {
	page := flag.Int("page", 1, "page to read, 1-based")
	above := flag.Float64("above", 0, "only lines whose baseline is this many points or less from the top of the page (0 = all)")
	grep := flag.String("grep", "", "only lines whose text contains this")
	flag.Parse()
	if flag.NArg() < 1 || flag.NArg() > 2 {
		fmt.Fprintln(os.Stderr, "usage: gotex-linediff [-page N] [-above PT] [-grep TEXT] <a.pdf> [b.pdf]")
		os.Exit(2)
	}
	opt := reportOptions{page: *page, above: *above, grep: *grep}
	if err := run(flag.Args(), opt, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "gotex-linediff:", err)
		os.Exit(1)
	}
}

type reportOptions struct {
	page  int
	above float64
	grep  string
}

func run(paths []string, opt reportOptions, w io.Writer) error {
	var sb strings.Builder
	for i, p := range paths {
		lines, pageH, err := linesOf(p, opt)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if i > 0 {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "### %s  page %d  (height %s pt, %d lines)\n", p, opt.page, ftoa(pageH), len(lines))
		for _, l := range lines {
			sb.WriteString(l.String() + "\n")
		}
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// A Line groups the runs that share a baseline, in the order they were drawn.
type Line struct {
	Y        float64 // distance from the TOP of the page, which is how a reader counts
	X0, X1   float64 // first run's origin and last run's end
	Font     string
	Size     float64
	Text     string
	Leader   int     // how many single-character runs of the same text it holds
	Pitch    float64 // the mean distance between them, 0 when there are fewer than three
	unknownW bool
}

func (l Line) String() string {
	end := ftoa(l.X1)
	if l.unknownW {
		end = "?"
	}
	s := fmt.Sprintf("  y=%8s x0=%8s x1=%8s %-26s @%-6s", ftoa(l.Y), ftoa(l.X0), end, trunc(l.Font, 26), ftoa(l.Size))
	if l.Leader > 2 {
		s += fmt.Sprintf(" leader=%3d pitch=%6s", l.Leader, ftoa(l.Pitch))
	} else {
		s += strings.Repeat(" ", 25)
	}
	return s + " | " + trunc(l.Text, 72)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// linesOf reads one page of one PDF and groups its runs into lines.
func linesOf(path string, opt reportOptions) ([]Line, float64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	doc, err := openPDF(b)
	if err != nil {
		return nil, 0, err
	}
	if opt.page < 1 || opt.page > doc.PageCount() {
		return nil, 0, fmt.Errorf("page %d of %d", opt.page, doc.PageCount())
	}
	runs, err := PageRuns(doc, opt.page) // the reader numbers pages from 1
	if err != nil {
		return nil, 0, err
	}
	h := pageHeight(doc, opt.page)
	return groupLines(runs, h, opt), h, nil
}

// groupLines gathers runs that share a baseline. Runs are grouped by their y
// rounded to a hundredth of a point: two glyphs a renderer puts on one line can
// differ in the last digit, and two real lines never differ by so little.
func groupLines(runs []Run, pageH float64, opt reportOptions) []Line {
	byY := map[int][]Run{}
	for _, r := range runs {
		byY[int(r.Y*100+0.5)] = append(byY[int(r.Y*100+0.5)], r)
	}
	keys := make([]int, 0, len(byY))
	for k := range byY {
		keys = append(keys, k)
	}
	// Descending y is top-down on the page.
	sort.Sort(sort.Reverse(sort.IntSlice(keys)))
	var out []Line
	for _, k := range keys {
		rs := byY[k]
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].X < rs[j].X })
		l := lineOf(rs, pageH)
		if opt.above > 0 && l.Y > opt.above {
			continue
		}
		if opt.grep != "" && !strings.Contains(l.Text, opt.grep) {
			continue
		}
		out = append(out, l)
	}
	return out
}

func lineOf(rs []Run, pageH float64) Line {
	l := Line{Y: pageH - rs[0].Y, X0: rs[0].X, Font: rs[0].Font, Size: rs[0].Size}
	var sb strings.Builder
	var prevEnd float64
	for i, r := range rs {
		// Neither engine draws a space glyph: a word break is a gap between two
		// show operators. Put one back where the gap is wider than a sixth of the
		// size, so the report reads as prose instead of as "Firstsection1".
		if i > 0 && r.W > 0 && r.X-prevEnd > r.Size/6 {
			sb.WriteByte(' ')
		}
		sb.WriteString(r.Text)
		prevEnd = r.X + r.W
		if prevEnd > l.X1 {
			l.X1 = prevEnd
		}
		if r.W == 0 {
			l.unknownW = true
		}
	}
	l.Text = sb.String()
	l.Leader, l.Pitch = leaderOf(rs)
	return l
}

// leaderOf finds the longest run of consecutive single-character runs all saying
// the same thing — a dot leader, which both engines draw as one show operator per
// tile — and returns its length and the mean distance between its tiles.
//
// A leader has to be measured from the DRAWN tiles, not from the text: the text
// of a dotted contents line is "Title. . . . . 7" whichever tile width was used,
// so a reader comparing two engines on their text alone cannot see that one of
// them tiled twice as densely as the other. The x of each tile can.
func leaderOf(rs []Run) (int, float64) {
	best, bestAt := 0, 0
	for i := 0; i < len(rs); {
		if len([]rune(rs[i].Text)) != 1 {
			i++
			continue
		}
		j := i + 1
		for j < len(rs) && rs[j].Text == rs[i].Text {
			j++
		}
		if j-i > best {
			best, bestAt = j-i, i
		}
		i = j
	}
	if best < 3 {
		return best, 0
	}
	run := rs[bestAt : bestAt+best]
	return best, (run[len(run)-1].X - run[0].X) / float64(best-1)
}
