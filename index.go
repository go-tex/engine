// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements LaTeX's index mechanism (\makeindex / \index / \printindex)
// through the same two-pass, .aux-style machinery the engine already uses for
// \label/\ref (crossref.go) and \tableofcontents (toc.go). On the first
// (auxiliary) pass every \index{term} runs the \index primitive, which — when
// index collection has been switched on by \makeindex — records an indexEntry
// (the raw term and the main-vertical-list index where the material landed).
// After the aux pass the page each entry falls on is computed from the assembled
// pages (finalizeIndexPages), and the entry table is carried into the render
// pass — exactly like the label and TOC tables — where \printindex typesets a
// sorted, grouped "Index".
//
// Semantics (a pragmatic subset of makeidx):
//
//   - \makeindex enables collection. When it has not run, \index still reads and
//     discards its argument but records nothing (a silent no-op, matching LaTeX).
//   - \index{term} records term with the page it lands on (the page is assigned
//     on the aux pass, the same multi-pass approximation the TOC lives with; see
//     toc.go's header). Two sub-syntaxes are honoured inside term:
//       * '!' separates levels: \index{animal!cat} nests "cat" under "animal".
//       * '@' gives a sort key: \index{alpha@display} sorts by "alpha" and
//         prints "display". The display part is typeset as literal source text —
//         embedded math/formatting (e.g. "$\alpha$") is NOT re-rendered, it is
//         printed verbatim. This is the documented simplification of '@'.
//     The page-range/format syntax '|' is NOT interpreted: a '|' and anything
//     after it is treated as ordinary term text (also documented-simplified).
//   - \printindex typesets a single-column, sorted, grouped index: an "Index"
//     heading, then main entries flush-left and subentries indented, each
//     followed by its page number(s). Entries are sorted case-insensitively by
//     sort key. A (term, page) pair is de-duplicated; a term appearing on several
//     pages lists all of them, sorted, e.g. "1, 3, 5".
//
// Page numbers are never fabricated: an entry that could not be placed reports
// page 0 and is printed without a number, exactly as toc.go does.

import (
	"sort"
	"strconv"
	"strings"
)

// indexEntry is one recorded \index call. term is the raw term as written
// (e.g. "animal!cat" or "alpha@display"), preserved for inspection and parsed at
// print time into levels/sort-keys. marker is len(mvl) at the moment \index ran,
// from which page is derived after the aux pass (see finalizeIndexPages).
type indexEntry struct {
	term   string
	marker int
	page   int
}

// doMakeIndex implements \makeindex: it switches index collection on. Until it
// runs, \index is a silent no-op (as in LaTeX, where \index without \makeindex
// writes to no .idx file).
func (e *Engine) doMakeIndex() {
	e.indexEnabled = true
}

// doIndex implements \index{term}: it always reads and discards the {term} group
// (so the argument never leaks into the typeset text), and — only when
// collection is enabled — records an indexEntry with the term and the current
// main-vertical-list position. A missing or malformed group records nothing and
// does not panic.
func (e *Engine) doIndex() {
	term := e.readIndexTerm()
	if !e.indexEnabled || term == "" {
		return
	}
	e.indexEntries = append(e.indexEntries, indexEntry{
		term:   term,
		marker: len(e.mvl),
	})
}

// readIndexTerm reads a {…} group and returns its content as a raw string with
// control sequences preserved (so a sort key like "alpha@\alpha" survives). A
// missing group yields "" and leaves the input untouched.
func (e *Engine) readIndexTerm() string {
	e.skipOptSpace()
	t, ok := e.getNext()
	if !ok || !(t.cat == catBegin && !t.cs_) {
		if ok {
			e.back(t)
		}
		return ""
	}
	return trimSpaces(e.toksToString(e.grabGroup()))
}

// indexList returns the entries to typeset, preferring the table carried from the
// aux pass (indexSource, with page numbers resolved) and falling back to whatever
// the current run has collected so far — the same fallback tocList uses, so a
// single-engine run still renders whatever preceded \printindex.
func (e *Engine) indexList() []indexEntry {
	if e.indexSource != nil {
		return e.indexSource
	}
	return e.indexEntries
}

// finalizeIndexPages fills in each recorded entry's page from the pages the
// auxiliary run assembled. It must be called after the aux Run, before the
// entries are carried into the render pass — mirroring finalizeTOCPages.
func (e *Engine) finalizeIndexPages() {
	for i := range e.indexEntries {
		e.indexEntries[i].page = e.pageOfIndex(e.indexEntries[i].marker)
	}
}

// idxNode is one node of the index tree: a level of one or more entries sharing a
// sort key. display is the text typeset; pages is the set of pages on which the
// term at this exact path was indexed; children holds nested (sub)entries keyed
// by their lower-cased sort key.
type idxNode struct {
	display  string
	pages    map[int]struct{}
	children map[string]*idxNode
	order    []string // child keys, in insertion order (sorted at print time)
}

func newIdxNode() *idxNode {
	return &idxNode{pages: map[int]struct{}{}, children: map[string]*idxNode{}}
}

// child returns (creating if needed) the child for the given sort key/display,
// remembering first-seen display and insertion order.
func (n *idxNode) child(key, display string) *idxNode {
	c, ok := n.children[key]
	if !ok {
		c = newIdxNode()
		c.display = display
		n.children[key] = c
		n.order = append(n.order, key)
	}
	return c
}

// parseIndexTerm splits a raw term into its levels ("a!b!c") and, per level,
// separates an optional "sort@display" key. Empty levels are dropped so a stray
// "!" or "a!" cannot create a blank entry. It returns parallel (sortKey, display)
// slices; a nil result means the term carried no usable level.
func parseIndexTerm(term string) (keys, displays []string) {
	for _, part := range strings.Split(term, "!") {
		part = trimSpaces(part)
		if part == "" {
			continue
		}
		sortKey, display := part, part
		if at := strings.Index(part, "@"); at >= 0 {
			sortKey = trimSpaces(part[:at])
			display = trimSpaces(part[at+1:])
			if sortKey == "" {
				sortKey = display
			}
			if display == "" {
				display = sortKey
			}
		}
		keys = append(keys, sortKey)
		displays = append(displays, display)
	}
	return keys, displays
}

// buildIndexTree folds every entry into a tree keyed by lower-cased sort key, so
// duplicate (term, page) pairs collapse (a set of pages per exact path) and the
// same term on several pages accumulates all of them.
func buildIndexTree(entries []indexEntry) *idxNode {
	root := newIdxNode()
	for _, en := range entries {
		keys, displays := parseIndexTerm(en.term)
		if len(keys) == 0 {
			continue
		}
		cur := root
		for i, k := range keys {
			cur = cur.child(strings.ToLower(k), displays[i])
		}
		if en.page > 0 {
			cur.pages[en.page] = struct{}{}
		}
	}
	return root
}

// pagesString renders a page set as a sorted, de-duplicated, comma-separated
// list (e.g. "1, 3, 5"). An empty set yields "".
func pagesString(set map[int]struct{}) string {
	if len(set) == 0 {
		return ""
	}
	ps := make([]int, 0, len(set))
	for p := range set {
		ps = append(ps, p)
	}
	sort.Ints(ps)
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ", ")
}

// doPrintIndex implements \printindex: an "Index" heading followed by the sorted,
// grouped entries. It prefers the aux-pass table (with page numbers) and falls
// back to the live entries. An empty index still prints the heading (as in LaTeX).
func (e *Engine) doPrintIndex() {
	tree := buildIndexTree(e.indexList())
	var b tocTokens
	b.e = e
	// Heading, styled like \section*{Index}.
	b.cs("par")
	b.cs("medskip")
	b.cs("noindent")
	b.begin()
	b.cs("Large")
	b.cs("bf")
	b.text("Index")
	b.end()
	b.cs("par")
	b.cs("nobreak")
	b.cs("smallskip")
	e.emitIndexNodes(&b, tree, 0)
	b.cs("par")
	b.cs("medskip")
	e.push(b.ts)
}

// emitIndexNodes appends the token list for one level of the index tree, sorted
// case-insensitively by sort key, then recurses into subentries with one 18pt
// indentation step per level. A node with no direct pages (a heading that only
// groups subentries) prints its display alone; otherwise a \dotfill leader runs
// to the right-flushed page list.
func (e *Engine) emitIndexNodes(b *tocTokens, n *idxNode, depth int) {
	keys := append([]string(nil), n.order...)
	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		c := n.children[k]
		b.cs("par")
		b.cs("noindent")
		if depth > 0 {
			// A box spacer (not glue) survives at the start of a broken line.
			b.spacer(depth * 18)
		}
		b.text(c.display)
		if ps := pagesString(c.pages); ps != "" {
			b.cs("dotfill")
			b.text(ps)
		}
		b.cs("par")
		e.emitIndexNodes(b, c, depth+1)
	}
}
