// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"math"
	"regexp"
	"strconv"
	"testing"
)

// titlesec was a pair of gobbles (\titleformat and \titlespacing read their
// arguments and recorded nothing), so a paper that restyled its headings kept the
// class's: +2 pages and a heading at 17.22pt against tectonic's 13.95 in
// 2402.04711. There is no titlesec.sty on the support tree to load, so the two
// commands now record what \@gxhead's callers read (classkernel.go,
// classprims.go).
//
// Measured against tectonic on the witness below (\titleformat{\section}
// {\normalfont\large\bfseries}{\thesection}{1em}{}): label at x=133.77, title at
// x=152.45, "Alpha" 35.39pt wide, the second heading 50.81pt under the first.
// gotex: 133.77, 152.46, 35.39, 50.81.

type tspan struct {
	text       string
	x, y, size float64
}

var tspanRE = regexp.MustCompile(`<tspan x="([^"]*)" y="([^"]*)" font-size="([^"]*)"[^>]*>([^<]*)</tspan>`)

func tsHeadings(t *testing.T, preamble string) map[string]tspan {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}`+preamble+
		`\begin{document}\section{Alpha}Body text one.\section{Beta}Body text two.\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]tspan{}
	for _, p := range e.RenderPages(e.renderMargin(0)) {
		for _, m := range tspanRE.FindAllStringSubmatch(p, -1) {
			x, _ := strconv.ParseFloat(m[1], 64)
			y, _ := strconv.ParseFloat(m[2], 64)
			s, _ := strconv.ParseFloat(m[3], 64)
			if _, seen := out[m[4]]; !seen {
				out[m[4]] = tspan{m[4], x, y, s}
			}
		}
	}
	for _, w := range []string{"Alpha", "Beta", "Body"} {
		if _, ok := out[w]; !ok {
			t.Fatalf("word %q not on the page: %v", w, out)
		}
	}
	return out
}

func TestTitleformatSetsTheHeadingFont(t *testing.T) {
	base := tsHeadings(t, ``)
	got := tsHeadings(t, `\usepackage{titlesec}\titleformat{\section}{\normalfont\footnotesize\bfseries}{\thesection}{1em}{}`)
	if base["Alpha"].size <= 10 {
		t.Fatalf("control: the class heading is %.2fpt, want above 10", base["Alpha"].size)
	}
	if got["Alpha"].size >= 10 {
		t.Errorf("\\titleformat font ignored: heading at %.2fpt, want the \\footnotesize one",
			got["Alpha"].size)
	}
	// Only \section was restyled: \subsection keeps the class's size, so the record is
	// per heading name and not a global switch.
	if got["Body"].size != base["Body"].size {
		t.Errorf("body text moved from %.2f to %.2fpt", base["Body"].size, got["Body"].size)
	}
}

func TestTitleformatLabelAndSeparator(t *testing.T) {
	one := tsHeadings(t, `\titleformat{\section}{\normalfont\large}{\thesection}{1em}{}`)
	four := tsHeadings(t, `\titleformat{\section}{\normalfont\large}{\thesection}{4em}{}`)
	// The title follows the label by the separator: three more em at the heading's
	// size, so the title moves right by 3*size.
	d := four["Alpha"].x - one["Alpha"].x
	want := 3 * one["Alpha"].size
	if math.Abs(d-want) > 1 {
		t.Errorf("a 4em separator moved the title %.2fpt right of a 1em one, want %.2f", d, want)
	}
}

func TestTitlespacingMovesTheHeading(t *testing.T) {
	base := tsHeadings(t, ``)
	got := tsHeadings(t, `\titlespacing*{\section}{0pt}{40pt}{20pt}`)
	// Space under the heading: the body line sits that much lower beneath it.
	gapBase := base["Body"].y - base["Alpha"].y
	gapGot := got["Body"].y - got["Alpha"].y
	if gapGot-gapBase < 5 {
		t.Errorf("heading-to-body gap %.2f with \\titlespacing after=20pt, %.2f without: ignored",
			gapGot, gapBase)
	}
}

// The space ABOVE, on its own: the space below is left at the class's value, so the
// only thing that can move the second heading is the before-space.
func TestTitlespacingBeforeSpace(t *testing.T) {
	base := tsHeadings(t, ``)
	got := tsHeadings(t, `\titlespacing*{\section}{0pt}{40pt}{2.3ex plus .2ex}`)
	d := (got["Beta"].y - got["Alpha"].y) - (base["Beta"].y - base["Alpha"].y)
	if d < 15 {
		t.Errorf("a 40pt before-space moved the second heading %.2fpt, want about 25", d)
	}
}

// With titlesec's `explicit` option the before-code of \titleformat takes the
// title as #1 (titlesec.sty:736), which is how wlpeerj.cls:75-79 uppercases its
// section titles. Tectonic prints "1 INTRODUCTION" for the witness; the engine
// printed "1 Introduction" because it recorded the code as a parameterless macro.
func TestTitleformatExplicitTakesTheTitle(t *testing.T) {
	src := func(opt string) []byte {
		return []byte(`\documentclass{article}\usepackage` + opt + `{titlesec}` +
			`\titleformat{\section}{\normalfont\large}{\thesection}{1em}{\MakeUppercase{#1}}` +
			`\begin{document}\section{Alpha}Body text one.\end{document}`)
	}
	words := func(opt string) map[string]bool {
		e, err := compile(src(opt), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, p := range e.RenderPages(e.renderMargin(0)) {
			for _, m := range tspanRE.FindAllStringSubmatch(p, -1) {
				out[m[4]] = true
			}
		}
		return out
	}
	got := words(`[explicit]`)
	if !got["ALPHA"] || got["Alpha"] {
		t.Errorf("explicit: the title was not given to the before-code, words %v", got)
	}
}
