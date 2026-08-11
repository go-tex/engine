// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

// This file implements a pragmatic BibTeX-style bibliography for the mini-LaTeX
// layer. It is deliberately not a byte-perfect reimplementation of BibTeX and its
// .bst styles; it models the common case well enough for real documents.
//
// Pipeline
//
//	\cite{key} / \citep{key} / \citet{key}   record key as "cited"
//	\nocite{key}   records key without typesetting; \nocite{*} means "all entries"
//	\bibliographystyle{plain}   accepted (only a plain-ish style is modelled)
//	\bibliography{refs}   reads refs.bib, parses it, keeps the cited (or all, for
//	                      \nocite{*}) entries, sorts them plain-style (alphabetical
//	                      by first author, then year, then key), and splices a
//	                      \begin{thebibliography}…\end{thebibliography} block back
//	                      into the input so it renders through the existing
//	                      hanging-indent numbered list (see latex.go). \bibitem
//	                      numbers each entry and stores that number under its key,
//	                      so a \cite anywhere resolves to "[n]" on the second pass —
//	                      exactly the .aux forward-reference mechanism (see api.go).
//
// Supported .bib subset (documented limitations)
//
//   - Entry types: @article, @book, @inproceedings, @incollection, @techreport,
//     @phdthesis, @mastersthesis, @misc, and any other type (formatted generically).
//   - @string{k = "v"} definitions are collected and expanded when a bare word in a
//     later value matches; @comment and @preamble are skipped. Case-insensitive
//     entry-type and field names.
//   - Field values may be {braced} (nested braces balanced), "quoted" (nested
//     braces balanced), or a bare word/number; pieces joined with '#' are
//     concatenated (BibTeX string concatenation).
//   - Unknown fields are ignored. A malformed entry (unbalanced brace, junk) is
//     skipped without a panic; parsing resumes at the next '@'.
//   - Author names: "Last, First" is reordered to "First Last"; multiple authors
//     are joined with ", " and a final " and ". No von/Jr particle handling and no
//     initials abbreviation (a plain, readable approximation, not bst-exact).
//   - Ordering: plain style — alphabetical by first author surname; numbering
//     follows that order. (Not citation-order/unsrt.)
//   - Field values are emitted as LaTeX source, so \'e, {LaTeX}, ~ etc. work; a
//     value must be valid LaTeX (as in real BibTeX).
//   - No filesystem in a wasm build: \bibliography needs a real file (server/CLI),
//     exactly like \input. Passing .bib content inline is out of scope.

import (
	"os"
	"sort"
	"strings"
)

// bibEntry is one parsed .bib record. typ and field names are lowercased.
type bibEntry struct {
	typ    string            // entry type without '@', e.g. "article"
	key    string            // citation key
	fields map[string]string // field name → value
}

// field returns the value of a field (already lowercased name), or "" if absent.
func (b bibEntry) field(name string) string { return b.fields[name] }

// ── parser ──────────────────────────────────────────────────────────────────

// bibParser walks a .bib source with a rune cursor, tolerant of malformed input.
type bibParser struct {
	src     []rune
	pos     int
	strings map[string]string // @string definitions, for bare-word expansion
}

// parseBib parses .bib source into entries in file order. It never panics on
// malformed input: a bad entry is skipped and parsing resumes at the next '@'.
func parseBib(src string) []bibEntry {
	p := &bibParser{src: []rune(src), strings: map[string]string{}}
	var out []bibEntry
	for {
		if !p.seekAt() {
			break
		}
		e, ok := p.parseEntry()
		if ok && e.key != "" {
			out = append(out, e)
		}
	}
	return out
}

// seekAt advances the cursor to just past the next '@', returning false at EOF.
func (p *bibParser) seekAt() bool {
	for p.pos < len(p.src) {
		if p.src[p.pos] == '@' {
			p.pos++
			return true
		}
		p.pos++
	}
	return false
}

// parseEntry parses one @type{...} record whose '@' has already been consumed. It
// handles @string / @comment / @preamble specially. ok is false when the record
// is malformed (the caller then resumes at the next '@').
func (p *bibParser) parseEntry() (bibEntry, bool) {
	typ := strings.ToLower(p.readName())
	if typ == "" {
		return bibEntry{}, false
	}
	p.skipSpace()
	if p.pos >= len(p.src) {
		return bibEntry{}, false
	}
	open := p.peek()
	if open != '{' && open != '(' {
		return bibEntry{}, false
	}
	close := byte('}')
	if open == '(' {
		close = ')'
	}
	p.pos++ // consume the opening delimiter
	switch typ {
	case "comment", "preamble":
		p.skipBalanced(rune(close))
		return bibEntry{}, false
	case "string":
		p.parseString(rune(close))
		return bibEntry{}, false
	}
	// A regular entry: key up to the first comma or the closing delimiter.
	key := strings.TrimSpace(p.readUntil(',', rune(close)))
	e := bibEntry{typ: typ, key: key, fields: map[string]string{}}
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return e, key != "" // tolerate a truncated (unclosed) entry
		}
		if p.peek() == rune(close) {
			p.pos++
			break
		}
		name := strings.ToLower(strings.TrimSpace(p.readName()))
		p.skipSpace()
		if p.peek() != '=' {
			// Junk we can't interpret: skip to the next comma or the closing brace.
			p.readUntil(',', rune(close))
			if p.pos < len(p.src) && p.peek() == ',' {
				p.pos++
			}
			continue
		}
		p.pos++ // consume '='
		val := p.readValue(rune(close))
		if name != "" {
			e.fields[name] = val
		}
		p.skipSpace()
		if p.pos < len(p.src) && p.peek() == ',' {
			p.pos++
		}
	}
	return e, key != ""
}

// parseString parses a @string{ name = "value" } definition into p.strings.
func (p *bibParser) parseString(close rune) {
	p.skipSpace()
	name := strings.ToLower(strings.TrimSpace(p.readName()))
	p.skipSpace()
	if p.peek() == '=' {
		p.pos++
		val := p.readValue(close)
		if name != "" {
			p.strings[name] = val
		}
	}
	p.skipBalanced(close)
}

// readValue reads one field value: a {braced} group, a "quoted" string, or a bare
// word (a number, or a @string macro name that is expanded). Pieces joined by '#'
// are concatenated.
func (p *bibParser) readValue(close rune) string {
	var b strings.Builder
	for {
		p.skipSpace()
		b.WriteString(p.readValuePiece(close))
		p.skipSpace()
		if p.pos < len(p.src) && p.peek() == '#' {
			p.pos++
			continue
		}
		return b.String()
	}
}

// readValuePiece reads a single value piece (no concatenation).
func (p *bibParser) readValuePiece(close rune) string {
	if p.pos >= len(p.src) {
		return ""
	}
	switch p.peek() {
	case '{':
		p.pos++
		return p.readBraced()
	case '"':
		p.pos++
		return p.readQuoted()
	default:
		word := p.readBareWord(close)
		if v, ok := p.strings[strings.ToLower(word)]; ok {
			return v
		}
		return word
	}
}

// readBraced reads to the matching '}' (its opening '{' already consumed),
// balancing nested braces, and returns the inner text.
func (p *bibParser) readBraced() string {
	depth := 1
	start := p.pos
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				s := string(p.src[start:p.pos])
				p.pos++
				return s
			}
		}
		p.pos++
	}
	return string(p.src[start:p.pos]) // unbalanced: return what we have
}

// readQuoted reads to the closing '"' (its opening quote already consumed) at
// brace depth 0, so a quote inside {…} does not terminate the value.
func (p *bibParser) readQuoted() string {
	depth := 0
	start := p.pos
	for p.pos < len(p.src) {
		switch p.src[p.pos] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '"':
			if depth == 0 {
				s := string(p.src[start:p.pos])
				p.pos++
				return s
			}
		}
		p.pos++
	}
	return string(p.src[start:p.pos]) // unterminated: return what we have
}

// readBareWord reads an unquoted value (number or macro name), stopping at a
// space, '#', ',' or the closing delimiter.
func (p *bibParser) readBareWord(close rune) string {
	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if r == '#' || r == ',' || r == close || isBibSpace(r) {
			break
		}
		p.pos++
	}
	return strings.TrimSpace(string(p.src[start:p.pos]))
}

// readName reads an identifier (entry type, field name, or key): any run that is
// not a space or a BibTeX delimiter.
func (p *bibParser) readName() string {
	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		if isBibSpace(r) || r == '{' || r == '}' || r == '(' || r == ')' ||
			r == '=' || r == ',' || r == '#' || r == '"' || r == '@' {
			break
		}
		p.pos++
	}
	return string(p.src[start:p.pos])
}

// readUntil reads up to (not past) the first of the given stop runes.
func (p *bibParser) readUntil(stops ...rune) string {
	start := p.pos
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		for _, s := range stops {
			if r == s {
				return string(p.src[start:p.pos])
			}
		}
		p.pos++
	}
	return string(p.src[start:p.pos])
}

// skipBalanced consumes runes until the given close delimiter at brace depth 0.
func (p *bibParser) skipBalanced(close rune) {
	depth := 0
	for p.pos < len(p.src) {
		r := p.src[p.pos]
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && r == close {
			p.pos++
			return
		}
		p.pos++
	}
}

// skipSpace advances over ASCII whitespace.
func (p *bibParser) skipSpace() {
	for p.pos < len(p.src) && isBibSpace(p.src[p.pos]) {
		p.pos++
	}
}

// peek returns the current rune (call sites guard p.pos < len).
func (p *bibParser) peek() rune { return p.src[p.pos] }

func isBibSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

// ── name & entry formatting (plain style) ────────────────────────────────────

// formatAuthors turns a BibTeX "and"-separated author field into a readable list,
// e.g. `Knuth, Donald E. and Lamport, Leslie` → "Donald E. Knuth and Leslie Lamport".
func formatAuthors(field string) string {
	names := splitAuthors(field)
	for i, n := range names {
		names[i] = formatName(n)
	}
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// splitAuthors splits on the delimiter " and " (case-insensitive), trimming each.
func splitAuthors(field string) []string {
	var out []string
	for _, part := range splitAndSep(field) {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// splitAndSep splits on a whitespace-delimited "and" token (BibTeX's separator).
func splitAndSep(field string) []string {
	words := strings.Fields(field)
	var parts []string
	var cur []string
	for _, w := range words {
		if strings.EqualFold(w, "and") {
			parts = append(parts, strings.Join(cur, " "))
			cur = nil
			continue
		}
		cur = append(cur, w)
	}
	parts = append(parts, strings.Join(cur, " "))
	return parts
}

// formatName reorders a single "Last, First" name to "First Last"; a name without
// a comma is returned unchanged.
func formatName(name string) string {
	if i := strings.IndexByte(name, ','); i >= 0 {
		last := strings.TrimSpace(name[:i])
		first := strings.TrimSpace(name[i+1:])
		if first == "" {
			return last
		}
		return first + " " + last
	}
	return strings.TrimSpace(name)
}

// surname returns the family name of the first author, used for sorting and for
// \citet's author label. "Last, First" → "Last"; "First Last" → "Last".
func surname(authorField string) string {
	names := splitAuthors(authorField)
	if len(names) == 0 {
		return ""
	}
	n := names[0]
	if i := strings.IndexByte(n, ','); i >= 0 {
		return strings.TrimSpace(n[:i])
	}
	fields := strings.Fields(n) // names[0] is a trimmed non-empty run ⇒ at least one field
	return fields[len(fields)-1]
}

// shortAuthor is \citet's inline author label: the first author's surname, with
// "et al." appended when there is more than one author.
func shortAuthor(authorField string) string {
	s := surname(authorField)
	if s == "" {
		return ""
	}
	if len(splitAuthors(authorField)) > 1 {
		return s + " et al."
	}
	return s
}

// formatEntry renders a plain-style reference for one entry. Missing fields are
// skipped; the result always ends with a period.
func formatEntry(b bibEntry) string {
	var parts []string
	add := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			parts = append(parts, s)
		}
	}
	add(formatAuthors(b.field("author")))
	add(b.field("title"))
	switch b.typ {
	case "book":
		add(joinNonEmpty(", ", b.field("publisher"), b.field("year")))
	case "inproceedings", "incollection":
		bt := b.field("booktitle")
		if bt != "" {
			bt = "In " + bt
		}
		add(joinNonEmpty(", ", bt, b.field("publisher"), b.field("year")))
	case "techreport":
		add(joinNonEmpty(", ", b.field("institution"), b.field("number"), b.field("year")))
	case "phdthesis", "mastersthesis":
		add(joinNonEmpty(", ", b.field("school"), b.field("year")))
	case "article":
		vol := b.field("journal")
		if v := b.field("volume"); v != "" {
			vol = joinNonEmpty(" ", vol, v)
		}
		add(joinNonEmpty(", ", vol, b.field("pages"), b.field("year")))
	default:
		add(joinNonEmpty(", ", b.field("howpublished"), b.field("year")))
	}
	if b.field("note") != "" {
		add(b.field("note"))
	}
	out := strings.Join(parts, ". ")
	if out == "" {
		return ""
	}
	if !strings.HasSuffix(out, ".") {
		out += "."
	}
	return out
}

// joinNonEmpty joins the non-empty pieces with sep.
func joinNonEmpty(sep string, pieces ...string) string {
	var kept []string
	for _, s := range pieces {
		if s = strings.TrimSpace(s); s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, sep)
}

// ── engine integration ───────────────────────────────────────────────────────

// recordCites marks keys as cited so \bibliography emits them. It is called by
// \cite, \citep, \citet and \nocite.
func (e *Engine) recordCites(keys []string) {
	if e.citedKeys == nil {
		e.citedKeys = map[string]bool{}
	}
	for _, k := range keys {
		if k == "*" {
			e.nociteAll = true
			continue
		}
		if !e.citedKeys[k] {
			e.citeOrder = append(e.citeOrder, k)
		}
		e.citedKeys[k] = true
	}
}

// doNocite implements \nocite{k1,k2} and \nocite{*}: it records the keys (so they
// appear in the bibliography) without typesetting anything.
func (e *Engine) doNocite() { e.recordCites(splitComma(e.readBraceName())) }

// doBibliographyStyle accepts \bibliographystyle{name}. Only a plain-ish style is
// modelled; the name is recorded but does not change the output.
func (e *Engine) doBibliographyStyle() { e.bibStyle = e.readBraceName() }

// doCitep implements natbib's \citep{k}: a bracketed, comma-joined number list —
// identical output to \cite in this numeric model.
func (e *Engine) doCitep() { e.doCite() }

// doCitet implements natbib's \citet{k1,k2}: "Author [n]" per key, comma-joined,
// where Author is the first-author surname (plus "et al." for multiple authors),
// carried from the aux pass so a \citet before the bibliography resolves.
func (e *Engine) doCitet() {
	keys := splitComma(e.readBraceName())
	e.recordCites(keys)
	var parts []string
	for _, k := range keys {
		author := e.bibAuthor[k]
		num := e.refText(k)
		if author != "" {
			parts = append(parts, author+" ["+num+"]")
		} else {
			parts = append(parts, "["+num+"]")
		}
	}
	e.pushString(joinComma(parts))
}

// doBibliography implements \bibliography{base}: it reads base.bib, parses it,
// keeps the cited (or all, for \nocite{*}) entries, sorts them plain-style, and
// splices a \begin{thebibliography}…\end{thebibliography} block into the input so
// the existing bibitem rendering (latex.go) typesets it. Numbers are assigned in
// the emitted order and stored under each key (via \bibitem's \label), so \cite
// resolves on the second pass.
func (e *Engine) doBibliography() {
	base := e.readBraceName()
	if base == "" {
		return
	}
	file := base
	if !strings.HasSuffix(strings.ToLower(file), ".bib") {
		file += ".bib"
	}
	data, err := os.ReadFile(file)
	if err != nil {
		e.fail("bibliography file not found: " + file)
		return
	}
	entries := parseBib(string(data))
	included := e.selectEntries(entries)
	if len(included) == 0 {
		return
	}
	sortEntriesPlain(included)
	if e.bibAuthor == nil {
		e.bibAuthor = map[string]string{}
	}
	var b strings.Builder
	b.WriteString(`\begin{thebibliography}{99}`)
	for _, en := range included {
		e.bibAuthor[en.key] = shortAuthor(en.field("author"))
		b.WriteString(`\bibitem{`)
		b.WriteString(en.key)
		b.WriteString(`}`)
		b.WriteString(formatEntry(en))
	}
	b.WriteString(`\end{thebibliography}`)
	e.spliceSource(b.String())
}

// selectEntries returns the entries to print: all of them under \nocite{*},
// otherwise only those a \cite/\nocite named, preserving parse (sortable) order.
func (e *Engine) selectEntries(entries []bibEntry) []bibEntry {
	if e.nociteAll {
		return entries
	}
	var out []bibEntry
	for _, en := range entries {
		if e.citedKeys[en.key] {
			out = append(out, en)
		}
	}
	return out
}

// sortEntriesPlain orders entries alphabetically by first-author surname, then by
// year, then by key — the "plain" bst ordering.
func sortEntriesPlain(entries []bibEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		si := strings.ToLower(surname(entries[i].field("author")))
		sj := strings.ToLower(surname(entries[j].field("author")))
		if si != sj {
			return si < sj
		}
		if yi, yj := entries[i].field("year"), entries[j].field("year"); yi != yj {
			return yi < yj
		}
		return entries[i].key < entries[j].key
	})
}

// spliceSource inserts s into the base input at the current position, so it is
// tokenized next with the live category codes — the same mechanism as \input.
func (e *Engine) spliceSource(s string) {
	insert := []rune(s + " ") // TeX appends a space at end of an inserted file
	tail := append(insert, e.base[e.bpos:]...)
	e.base = append(e.base[:e.bpos:e.bpos], tail...)
	e.buildLineStarts()
}
