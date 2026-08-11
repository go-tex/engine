// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A numbered \section/\subsection records a "toc" entry (level, number, title);
// a starred \section* records nothing, matching LaTeX's contents list.
func TestTOCEntriesRecorded(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\section{Alpha}
\subsection{Beta}
\subsection{Gamma}
\section{Delta}
\section*{Hidden}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	want := []tocEntry{
		{kind: "toc", level: 1, number: "1", title: "Alpha"},
		{kind: "toc", level: 2, number: "1.1", title: "Beta"},
		{kind: "toc", level: 2, number: "1.2", title: "Gamma"},
		{kind: "toc", level: 1, number: "2", title: "Delta"},
	}
	if len(e.tocEntries) != len(want) {
		t.Fatalf("recorded %d entries, want %d: %+v", len(e.tocEntries), len(want), e.tocEntries)
	}
	for i, w := range want {
		g := e.tocEntries[i]
		if g.kind != w.kind || g.level != w.level || g.number != w.number || g.title != w.title {
			t.Errorf("entry %d = {%q,%d,%q,%q}, want {%q,%d,%q,%q}",
				i, g.kind, g.level, g.number, g.title, w.kind, w.level, w.number, w.title)
		}
	}
	// \section* must not appear.
	for _, en := range e.tocEntries {
		if en.title == "Hidden" {
			t.Error("starred \\section* leaked into the contents list")
		}
	}
}

// The full two-pass compile carries the aux-pass entries into the render pass,
// where \tableofcontents typesets a "Contents" heading and one line per numbered
// section/subsection, in order, with the numbers, titles and page numbers.
func TestTOCRenderTwoPass(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
\tableofcontents
\section{Alpha}
\subsection{Beta}
\section{Gamma}
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The render engine must have received the aux-pass entries with resolved pages.
	if len(e.tocSource) != 3 {
		t.Fatalf("render engine carried %d entries, want 3: %+v", len(e.tocSource), e.tocSource)
	}
	for i, en := range e.tocSource {
		if en.page < 1 {
			t.Errorf("entry %d %q has non-positive page %d", i, en.title, en.page)
		}
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	text := b.String()
	if !strings.Contains(text, "Contents") {
		t.Fatalf("no Contents heading in output: %q", text)
	}
	// The heading precedes every entry, and entries appear in document order.
	idx := func(s string) int { return strings.Index(text, s) }
	order := []string{"Contents", "Alpha", "Beta", "Gamma"}
	for i := 1; i < len(order); i++ {
		if idx(order[i-1]) < 0 || idx(order[i]) < 0 {
			t.Fatalf("missing %q or %q in %q", order[i-1], order[i], text)
		}
		if idx(order[i-1]) >= idx(order[i]) {
			t.Errorf("%q should precede %q in TOC output: %q", order[i-1], order[i], text)
		}
	}
	// The numbers 1, 1.1 and 2 must be typeset (both in the TOC and the bodies).
	for _, num := range []string{"1.1", "2"} {
		if !strings.Contains(text, num) {
			t.Errorf("section number %q not typeset: %q", num, text)
		}
	}
	// Dot leaders were emitted for the contents lines.
	if !hasDotLeader(e.mvl) {
		t.Error("no dot leader (\\dotfill) found in the contents list")
	}
}

// hasDotLeader reports whether any glue node tagged as a dot leader is present,
// walking boxes recursively (TOC lines carry a \dotfill leader).
func hasDotLeader(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case glueNode:
			if v.leader == leaderDots {
				return true
			}
		case *boxNode:
			if hasDotLeader(v.list) {
				return true
			}
		case frameNode:
			if v.inner != nil && hasDotLeader(v.inner.list) {
				return true
			}
		}
	}
	return false
}

// A \tableofcontents with no sections renders just the heading and does not
// crash (an empty contents list is valid, as in LaTeX).
func TestTOCEmpty(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
\tableofcontents
Body text only.
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.tocSource) != 0 {
		t.Fatalf("expected no entries, got %+v", e.tocSource)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "Contents") {
		t.Errorf("empty TOC should still print the Contents heading: %q", b.String())
	}
}

// \listoffigures / \listoftables collect caption entries of the matching float
// type only, and each renders under its own heading.
func TestListOfFiguresAndTables(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
\listoffigures
\listoftables
\begin{figure}\caption{A picture}\end{figure}
\begin{table}\caption{Some data}\end{table}
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	var figs, tabs int
	for _, en := range e.tocSource {
		switch en.kind {
		case "figure":
			figs++
			if en.title != "A picture" || en.number != "1" {
				t.Errorf("figure entry = {%q,%q}, want {\"1\",\"A picture\"}", en.number, en.title)
			}
		case "table":
			tabs++
			if en.title != "Some data" || en.number != "1" {
				t.Errorf("table entry = {%q,%q}, want {\"1\",\"Some data\"}", en.number, en.title)
			}
		}
	}
	if figs != 1 || tabs != 1 {
		t.Fatalf("collected %d figure and %d table entries, want 1 and 1", figs, tabs)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	// Inter-word spaces are glue (not charNodes), so compare with spaces removed.
	text := strings.ReplaceAll(b.String(), " ", "")
	for _, want := range []string{"ListofFigures", "ListofTables", "Apicture", "Somedata"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q: %q", want, text)
		}
	}
}

// doTOCEntry tolerates a malformed call with missing groups: a bare \@tocentry
// records a zero-valued entry rather than crashing (error-branch coverage).
func TestTOCEntryMalformed(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\@tocentry`); err != nil {
		t.Fatal(err)
	}
	if len(e.tocEntries) != 1 {
		t.Fatalf("want 1 entry recorded, got %d", len(e.tocEntries))
	}
	if g := e.tocEntries[0]; g.kind != "" || g.level != 0 || g.number != "" || g.title != "" {
		t.Errorf("malformed \\@tocentry recorded %+v, want zero value", g)
	}
}

// A \@tocentry whose arguments are not brace groups pushes each peeked
// non-brace token back (the readBraceGroupString error branch) and records a
// zero-valued entry, leaving the stray token to be typeset normally.
func TestTOCEntryNonBraceArgs(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt \@tocentry x`); err != nil {
		t.Fatal(err)
	}
	if len(e.tocEntries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(e.tocEntries))
	}
	if g := e.tocEntries[0]; g.kind != "" || g.level != 0 {
		t.Errorf("non-brace \\@tocentry recorded %+v, want zero value", g)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "x") {
		t.Errorf("stray token should still typeset: %q", b.String())
	}
}

// pageOfIndex clamps out-of-range indices to the last page and never returns 0
// for a document with content.
func TestPageOfIndex(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.vsize = 400 * 65536 // small pages to force multiple breaks
	if _, err := e.Run(`\hsize=300pt Line one.\par\vfill\penalty-10000 Line two.\par`); err != nil {
		t.Fatal(err)
	}
	if p := e.pageOfIndex(0); p != 1 {
		t.Errorf("first node on page %d, want 1", p)
	}
	if p := e.pageOfIndex(len(e.mvl) + 100); p < 1 {
		t.Errorf("out-of-range index gave page %d, want >= 1", p)
	}
	// A wholly empty main vertical list still reports page 1 (bottom guard).
	empty := New()
	if p := empty.pageOfIndex(0); p != 1 {
		t.Errorf("empty document page = %d, want 1", p)
	}
	// A list that trims to nothing (only glue) still reports page 1 for its
	// first index (the in-loop guard).
	glueOnly := New()
	glueOnly.mvl = []node{glueNode{}, glueNode{}}
	if p := glueOnly.pageOfIndex(0); p != 1 {
		t.Errorf("all-glue list page = %d, want 1", p)
	}
}

// tocList falls back to the live entries when no aux-pass table was carried,
// so a single-engine run still renders whatever preceded the command.
func TestTOCFallbackSinglePass(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\section{Alpha}
\tableofcontents`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "Contents") || !strings.Contains(b.String(), "Alpha") {
		t.Errorf("single-pass fallback TOC missing heading or entry: %q", b.String())
	}
}
