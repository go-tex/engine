// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"fmt"
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
	// Each title must appear UNDER its own heading, not merely somewhere on the page:
	// the caption itself carries the same words, so a bare Contains passed even when
	// both lists came out as empty headings (which is what they did — a class asks for
	// its list by file name, \@starttoc{lof}, and nothing matched the "figure" kind
	// the entry was recorded under).
	lof := strings.Index(text, "ListofFigures")
	lot := strings.Index(text, "ListofTables")
	if lof < 0 || lot < 0 || lot < lof {
		t.Fatalf("headings missing or out of order: %q", text)
	}
	if entry := text[lof:lot]; !strings.Contains(entry, "1Apicture") {
		t.Errorf("the list of figures carries no entry: %q", entry)
	}
	if entry := text[lot:]; !strings.Contains(entry, "1Somedata") {
		t.Errorf("the list of tables carries no entry: %q", entry)
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

// A contents list long enough to spill onto a second page pushes every section
// it lists one page further down than the auxiliary pass — which laid out a
// document with no contents list at all — measured. Two passes report those
// stale numbers; compile reruns until they settle.
//
// The short document is the control, and it is the part that proves the fix is
// arithmetic rather than a constant: its contents list fits on one page, so
// nothing moves and no rerun is spent.
func TestTOCPagesSettleAcrossReruns(t *testing.T) {
	doc := func(n int) []byte {
		var b strings.Builder
		b.WriteString("\\documentclass{article}\n\\begin{document}\n\\tableofcontents\n")
		for i := 1; i <= n; i++ {
			fmt.Fprintf(&b, "\\newpage\\section{Section number %d}Body %d.\n", i, i)
		}
		b.WriteString("\\end{document}")
		return []byte(b.String())
	}
	for _, c := range []struct {
		name      string
		sections  int
		wantFirst int // page the first section falls on
	}{
		// Counted in tectonic on these very documents: 18 sections give 19 pages
		// (one of contents), 30 give 32 and 45 give 47 (two of contents each).
		// The first row USED to say 30 sections, one contents page — an expectation
		// written by hand, which pinned a contents list of ours that was too tight
		// because every \section entry was missing \l@section's \addvspace{1.0em}.
		{"contents on one page", 18, 2},
		{"contents spilling onto a second", 30, 3},
		{"and still two at 45", 45, 3},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := compile(doc(c.sections), Options{})
			if err != nil {
				t.Fatal(err)
			}
			if got := e.tocSource[0].page; got != c.wantFirst {
				t.Errorf("contents list places section 1 on page %d, want %d", got, c.wantFirst)
			}
			// Every number the reader sees must be one this very pass would
			// measure again — the definition of having settled.
			if !e.crossRefsAgree(e.labelPages) {
				t.Error("page numbers had not settled when compile returned")
			}
			last := len(e.tocSource) - 1
			if got, want := e.tocSource[last].page, c.wantFirst+last; got != want {
				t.Errorf("contents list places the last section on page %d, want %d", got, want)
			}
		})
	}
}

// article.cls sets a \section contents entry with NO dot leader at all —
// \l@section is "#1\nobreak\hfil\nobreak\hb@xt@\@pnumwidth{\hss #2}"
// (texmf/article.cls:528-543) — and reaches for \@dottedtocline only from
// \l@subsection down. A contents list whose sections are dotted is a list that
// does not look like LaTeX's, measurably: against tectonic the dots ran across
// every section line here before this was fixed.
func TestTOCSectionEntriesCarryNoDotLeader(t *testing.T) {
	sections := []byte(`\documentclass{article}
\begin{document}
\tableofcontents
\section{Alpha}
\section{Gamma}
\end{document}`)
	e, err := compile(sections, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if hasDotLeader(e.mvl) {
		t.Error("a contents list of \\section entries alone carries a dot leader; LaTeX's \\l@section has none")
	}
	// The same document with one subsection must have one: \l@subsection IS
	// \@dottedtocline, so this is the control that the leader still works.
	withSub := []byte(`\documentclass{article}
\begin{document}
\tableofcontents
\section{Alpha}
\subsection{Beta}
\end{document}`)
	e2, err := compile(withSub, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDotLeader(e2.mvl) {
		t.Error("no dot leader on a \\subsection entry; \\@dottedtocline sets one")
	}
}

// \@dottedtocline tiles \hbox{$\mkern\@dotsep mu.\mkern\@dotsep mu$}: one dot
// plus 2 x 4.5mu, which is half an em of kern around it. Plain TeX's \dotfill
// tiles a .44em box instead — less than half as wide — so a contents list set
// with \dotfill shows roughly twice as many dots as LaTeX's. Measured against
// tectonic on an 11pt article: 8.49pt between dots there, 4.82pt here.
func TestTOCDotCellIsTheDotsepTile(t *testing.T) {
	e := New()
	e.SetFont(spMock{})
	// spMock: '.' is 5pt wide at a 10pt design size, so the tile is 5 + 10/2.
	if got, want := e.tocDotCell(), 10*unity; got != want {
		t.Errorf("tocDotCell() = %d sp, want %d (dot + 2x\\@dotsep)", got, want)
	}
	if dotfill := ptToSP(0.44 * 10); e.tocDotCell() <= dotfill {
		t.Errorf("the contents tile (%d sp) is no wider than \\dotfill's (%d sp)", e.tocDotCell(), dotfill)
	}
	// No font: fall back to the renderers' own \dotfill tile rather than zero.
	if got := New().tocDotCell(); got != 0 {
		t.Errorf("tocDotCell() with no font = %d, want 0", got)
	}
}

// The fill that pushes a section entry's page number to the right margin must
// outrank the paragraph's own \parfillskip. LaTeX cancels \parfillskip
// (\parfillskip -\@pnumwidth) and fills with \hfil; the engine does not, so an
// \hfil there SHARES the space with \parfillskip and strands the page number
// mid-line — which is exactly what it did, 134pt short of the margin.
func TestTOCSectionFillOutranksParfillskip(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
\tableofcontents
\section{Alpha}
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasFillGlue(e.mvl) {
		t.Error("no order-2 fill on the section entry: an \\hfil would leave the page number mid-line")
	}
}

// hasFillGlue reports whether any glue node with order-2 (fill) stretch and no
// leader is present, walking boxes recursively.
func hasFillGlue(nodes []node) bool {
	for _, n := range nodes {
		switch v := n.(type) {
		case glueNode:
			if v.leader == leaderNone && v.spec.stretchOrder == 2 && v.spec.stretch > 0 {
				return true
			}
		case *boxNode:
			if hasFillGlue(v.list) {
				return true
			}
		case frameNode:
			if v.inner != nil && hasFillGlue(v.inner.list) {
				return true
			}
		}
	}
	return false
}

// \caption records a figure/table entry at LEVEL 1 (latex.go:779), but article's
// \l@figure and \l@table are \@dottedtocline{1}{1.5em}{2.3em} (article.cls:554,
// 562) — the \l@subsection shape, dotted and indented. Only a CONTENTS entry at
// level 1 is \l@section's undotted one. Reading the level and ignoring the kind
// would set every list of figures in bold with no leader at all.
func TestTOCFigureEntriesTakeTheDottedShape(t *testing.T) {
	sec := tocShapeFor("toc", 1)
	if sec.dotted || sec.indent != 0 {
		t.Errorf("a level-1 contents entry = %+v, want \\l@section: undotted at indent 0", sec)
	}
	for _, kind := range []string{"figure", "table"} {
		got := tocShapeFor(kind, 1)
		if !got.dotted || got.indent != 1.5 || got.numWidth != 2.3 {
			t.Errorf("a level-1 %s entry = %+v, want \\@dottedtocline{1}{1.5em}{2.3em}", kind, got)
		}
	}
	// And a list of figures really does carry a leader end to end.
	src := []byte(`\documentclass{article}
\begin{document}
\listoffigures
\begin{figure}\caption{Alpha}\end{figure}
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !hasDotLeader(e.mvl) {
		t.Error("no dot leader in the list of figures; \\l@figure is \\@dottedtocline")
	}
}
