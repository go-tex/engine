// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's \tableofcontents (and \listoffigures /
// \listoftables) through the same two-pass, .aux-style mechanism the engine
// already uses for \label/\ref (see crossref.go and api.go). On the first
// (auxiliary) pass every numbered \section/\subsection and every \caption runs
// the \@tocentry primitive, which records a tocEntry (kind, level, number,
// title, and the main-vertical-list index where the material landed). After the
// aux pass the page each entry falls on is computed from the assembled pages
// (finalizeTOCPages), and the entry table is carried into the render pass —
// exactly as the label table is — where \tableofcontents typesets it.
//
// Page numbers are the classic multi-pass approximation LaTeX also lives with:
// they are the pages the sections occupied on the AUXILIARY pass, which did not
// yet contain the (space-consuming) contents list itself. A contents list long
// enough to spill onto extra pages can therefore shift later sections by a page
// relative to the numbers printed — the same instability real LaTeX resolves by
// running two or three times. The numbers are never invented: an entry that
// could not be placed reports page 0 (printed blank), not a fabricated value.

import (
	"strconv"
	"strings"
)

// tocEntry is one recorded contents line. kind selects the list it belongs to:
// "toc" for \tableofcontents (sections/subsections), "figure" for
// \listoffigures, "table" for \listoftables. level nests the entry (1 = section,
// 2 = subsection); number is the fully-expanded label (e.g. "2" or "2.1");
// title is the heading/caption text. marker is the len(mvl) at the moment the
// entry was recorded, from which page is derived after the aux pass.
type tocEntry struct {
	kind   string
	level  int
	number string
	title  string // detokenized, re-typeset for the on-page contents list
	// plainTitle is title with markup and accents resolved to clean Unicode text,
	// for a PDF bookmark /Title (a detokenized title would show literal \name and
	// accent commands as mojibake; see pdfstring.go).
	plainTitle string
	marker     int
	page       int
}

// doTOCEntry implements \@tocentry{kind}{level}{number}{title}: it appends a
// tocEntry recording where in the main vertical list the sectioning/caption
// material begins, so the two-pass machinery can later typeset it with a page
// number. It is emitted by the (redefined) \@nsection/\@nsubsection/\caption
// macros — only their NUMBERED forms, so \section* never reaches here, matching
// LaTeX (starred sections are absent from the contents list).
func (e *Engine) doTOCEntry() {
	kind := e.readBraceGroupString()
	level, _ := strconv.Atoi(trimSpaces(e.readBraceGroupString()))
	number := e.readBraceGroupString()
	title, plain := e.readTitleGroupBoth()
	e.tocEntries = append(e.tocEntries, tocEntry{
		kind:       kind,
		level:      level,
		number:     number,
		title:      title,
		plainTitle: plain,
		marker:     len(e.mvl),
	})
}

// tocAddLevels are the only names \addcontentsline records, and the levels they
// take. \chapter and \part are not \@startsection-based in any class — they call
// \addcontentsline themselves (book.cls:318,359) — so nothing else records them,
// and a book's contents list came out with no chapters in it at all.
//
// Everything else is dropped on purpose. A class calls \addcontentsline for its
// sections and its captions too, and both of those already reach the table by
// their own route (\@gxnum in classprims.go and the redefined \caption), so
// recording them here would put every section in the list twice.
var tocAddLevels = map[string]int{"part": -1, "chapter": 0}

// numberlineEnd is what \numberline{N} leaves after the number (classkernel.go),
// so the recorder can tell the entry's number from its title. A class writes the
// two as one argument: "\numberline{3}The Third Chapter".
const numberlineEnd = `\gotex@numberlineend`

// doAddContentsLine implements \addcontentsline{kind}{name}{text} for the names
// in tocAddLevels. The number, when the class supplied one, arrives inside the
// text wrapped in \numberline and is split back out here, so it lands in its own
// box in the contents list instead of running into the title.
func (e *Engine) doAddContentsLine() {
	kind := trimSpaces(e.readBraceGroupString())
	name := trimSpaces(e.readBraceGroupString())
	text, plain := e.readTitleGroupBoth()
	level, ok := tocAddLevels[name]
	if kind != "toc" || !ok {
		return
	}
	number, title := splitNumberline(text)
	// The plain rendering (for a PDF bookmark) drops control sequences, so the
	// marker is not in it and the number is simply glued to the front: "1One".
	// Take it off by the number the detokenized text yielded.
	plainTitle := trimSpaces(strings.TrimPrefix(trimSpaces(plain), number))
	e.tocEntries = append(e.tocEntries, tocEntry{
		kind:       "toc",
		level:      level,
		number:     number,
		title:      title,
		plainTitle: plainTitle,
		marker:     len(e.mvl),
	})
}

// splitNumberline separates "<number>\gotex@numberlineend<title>" into its two
// halves. Text with no marker is all title and carries no number, which is what
// an unnumbered chapter (\frontmatter, or \c@secnumdepth below zero) produces.
func splitNumberline(s string) (number, title string) {
	i := strings.Index(s, numberlineEnd)
	if i < 0 {
		return "", trimSpaces(s)
	}
	return trimSpaces(s[:i]), trimSpaces(s[i+len(numberlineEnd):])
}

// readTitleGroupBoth reads a {…} title group once and returns it two ways: the
// detokenized string the on-page contents list re-typesets (like
// readBraceGroupString), and a clean plain-text rendering for a PDF bookmark
// (see pdfstring.go). Reading once keeps the two in step and consumes the group
// exactly once.
func (e *Engine) readTitleGroupBoth() (detok, plain string) {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return "", ""
	}
	toks := e.expandList(e.grabGroup())
	return e.toksToString(toks), e.tokensToPlainText(toks)
}

// readBraceGroupString reads a {…} group and returns its content fully expanded
// to a string (like doLabel does for \@currentlabel). A missing group yields "".
func (e *Engine) readBraceGroupString() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return ""
	}
	return e.toksToString(e.expandList(e.grabGroup()))
}

// tocList returns the entries to typeset for a given list kind, preferring the
// table carried from the aux pass (tocSource, with page numbers resolved) and
// falling back to whatever the current run has collected so far. The fallback
// makes a single-engine run (no two-pass) still render whatever precedes the
// command, which is what direct callers and tests rely on.
func (e *Engine) tocList(kind string) []tocEntry {
	// A real class asks for its list by FILE name — \@starttoc{lof} for the list of
	// figures, {lot} for tables (latex.ltx: \listoffigures calls \@starttoc{lof}) —
	// while an entry records the caption type \@captype gave it. Without this
	// mapping \listoffigures and \listoftables came out as a heading with nothing
	// under it, whatever the document contained.
	switch kind {
	case "lof":
		kind = "figure"
	case "lot":
		kind = "table"
	}
	src := e.tocSource
	if src == nil {
		src = e.tocEntries
	}
	var out []tocEntry
	for _, en := range src {
		if en.kind == kind {
			out = append(out, en)
		}
	}
	return out
}

// doTableOfContents implements \tableofcontents: a "Contents" heading followed
// by one line per recorded section/subsection entry.
func (e *Engine) doTableOfContents() {
	e.emitTOCList("Contents", e.tocList("toc"))
}

// doListOfFigures implements \listoffigures.
func (e *Engine) doListOfFigures() {
	e.emitTOCList("List of Figures", e.tocList("figure"))
}

// doListOfTables implements \listoftables.
func (e *Engine) doListOfTables() {
	e.emitTOCList("List of Tables", e.tocList("table"))
}

// emitTOCList pushes the TeX token list that typesets a contents list: a
// \section*-styled heading, then, for each entry, a \noindent line of
// "number title" — indented one \quad per nesting level beyond the first —
// with a \dotfill dot leader stretching to a right-flushed page number, broken
// to \hsize by the ordinary paragraph builder. An empty list emits just the
// heading (as LaTeX does). The tokens are built with the engine's live
// catcodes, so control sequences (\Large, \dotfill, \par…) are real cs tokens,
// not literal text.
func (e *Engine) emitTOCList(heading string, entries []tocEntry) {
	var b tocTokens
	b.e = e
	// Heading, styled like \section*{heading}.
	b.cs("par")
	b.cs("medskip")
	b.cs("noindent")
	b.begin()
	b.cs("Large")
	b.cs("bf")
	b.text(heading)
	b.end()
	b.cs("par")
	b.cs("nobreak")
	b.cs("smallskip")
	e.emitTOCEntryTokens(&b, entries)
	e.push(b.ts)
}

// doStartTOC implements \@starttoc{kind}: it typesets the recorded entries for that
// list (toc/lof/lot) WITHOUT a heading — a real class's \tableofcontents/
// \listoffigures already issues its own \section*{…} heading and then calls
// \@starttoc. It bridges the class's TOC command to the engine's two-pass entry
// table (see tocList), so a loaded article.cls still renders a dotted contents list.
func (e *Engine) doStartTOC() {
	kind := e.readBraceName()
	if kind == "" {
		return
	}
	var b tocTokens
	b.e = e
	e.emitTOCEntryTokens(&b, e.tocList(kind))
	e.push(b.ts)
}

// tocShape is how ONE contents entry is set: the indent it hangs at and the width
// of the box its number sits in (both em of the entry's font), whether a dot
// leader runs from the title to the page number, whether the title is bold, and
// the \addvspace that precedes the entry (em).
type tocShape struct {
	indent, numWidth float64
	dotted           bool
	bold             bool
	vspaceBefore     float64
}

// tocFallbackLadder is article.cls's contents ladder (texmf/article.cls:528-547),
// used only when NO class defines an \l@<name> for the level — the engine's own
// LaTeX emulation (latex.go's \@nsection), which has no class file to read.
// Levels past the last row reuse it, as \@dottedtocline does for an unlisted depth.
var tocFallbackLadder = []tocShape{
	{numWidth: 1.5, bold: true, vspaceBefore: 1}, // \l@section:    \@tempdima 1.5em, \bfseries, \hfil
	{indent: 1.5, numWidth: 2.3, dotted: true},   // \l@subsection: \@dottedtocline{2}{1.5em}{2.3em}
	{indent: 3.8, numWidth: 3.2, dotted: true},   // \l@subsubsection
	{indent: 7.0, numWidth: 4.1, dotted: true},   // \l@paragraph
	{indent: 10.0, numWidth: 5.0, dotted: true},  // \l@subparagraph
}

// tocLevelName names the sectioning command that recorded an entry at a given
// level. \@startsection numbers them identically in article, report and book
// (article.cls:390-410, book.cls:404-420), so the level alone picks the \l@<name>
// the loaded class defines — which is the macro that decides the entry's shape.
var tocLevelName = map[int]string{
	-1: "part", 0: "chapter", 1: "section", 2: "subsection",
	3: "subsubsection", 4: "paragraph", 5: "subparagraph",
}

// tocPnumWidth is \@pnumwidth (article.cls:499): the box the page number is set
// right-aligned in, flush with the right margin. The leader (or the \hfil of a
// section entry) stops where that box begins, which is why the dots of a real
// contents list stop short of the margin rather than running up to the number.
const tocPnumWidth = 1.55 // em

// tocDotSep is \@dotsep (article.cls:501), in mu — 18mu is one em. LaTeX's
// contents leader is \leaders\hbox{$\mkern\@dotsep mu\hbox{.}\mkern\@dotsep mu$},
// so one tile is the dot plus 2 x 4.5mu = half an em of surrounding kern.
const tocDotSep = 4.5

// tocDotCell returns the width in sp of one tile of that leader for the current
// font. It is about twice the .44em box plain TeX's \dotfill tiles, which is the
// whole point: a contents list set with \dotfill has visibly too many dots.
// Zero when no font is current, leaving the renderers on the \dotfill tile.
func (e *Engine) tocDotCell() int {
	f := e.curFont
	if f == nil {
		return 0
	}
	w, _, _ := f.charDimsSP('.')
	return w + ptToSP(2*tocDotSep/18*float64(f.sizePt()))
}

// tocShapeFor resolves how an entry is set by READING THE LOADED CLASS, not by
// assuming one. The level is not enough on its own: article's \l@section is
// hand-written (bold, \hfil, no leader) while book's and report's \l@section at
// the SAME level is \@dottedtocline{1}{1.5em}{2.3em} (book.cls:635). Keying the
// shape on the level alone set a thesis's contents list in article's shape and
// cost three pages on a real corpus paper (2402.04711, \documentclass{book}).
// A figure or table entry names its own \l@figure/\l@table, which every class
// writes as \@dottedtocline even though \caption records them at level 1.
func (e *Engine) tocShapeFor(en tocEntry) tocShape {
	name := en.kind // "figure"/"table" ARE the \l@ names
	if en.kind == "toc" {
		name = tocLevelName[en.level]
	}
	indent, numWidth, haveDims, dotted := e.dottedTocLine(name)
	fallback := tocFallbackLadder[min(max(en.level, 1), len(tocFallbackLadder))-1]
	switch {
	case dotted:
		s := tocShape{indent: fallback.indent, numWidth: fallback.numWidth, dotted: true}
		if haveDims {
			s.indent, s.numWidth = indent, numWidth
		}
		return s
	case e.eq["l@"+name] != nil:
		// A hand-written \l@… — \l@part, \l@chapter, article's \l@section — which
		// every class spells the same way: bold, \hfil instead of a leader, and a
		// blank line before. Only \l@part is set apart (article.cls:509-526).
		if name == "part" {
			return tocShape{numWidth: 3, bold: true, vspaceBefore: 2.25}
		}
		return tocShape{numWidth: 1.5, bold: true, vspaceBefore: 1}
	}
	if en.kind != "toc" {
		// No class: a list of figures still takes the \l@subsection shape, which is
		// what \l@figure is in every class that defines one.
		return tocFallbackLadder[1]
	}
	return fallback
}

// dottedTocLine reads the class's own \l@<name>: dotted reports whether it is a
// \@dottedtocline, and indent/numWidth its second and third arguments when both
// are a plain <number>em — which is how every class in the wild writes them
// (article.cls:544-547, book.cls:635-639). A \@dottedtocline whose dimensions are
// written some other way still counts as dotted; only its two lengths are lost,
// and the caller falls back to article's for that level rather than to no leader.
func (e *Engine) dottedTocLine(name string) (indent, numWidth float64, haveDims, dotted bool) {
	m := e.eq["l@"+name]
	if m == nil || m.kind != mMacro {
		return 0, 0, false, false
	}
	i := indexOfCS(m.body, "@dottedtocline")
	if i < 0 {
		return 0, 0, false, false
	}
	_, j, ok1 := braceGroupAt(m.body, i+1) // the level, which the entry already carries
	g2, k, ok2 := braceGroupAt(m.body, j)
	g3, _, ok3 := braceGroupAt(m.body, k)
	if !ok1 || !ok2 || !ok3 {
		return 0, 0, false, true
	}
	indent, okA := emOfToks(g2)
	numWidth, okB := emOfToks(g3)
	return indent, numWidth, okA && okB, true
}

// indexOfCS returns the index of the first control-sequence token named name,
// or -1.
func indexOfCS(ts []tok, name string) int {
	for i, t := range ts {
		if t.cs == name {
			return i
		}
	}
	return -1
}

// braceGroupAt reads the balanced {…} group starting at or after from, skipping
// spaces before it, and returns its contents and the index just past its '}'.
func braceGroupAt(ts []tok, from int) ([]tok, int, bool) {
	i := from
	for i < len(ts) && ts[i].cs == "" && ts[i].cat == catSpace {
		i++
	}
	if i >= len(ts) || ts[i].cs != "" || ts[i].cat != catBegin {
		return nil, from, false
	}
	depth, start := 1, i+1
	for i++; i < len(ts); i++ {
		if ts[i].cs != "" {
			continue
		}
		switch ts[i].cat {
		case catBegin:
			depth++
		case catEnd:
			if depth--; depth == 0 {
				return ts[start:i], i + 1, true
			}
		}
	}
	return nil, from, false
}

// emOfToks reads a "<number>em" dimension written as character tokens. It accepts
// only em, the unit every class writes these two lengths in; anything else is
// reported unreadable rather than silently converted at the wrong font size.
func emOfToks(ts []tok) (float64, bool) {
	var sb strings.Builder
	for _, t := range ts {
		if t.cs != "" {
			return 0, false
		}
		if t.cat != catSpace {
			sb.WriteRune(t.ch)
		}
	}
	s := sb.String()
	if !strings.HasSuffix(s, "em") {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "em"), 64)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// emitTOCEntryTokens appends one line per entry to b, shaped by what the loaded
// class says about it (see tocShapeFor): an empty box for the indent, the number
// left-aligned in a box of the level's width, the title, then either a dot leader
// or plain \hfil, and last the page number right-aligned in a \@pnumwidth box.
// Shared by \tableofcontents and \@starttoc.
func (e *Engine) emitTOCEntryTokens(b *tocTokens, entries []tocEntry) {
	for _, en := range entries {
		shape := e.tocShapeFor(en)
		if shape.vspaceBefore > 0 {
			// \l@chapter opens with \vskip 1.0em and article's \l@section with
			// \addvspace{1.0em}: the top entries stand apart from the ones under them.
			b.cs("addvspace")
			b.text(strconv.FormatFloat(shape.vspaceBefore, 'f', -1, 64) + "em")
		}
		b.cs("par")
		b.cs("noindent")
		// A group, so the bold of a top-level entry does not leak into the next one.
		b.begin()
		if shape.indent > 0 {
			// Indent with an empty hbox: unlike leading glue (\quad/\hspace), a box
			// is not discarded at the start of a broken line.
			b.boxTo(shape.indent)
			b.end()
		}
		if shape.bold {
			b.cs("bfseries")
		}
		// The number sits left-aligned in a box of the level's width, so the title
		// starts at the same place whether or not the entry carries a number.
		b.boxTo(shape.numWidth)
		if en.number != "" {
			b.text(en.number)
		}
		b.cs("hfil")
		b.end()
		b.text(en.title)
		b.cs("nobreak")
		if shape.dotted {
			b.cs("@tocdotfill")
		} else {
			// \hfill, where LaTeX writes \hfil: LaTeX also sets \parfillskip
			// -\@pnumwidth, so its \hfil is the only stretch on the line. Ours is
			// not — the paragraph's own \parfillskip (0pt plus 1fil) would share
			// the space with an \hfil and leave the page number mid-line. One
			// order higher outranks it and flushes the number to the margin.
			b.cs("hfill")
		}
		b.cs("nobreak")
		b.boxTo(tocPnumWidth)
		b.cs("hfil")
		if en.page > 0 {
			b.text(strconv.Itoa(en.page))
		}
		b.end()
		b.end()
		b.cs("par")
	}
	b.cs("par")
	b.cs("medskip")
}

// tocTokens builds a token list with the engine's live catcodes.
type tocTokens struct {
	e  *Engine
	ts []tok
}

// cs appends a control-sequence token.
func (b *tocTokens) cs(name string) { b.ts = append(b.ts, csTok(name)) }

// begin/end append a group-open/close character token.
func (b *tocTokens) begin() { b.ts = append(b.ts, chTok('{', catBegin)) }
func (b *tocTokens) end()   { b.ts = append(b.ts, chTok('}', catEnd)) }

// spacer appends an empty \hbox of the given width in points (see boxTo for why
// a box and not glue). The index uses it; the contents ladder works in em.
func (b *tocTokens) spacer(pt int) {
	b.cs("hbox")
	b.text("to " + strconv.Itoa(pt) + "pt")
	b.begin()
	b.end()
}

// boxTo opens "\hbox to <em>em{": the caller appends the box's content and closes
// it with end(). An empty one is a fixed, non-discardable horizontal space usable
// at the start of a line, where glue would be dropped by the line breaker.
func (b *tocTokens) boxTo(em float64) {
	b.cs("hbox")
	b.text("to " + strconv.FormatFloat(em, 'f', -1, 64) + "em")
	b.begin()
}

// text appends the runes of s as character tokens, each with its live catcode
// (so letters remain letters, spaces remain spaces, digits/punctuation other).
func (b *tocTokens) text(s string) {
	for _, r := range s {
		b.ts = append(b.ts, chTok(r, b.e.catOf(r)))
	}
}

// finalizeTOCPages fills in each recorded entry's page from the pages the
// auxiliary run assembled. It must be called after the aux Run, before the
// entries are carried into the render pass. See the file header for the
// (documented, never-fabricated) page-number approximation this represents.
func (e *Engine) finalizeTOCPages() {
	for i := range e.tocEntries {
		e.tocEntries[i].page = e.pageOfIndex(e.tocEntries[i].marker)
	}
}

// pageOfIndex returns the 1-based page number the main-vertical-list node at
// index idx falls on, using the same cost-based page breaking as Pages() so the
// numbers agree with the rendered output. An index past the last page reports
// the last page. It never returns 0 for a document with any content.
func (e *Engine) pageOfIndex(idx int) int {
	list := e.mvl
	pageNo := 0
	for start := 0; start < len(list); {
		end := e.findPageBreak(list, start)
		if len(trimTrailingGlue(list[start:end])) > 0 {
			pageNo++
		}
		if idx < end {
			if pageNo == 0 {
				pageNo = 1
			}
			return pageNo
		}
		next := skipDiscardable(list, end)
		if end > next-1 {
			next = end + 1
		}
		start = next
		if end == len(list) {
			break
		}
	}
	if pageNo == 0 {
		pageNo = 1
	}
	return pageNo
}
