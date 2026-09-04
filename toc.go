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

import "strconv"

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

// emitTOCEntryTokens appends one dotted line per entry to b (see emitTOCList for
// the line shape); shared by \tableofcontents and \@starttoc.
func (e *Engine) emitTOCEntryTokens(b *tocTokens, entries []tocEntry) {
	for _, en := range entries {
		b.cs("par")
		b.cs("noindent")
		if en.level > 1 {
			// Indent nested entries with an empty hbox spacer: unlike leading glue
			// (\quad/\hspace), a box is not discarded at the start of a broken line.
			b.spacer((en.level - 1) * 18)
		}
		if en.number != "" {
			b.text(en.number)
			b.cs("quad")
		}
		b.text(en.title)
		b.cs("dotfill")
		if en.page > 0 {
			b.text(strconv.Itoa(en.page))
		}
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

// spacer appends an empty \hbox of the given width in points — a fixed,
// non-discardable horizontal space usable at the start of a line (where glue
// would be dropped by the line breaker).
func (b *tocTokens) spacer(pt int) {
	b.cs("hbox")
	b.text("to " + strconv.Itoa(pt) + "pt")
	b.begin()
	b.end()
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
