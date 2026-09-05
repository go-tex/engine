// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "strconv"

// This file implements LaTeX's cross-reference mechanism (\label / \ref and its
// variants). LaTeX resolves references through an auxiliary file written on one
// run and read on the next, so a forward \ref (used before its \label) resolves
// on the second pass. The engine mirrors this exactly: CompileToPDF /
// CompileToSVGPages compile the source twice when it contains \label, carrying the
// label table from the first pass into the second (see compileLabels).
//
// A \label records the current \@currentlabel — the reference text the last
// counter-stepping command (\section, \subsection, an equation, an enumerate item)
// left behind, fully expanded to a string. A \ref pushes that string back into the
// input so it typesets in place; an unresolved key yields "??" (as in LaTeX).

// setLabel stores key → the fully-expanded \@currentlabel.
func (e *Engine) doLabel() {
	key := e.readBraceName()
	if key == "" {
		return
	}
	val := e.toksToString(e.expandList([]tok{csTok("@currentlabel")}))
	if e.labels == nil {
		e.labels = map[string]string{}
	}
	e.labels[key] = val
	// Remember where in the main vertical list the label was declared, so that
	// once the document is broken into pages we can say which page it fell on
	// (finalizeLabelPages, for \pageref).
	if e.labelMarks == nil {
		e.labelMarks = map[string]int{}
	}
	e.labelMarks[key] = len(e.mvl)
	// Additionally freeze the reference type and title name for typed references
	// (\autoref / \nameref / \cref / \Cref); see typedrefs.go.
	e.recordRefMeta(key)
}

// finalizeLabelPages turns each label's recorded main-vertical-list position
// into a page number, once the run has broken the document into pages. It is
// the \pageref counterpart of finalizeTOCPages, and is called on the auxiliary
// pass so that the render pass can resolve a \pageref that precedes its \label.
func (e *Engine) finalizeLabelPages() {
	if len(e.labelMarks) == 0 {
		return
	}
	pages := make(map[string]int, len(e.labelMarks))
	for key, mark := range e.labelMarks {
		pages[key] = e.pageOfIndex(mark)
	}
	e.labelPages = pages
}

// doPageref implements \pageref: the number of the page its \label fell on.
// LaTeX resolves this through the .aux file; the engine resolves it from the
// label page table gathered by the auxiliary pass. An unknown key — or a single
// pass, which has no table yet — yields "??", LaTeX's unresolved marker.
func (e *Engine) doPageref() {
	key := e.readBraceName()
	if p, ok := e.labelPages[key]; ok && p > 0 {
		e.pushString(strconv.Itoa(p))
		return
	}
	e.pushString("??")
}

// finalizePages resolves every marker this run recorded — table of contents,
// index and \label — into the page it fell on, now that the document has been
// broken into pages.
func (e *Engine) finalizePages() {
	e.finalizeTOCPages()
	e.finalizeIndexPages()
	e.finalizeLabelPages()
}

// carryCrossRefs hands this engine everything a previous pass learned that a
// later one needs to resolve a reference made before its definition: the label
// table and its typed variants, the \citet author labels, and the table of
// contents and index entries with the pages that pass measured.
func (e *Engine) carryCrossRefs(prev *Engine) {
	e.labels = prev.labels
	e.refTypes = prev.refTypes
	e.refNames = prev.refNames
	e.labelPages = prev.labelPages
	e.bibAuthor = prev.bibAuthor // \citet author labels, gathered by the aux \bibliography
	e.tocSource = prev.tocEntries
	e.indexSource = prev.indexEntries
}

// crossRefsAgree reports whether the page numbers this run typeset are the ones
// it would collect from its own output — that is, whether the numbers have
// stopped moving and a further pass would change nothing. wasLabelPages is the
// label table the run was handed, captured before finalizePages replaced it.
func (e *Engine) crossRefsAgree(wasLabelPages map[string]int) bool {
	if len(wasLabelPages) != len(e.labelPages) {
		return false
	}
	for k, v := range wasLabelPages {
		if e.labelPages[k] != v {
			return false
		}
	}
	if len(e.tocSource) != len(e.tocEntries) {
		return false
	}
	for i := range e.tocSource {
		if e.tocSource[i].page != e.tocEntries[i].page {
			return false
		}
	}
	if len(e.indexSource) != len(e.indexEntries) {
		return false
	}
	for i := range e.indexSource {
		if e.indexSource[i].page != e.indexEntries[i].page {
			return false
		}
	}
	return true
}

// doRef pushes the recorded reference text for a key back into the input. An
// unknown or empty key yields "??", matching LaTeX's unresolved-reference marker.
func (e *Engine) doRef() {
	key := e.readBraceName()
	e.pushString(e.refText(key))
}

// doEqref is \eqref: like \ref but parenthesised, for equation numbers.
func (e *Engine) doEqref() {
	key := e.readBraceName()
	e.pushString("(" + e.refText(key) + ")")
}

// refText returns the stored reference text for key, or "??" when unresolved.
func (e *Engine) refText(key string) string {
	if v, ok := e.labels[key]; ok && v != "" {
		return v
	}
	return "??"
}

// doCite implements \cite{k1,k2,…}: the bracketed, comma-joined reference numbers
// of one or more \bibitem entries, e.g. "[1, 3]". Each \bibitem stores its number
// in the label table (via \label), so a \cite before the bibliography resolves on
// the second pass. \nocite is a no-op here (it only affects a real .bib run).
func (e *Engine) doCite() {
	keys := splitComma(e.readBraceName())
	e.recordCites(keys) // remember the keys so an auto \bibliography emits them
	for i, k := range keys {
		keys[i] = e.refText(k)
	}
	e.pushString("[" + joinComma(keys) + "]")
}

// needsTwoPass reports whether the source must be compiled twice: it defines a
// \label or a \bibitem whose number a \ref or \cite may use before the
// definition is seen, or it requests a \tableofcontents / \listoffigures /
// \listoftables, whose entries are only known once the whole document has run
// (LaTeX's .toc/.lof/.lot mechanism — see toc.go), or it requests a \printindex,
// whose \index entries and their page numbers are only known after a full run
// (LaTeX's .idx mechanism — see index.go).
func needsTwoPass(src []byte) bool {
	return indexOf(src, `\label`) >= 0 ||
		indexOf(src, `\bibitem`) >= 0 ||
		indexOf(src, `\pageref`) >= 0 || // needs the label page table (finalizeLabelPages)
		indexOf(src, `\bibliography`) >= 0 || // \cite forward-references an auto bibliography
		indexOf(src, `\tableofcontents`) >= 0 ||
		indexOf(src, `\listoffigures`) >= 0 ||
		indexOf(src, `\listoftables`) >= 0 ||
		indexOf(src, `\printindex`) >= 0
}

// splitComma splits a comma-separated key list, trimming spaces around each key.
func splitComma(s string) []string {
	var out []string
	start := 0
	flush := func(end int) {
		k := trimSpaces(s[start:end])
		if k != "" {
			out = append(out, k)
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			flush(i)
			start = i + 1
		}
	}
	flush(len(s))
	return out
}

// joinComma joins keys with ", ".
func joinComma(keys []string) string {
	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += k
	}
	return out
}

// trimSpaces drops leading and trailing ASCII spaces.
func trimSpaces(s string) string {
	i, j := 0, len(s)
	for i < j && s[i] == ' ' {
		i++
	}
	for j > i && s[j-1] == ' ' {
		j--
	}
	return s[i:j]
}

// indexOf is a tiny substring search (avoids importing bytes just for this).
func indexOf(hay []byte, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(hay); i++ {
		if string(hay[i:i+n]) == needle {
			return i
		}
	}
	return -1
}
