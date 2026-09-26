// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// achemso's author block has the same SHAPE as acmart's — \author plus \affiliation
// wrapping \institution / \city / \country sub-fields — and the article emulation
// knows none of it. Undefined, \affiliation's argument is not discarded in the
// document, so the affiliations were typeset as BODY TEXT: corpus paper 2209.13121
// has twelve of them, and "Measurement", "Gaithersburg," and "(NIST)," landed in the
// running prose. They ARE in its reference, so dropping them would be a content loss.
//
// The class also emits the title block itself at \begin{document} — the paper never
// writes \maketitle — so accumulating the fields is not enough: without firing it,
// the words are stored and never typeset, which is the same loss moved. First attempt
// did exactly that, and the four probes below are what caught it.
func TestAchemsoKeepsItsAffiliations(t *testing.T) {
	const src = `\documentclass{achemso}` +
		`\title{THETITLE}\author{A. Author}` +
		`\affiliation{\institution{Nowhere Institute}\city{Gaithersburg}\country{USA}}` +
		`\begin{document}BODYWORD\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	for _, want := range []string{"THETITLE", "Gaithersburg", "Nowhere", "BODYWORD"} {
		if !strings.Contains(svg, want) {
			t.Errorf("%q is not on the page — the title block was not emitted", want)
		}
	}
	if e.SkippedCommands()["affiliation"] != 0 {
		t.Errorf("\\affiliation still undefined: %v", e.SkippedCommands())
	}
}

// \maketitle disarms itself, so a class that fires it at \begin{document} and a
// document that also calls it do not produce the block twice. article.cls:192 and
// :225 do the same.
func TestMaketitleRunsOnce(t *testing.T) {
	const src = `\documentclass{achemso}\title{ONLYONCE}\author{A}` +
		`\begin{document}\maketitle BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	if n := strings.Count(svg, "ONLYONCE"); n != 1 {
		t.Errorf("the title appears %d times, want 1", n)
	}
}
