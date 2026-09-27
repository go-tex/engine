// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \begin{appendices} … \end{appendices} is the appendix package's environment form of
// \appendix (appendix.sty:204-237). Its body is mostly the [toc]/[page]/[title] options'
// machinery; what it does unconditionally is \@resets@pp — zero the section counters and
// letter them, which is \appendix — and it ends with \@ppsaveapp\@pprestoresec, RESTORING
// the numbering afterwards, which a bare \appendix does not.
//
// Undefined, \begin{appendices} resolved to \relax through \csname and the appendices were
// numbered as ordinary sections. Four corpus papers.
//
// The restore is GLOBAL, and that is the part that took measuring: \appendix redefines
// \thesection with \gdef, so a \let saved and restored locally does not survive it. With
// the local form tectonic letters the section after \end{appendices} exactly as we did —
// the reference AGREED WITH THE BUG, which is how the mechanism got named. With
// \global\let / \xdef both engines give 1, A, B, 2.
func TestAppendicesLettersAndThenRestores(t *testing.T) {
	const src = `\documentclass{article}\usepackage{appendix}\begin{document}` +
		`\section{FIRST}\label{s1}` +
		`\begin{appendices}\section{APPA}\label{a1}\section{APPB}\label{a2}\end{appendices}` +
		`\section{AFTER}\label{s2}` +
		`refs: \ref{s1} \ref{a1} \ref{a2} \ref{s2}` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := e.Diagnostics().UndefinedEnvs["appendices"]; got != 0 {
		t.Errorf("appendices still reported undefined (%d)", got)
	}
	// The cross-references are the numbering, written down: 1, A, B, 2 — and a \\ref is
	// where the numbering is legible, since a heading lays its number out separately.
	//
	// The markup is stripped first: the SVG splits a line into <tspan> elements, so
	// "refs: 1 A B 2" is not a contiguous string in it and asserting on the raw SVG failed
	// against a fix that works.
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "refs: 1 A B 2") {
		i := strings.Index(text, "refs:")
		got := text
		if i >= 0 && i+24 <= len(text) {
			got = text[i : i+24]
		}
		t.Errorf("the numbering reads %q, want it to contain \"refs: 1 A B 2\"", got)
	}
}

// stripSVGTags reduces a rendered page to the characters it shows, collapsing runs of
// whitespace, so a test can assert on a LINE rather than on how the renderer split it.
func stripSVGTags(svg string) string {
	var b strings.Builder
	depth := 0
	for _, r := range svg {
		switch {
		case r == '<':
			depth++
		case r == '>':
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
