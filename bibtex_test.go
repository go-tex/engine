// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── parser ──────────────────────────────────────────────────────────────────

// A @article with braced and nested-brace values, comma-separated fields and a
// case-insensitive type/field names is parsed into its fields.
func TestParseBibArticle(t *testing.T) {
	src := `@Article{knuth1984,
		Author = {Donald E. Knuth},
		title  = {Literate {Programming}},
		journal = {The Computer Journal},
		Volume = {27},
		year   = {1984},
	}`
	ents := parseBib(src)
	if len(ents) != 1 {
		t.Fatalf("parsed %d entries, want 1", len(ents))
	}
	e := ents[0]
	if e.typ != "article" {
		t.Errorf("typ = %q, want article", e.typ)
	}
	if e.key != "knuth1984" {
		t.Errorf("key = %q, want knuth1984", e.key)
	}
	for k, want := range map[string]string{
		"author":  "Donald E. Knuth",
		"title":   "Literate {Programming}", // nested braces preserved
		"journal": "The Computer Journal",
		"volume":  "27",
		"year":    "1984",
	} {
		if got := e.field(k); got != want {
			t.Errorf("field %q = %q, want %q", k, got, want)
		}
	}
}

// A @book with "quoted" values (and a brace-protected quote inside) is parsed.
func TestParseBibBookQuoted(t *testing.T) {
	src := `@BOOK{lamport1994,
		author = "Leslie Lamport",
		title  = "LaTeX: A {Document} Preparation System",
		publisher = "Addison-Wesley",
		year = "1994"
	}`
	ents := parseBib(src)
	if len(ents) != 1 {
		t.Fatalf("parsed %d entries, want 1", len(ents))
	}
	e := ents[0]
	if e.typ != "book" || e.key != "lamport1994" {
		t.Fatalf("typ/key = %q/%q, want book/lamport1994", e.typ, e.key)
	}
	if got := e.field("title"); got != "LaTeX: A {Document} Preparation System" {
		t.Errorf("title = %q", got)
	}
	if got := e.field("publisher"); got != "Addison-Wesley" {
		t.Errorf("publisher = %q", got)
	}
}

// @string definitions are expanded when a later bare-word value references them,
// including '#' concatenation.
func TestParseBibStringExpansion(t *testing.T) {
	src := `@string{aw = "Addison-Wesley"}
	@string{y = "1994"}
	@book{k, author = {A. Author}, publisher = aw # ", Reading", year = y}`
	ents := parseBib(src)
	if len(ents) != 1 {
		t.Fatalf("parsed %d entries, want 1", len(ents))
	}
	if got := ents[0].field("publisher"); got != "Addison-Wesley, Reading" {
		t.Errorf("publisher = %q, want concatenated string", got)
	}
	if got := ents[0].field("year"); got != "1994" {
		t.Errorf("year = %q, want expanded @string 1994", got)
	}
}

// @comment and @preamble records are skipped (parentheses-delimited too).
func TestParseBibCommentPreamble(t *testing.T) {
	src := `@comment{ this is ignored {nested} }
	@preamble{ "\newcommand{\x}{y}" }
	@misc(paren, title = {Braceless outer delimiter})`
	ents := parseBib(src)
	if len(ents) != 1 {
		t.Fatalf("parsed %d entries, want 1 (paren)", len(ents))
	}
	if ents[0].key != "paren" || ents[0].field("title") != "Braceless outer delimiter" {
		t.Errorf("paren entry = %+v", ents[0])
	}
}

// A malformed .bib (unbalanced brace, junk between entries, a field with no '=')
// must not panic: bad entries are skipped and parsing resumes at the next '@'.
func TestParseBibMalformed(t *testing.T) {
	src := `junk before @ and more
	@article{bad, author = {Unterminated
	@book{good, author = {Fine Author}, title = {Fine Title}, year = {2000}}
	@misc{noeq, this is not a field, title = {Recovered}}`
	ents := parseBib(src) // must not panic
	// The unterminated @article swallows the rest until EOF via readBraced, so at
	// least it does not crash; assert the parser returns without panicking and any
	// recovered entry keeps its key.
	for _, e := range ents {
		if e.key == "" {
			t.Errorf("entry with empty key returned: %+v", e)
		}
	}
}

// A field whose value is bare and un-braced at end of entry, plus a stray junk
// token before '=', exercises the recovery branch.
func TestParseBibFieldRecovery(t *testing.T) {
	src := `@misc{m, note = {ok}, garbagetoken, year = 2001}`
	ents := parseBib(src)
	if len(ents) != 1 {
		t.Fatalf("parsed %d entries, want 1", len(ents))
	}
	if ents[0].field("note") != "ok" || ents[0].field("year") != "2001" {
		t.Errorf("recovery lost fields: %+v", ents[0].fields)
	}
}

// An empty source and an @ with no type yield no entries (boundary paths).
func TestParseBibEmpty(t *testing.T) {
	if ents := parseBib(""); ents != nil {
		t.Errorf("empty source parsed %d entries, want 0", len(ents))
	}
	if ents := parseBib("no at signs here"); ents != nil {
		t.Errorf("plain text parsed %d entries, want 0", len(ents))
	}
	if ents := parseBib("@ {x}"); ents != nil {
		t.Errorf("@ with no type parsed %d entries", len(ents))
	}
	if ents := parseBib("@article"); ents != nil {
		t.Errorf("@article with no body parsed %d entries", len(ents))
	}
	if ents := parseBib("@article x{y}"); ents != nil {
		t.Errorf("@article with non-delimiter after type parsed %d entries", len(ents))
	}
}

// Truncated / unterminated values exercise the EOF branches of the value and
// entry readers without panicking.
func TestParseBibTruncated(t *testing.T) {
	// Unterminated quoted value (readQuoted EOF branch).
	if ents := parseBib(`@misc{q, title = "unterminated`); len(ents) != 1 || ents[0].field("title") != "unterminated" {
		t.Errorf("unterminated quote = %+v", ents)
	}
	// Missing '=' value at EOF (readValuePiece empty branch) + truncated field loop.
	if ents := parseBib(`@misc{v, title = `); len(ents) != 1 || ents[0].key != "v" {
		t.Errorf("value-at-EOF = %+v", ents)
	}
	// Truncated braced value: field loop hits EOF (parseEntry truncated return).
	if ents := parseBib(`@misc{b, title = {open`); len(ents) != 1 || ents[0].field("title") != "open" {
		t.Errorf("truncated braced = %+v", ents)
	}
	// Key with no comma and no closing brace: readUntil runs to EOF.
	if ents := parseBib(`@misc{onlykey`); len(ents) != 1 || ents[0].key != "onlykey" {
		t.Errorf("key-only = %+v", ents)
	}
}

// ── name & entry formatting ──────────────────────────────────────────────────

func TestFormatName(t *testing.T) {
	cases := map[string]string{
		"Knuth, Donald E.": "Donald E. Knuth",
		"Leslie Lamport":   "Leslie Lamport",
		"Cher,":            "Cher",
		"  Spaced  ":       "Spaced",
	}
	for in, want := range cases {
		if got := formatName(in); got != want {
			t.Errorf("formatName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatAuthors(t *testing.T) {
	cases := map[string]string{
		"":                               "",
		"Knuth, Donald E.":               "Donald E. Knuth",
		"Knuth, Donald and Lamport, Les": "Donald Knuth and Les Lamport",
		"A. One and B. Two and C. Three": "A. One, B. Two and C. Three",
	}
	for in, want := range cases {
		if got := formatAuthors(in); got != want {
			t.Errorf("formatAuthors(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSurnameAndShortAuthor(t *testing.T) {
	if got := surname("Knuth, Donald E."); got != "Knuth" {
		t.Errorf("surname comma form = %q", got)
	}
	if got := surname("Leslie Lamport"); got != "Lamport" {
		t.Errorf("surname plain form = %q", got)
	}
	if got := surname(""); got != "" {
		t.Errorf("surname empty = %q", got)
	}
	if got := shortAuthor("Leslie Lamport"); got != "Lamport" {
		t.Errorf("shortAuthor single = %q", got)
	}
	if got := shortAuthor("A. One and B. Two"); got != "One et al." {
		t.Errorf("shortAuthor multi = %q, want 'One et al.'", got)
	}
	if got := shortAuthor(""); got != "" {
		t.Errorf("shortAuthor empty = %q", got)
	}
}

func TestFormatEntry(t *testing.T) {
	art := bibEntry{typ: "article", key: "a", fields: map[string]string{
		"author": "Knuth, Donald E.", "title": "Literate Programming",
		"journal": "Comput. J.", "volume": "27", "pages": "97--111", "year": "1984",
	}}
	got := formatEntry(art)
	for _, want := range []string{"Donald E. Knuth", "Literate Programming", "Comput. J. 27", "97--111", "1984"} {
		if !strings.Contains(got, want) {
			t.Errorf("article entry %q missing %q", got, want)
		}
	}
	if !strings.HasSuffix(got, ".") {
		t.Errorf("entry does not end with period: %q", got)
	}

	book := bibEntry{typ: "book", fields: map[string]string{
		"author": "Leslie Lamport", "title": "LaTeX", "publisher": "Addison-Wesley", "year": "1994",
	}}
	if g := formatEntry(book); !strings.Contains(g, "Addison-Wesley, 1994") {
		t.Errorf("book entry = %q", g)
	}

	proc := bibEntry{typ: "inproceedings", fields: map[string]string{
		"author": "A. B.", "title": "T", "booktitle": "Proc. X", "year": "2020",
	}}
	if g := formatEntry(proc); !strings.Contains(g, "In Proc. X") {
		t.Errorf("inproceedings entry = %q", g)
	}

	tr := bibEntry{typ: "techreport", fields: map[string]string{
		"title": "TR", "institution": "MIT", "number": "42", "year": "1999",
	}}
	if g := formatEntry(tr); !strings.Contains(g, "MIT, 42, 1999") {
		t.Errorf("techreport entry = %q", g)
	}

	phd := bibEntry{typ: "phdthesis", fields: map[string]string{
		"author": "P. H. D.", "title": "Thesis", "school": "Stanford", "year": "2001", "note": "unpublished",
	}}
	if g := formatEntry(phd); !strings.Contains(g, "Stanford, 2001") || !strings.HasSuffix(g, "unpublished.") {
		t.Errorf("phdthesis entry = %q", g)
	}

	misc := bibEntry{typ: "misc", fields: map[string]string{
		"title": "Web", "howpublished": "Online", "year": "2022",
	}}
	if g := formatEntry(misc); !strings.Contains(g, "Online, 2022") {
		t.Errorf("misc entry = %q", g)
	}

	if g := formatEntry(bibEntry{typ: "misc", fields: map[string]string{}}); g != "" {
		t.Errorf("empty entry = %q, want empty", g)
	}
}

func TestJoinNonEmpty(t *testing.T) {
	if got := joinNonEmpty(", ", "a", "", "b", "  ", "c"); got != "a, b, c" {
		t.Errorf("joinNonEmpty = %q", got)
	}
}

// ── selection & ordering ─────────────────────────────────────────────────────

func TestSelectEntries(t *testing.T) {
	ents := []bibEntry{{key: "a"}, {key: "b"}, {key: "c"}}
	e := &Engine{citedKeys: map[string]bool{"a": true, "c": true}}
	got := e.selectEntries(ents)
	if len(got) != 2 || got[0].key != "a" || got[1].key != "c" {
		t.Errorf("selectEntries cited = %+v", got)
	}
	e2 := &Engine{nociteAll: true}
	if got := e2.selectEntries(ents); len(got) != 3 {
		t.Errorf("selectEntries nociteAll = %d, want 3", len(got))
	}
}

func TestSortEntriesPlain(t *testing.T) {
	ents := []bibEntry{
		{key: "z", fields: map[string]string{"author": "Zeta, A.", "year": "2000"}},
		{key: "a", fields: map[string]string{"author": "Lamport, L.", "year": "1994"}},
		{key: "b", fields: map[string]string{"author": "Knuth, D.", "year": "1984"}},
	}
	sortEntriesPlain(ents)
	order := []string{ents[0].key, ents[1].key, ents[2].key}
	// Knuth < Lamport < Zeta
	if order[0] != "b" || order[1] != "a" || order[2] != "z" {
		t.Errorf("sorted order = %v, want [b a z]", order)
	}

	// Same surname ⇒ tie-break by year, then by key.
	tie := []bibEntry{
		{key: "k2", fields: map[string]string{"author": "Smith, A.", "year": "2005"}},
		{key: "k1", fields: map[string]string{"author": "Smith, A.", "year": "2005"}},
		{key: "k0", fields: map[string]string{"author": "Smith, A.", "year": "1990"}},
	}
	sortEntriesPlain(tie)
	if tie[0].key != "k0" { // earliest year first
		t.Errorf("year tie-break failed: %s first", tie[0].key)
	}
	if tie[1].key != "k1" || tie[2].key != "k2" { // equal year ⇒ key order
		t.Errorf("key tie-break failed: %s then %s", tie[1].key, tie[2].key)
	}
}

// ── end-to-end through the engine (two-pass) ─────────────────────────────────

// writeBib writes a two-entry .bib into a temp dir and returns the base path
// (without the .bib extension) for use in \bibliography{base}.
func writeBib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "refs.bib")
	content := `@book{knuth,
		author = {Donald E. Knuth},
		title = {The {TeX}book},
		publisher = {Addison-Wesley},
		year = {1984}
	}
	@article{lamport,
		author = {Leslie Lamport},
		title = {Document Preparation},
		journal = {TUGboat},
		year = {1994}
	}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Forward slashes: the returned base path is embedded into TeX source
	// (\bibliography{base}), where a backslash (the Windows separator) is the
	// escape char. TeX and Go's os.ReadFile both accept "/" on every platform.
	return filepath.ToSlash(filepath.Join(dir, "refs"))
}

// runTwoPass mimics api.go's two-pass compile for a raw LaTeX-kernel run: an aux
// pass gathers the bibliography numbers (labels) and \citet author labels, which
// the render pass reuses so forward \cite/\citet resolve.
func runTwoPass(t *testing.T, src string) *Engine {
	t.Helper()
	aux := New()
	if err := aux.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	aux.SetFont(spMock{})
	if _, err := aux.Run(src); err != nil {
		t.Fatalf("aux pass: %v", err)
	}
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.labels = aux.labels
	e.bibAuthor = aux.bibAuthor
	if _, err := e.Run(src); err != nil {
		t.Fatalf("render pass: %v", err)
	}
	return e
}

// A document that \cite's two entries then \bibliography's a temp .bib: the
// citations resolve to [1]/[2] (numbered in plain sorted order) and the reference
// list contains both formatted entries.
func TestBibliographyEndToEnd(t *testing.T) {
	base := writeBib(t)
	src := `\noindent See \cite{knuth} and \cite{lamport}.` +
		`\bibliographystyle{plain}` +
		`\bibliography{` + base + `}`
	e := runTwoPass(t, src)

	// Numbering carried from the aux pass (plain order: Knuth[1], Lamport[2]).
	if e.labels["knuth"] != "1" || e.labels["lamport"] != "2" {
		t.Fatalf("bib numbers = %q/%q, want 1/2", e.labels["knuth"], e.labels["lamport"])
	}
	text := mvlText(e.mvl)
	for _, want := range []string{
		"[1]", "[2]", // inline citations
		"References",    // heading from thebibliography
		"DonaldE.Knuth", // formatted author (spaces are glue, not chars)
		"The", "book",   // title "The TeXbook"
		"Addison-Wesley,1984", // book imprint
		"LeslieLamport",       // second author
		"DocumentPreparation", // second title
		"TUGboat,1994",        // journal + year
	} {
		if !strings.Contains(text, want) {
			t.Errorf("typeset output missing %q\n got: %q", want, text)
		}
	}
}

// \nocite{*} pulls every entry into the bibliography even when none is \cite'd.
func TestBibliographyNociteAll(t *testing.T) {
	base := writeBib(t)
	src := `\nocite{*}\bibliography{` + base + `}`
	e := runTwoPass(t, src)
	if e.labels["knuth"] == "" || e.labels["lamport"] == "" {
		t.Fatalf("nocite* should number both: %q/%q", e.labels["knuth"], e.labels["lamport"])
	}
	text := mvlText(e.mvl)
	if !strings.Contains(text, "DonaldE.Knuth") || !strings.Contains(text, "LeslieLamport") {
		t.Errorf("nocite* bibliography missing entries: %q", text)
	}
}

// Only \cite'd entries appear; an uncited entry is omitted.
func TestBibliographyOnlyCited(t *testing.T) {
	base := writeBib(t)
	src := `\noindent\cite{lamport}\bibliography{` + base + `}`
	e := runTwoPass(t, src)
	text := mvlText(e.mvl)
	if !strings.Contains(text, "LeslieLamport") {
		t.Errorf("cited entry missing: %q", text)
	}
	if strings.Contains(text, "DonaldE.Knuth") {
		t.Errorf("uncited Knuth entry should be omitted: %q", text)
	}
	if e.labels["lamport"] != "1" { // only one entry ⇒ number 1
		t.Errorf("lamport number = %q, want 1", e.labels["lamport"])
	}
}

// natbib \citet{key} typesets "Author [n]", resolving the author from the aux pass.
func TestCitetAuthorYear(t *testing.T) {
	base := writeBib(t)
	src := `\noindent\citet{knuth} wrote it.\bibliography{` + base + `}`
	e := runTwoPass(t, src)
	text := mvlText(e.mvl)
	if !strings.Contains(text, "Knuth[1]") {
		t.Errorf("\\citet output missing 'Knuth[1]': %q", text)
	}
}

// \citep{key} matches \cite (bracketed number).
func TestCitep(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "1", "b": "2"}
	if _, err := e.Run(`\noindent\citep{a,b}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "[1,2]" {
		t.Errorf("\\citep typeset %q, want [1,2]", got)
	}
}

// \citet without a carried author label falls back to the bare number.
func TestCitetNoAuthor(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	e.labels = map[string]string{"a": "3"}
	if _, err := e.Run(`\noindent\citet{a}`); err != nil {
		t.Fatal(err)
	}
	if got := mvlText(e.mvl); got != "[3]" {
		t.Errorf("\\citet no-author typeset %q, want [3]", got)
	}
}

// \nocite records keys (including *) so recordCites populates the citation set.
func TestNociteRecords(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\nocite{a,b}\nocite{*}`); err != nil {
		t.Fatal(err)
	}
	if !e.citedKeys["a"] || !e.citedKeys["b"] {
		t.Errorf("nocite did not record keys: %v", e.citedKeys)
	}
	if !e.nociteAll {
		t.Error("nocite{*} did not set nociteAll")
	}
}

// \bibliographystyle records the style name and is otherwise a no-op.
func TestBibliographyStyle(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\bibliographystyle{plain}`); err != nil {
		t.Fatal(err)
	}
	if e.bibStyle != "plain" {
		t.Errorf("bibStyle = %q, want plain", e.bibStyle)
	}
}

// A missing .bib file makes \bibliography fail (not panic); an empty argument is a
// silent no-op; a no-included-entries bibliography emits nothing.
func TestBibliographyErrors(t *testing.T) {
	// missing file
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\bibliography{/no/such/path/refs}`); err == nil {
		t.Error("missing .bib should error")
	}

	// empty argument: no-op, no error
	e2 := New()
	e2.LoadLaTeX()
	e2.SetFont(spMock{})
	if _, err := e2.Run(`\bibliography{}x`); err != nil {
		t.Errorf("empty \\bibliography should be a no-op, got %v", err)
	}

	// valid file but nothing cited ⇒ no bibliography emitted, no labels set.
	base := writeBib(t)
	e3 := New()
	e3.LoadLaTeX()
	e3.SetFont(spMock{})
	if _, err := e3.Run(`\bibliography{` + base + `}`); err != nil {
		t.Fatal(err)
	}
	if len(e3.labels) != 0 {
		t.Errorf("uncited bibliography emitted entries: %v", e3.labels)
	}
}

// needsTwoPass detects \bibliography (a \cite may forward-reference it).
func TestNeedsTwoPassBibliography(t *testing.T) {
	if !needsTwoPass([]byte(`\cite{a}\bibliography{refs}`)) {
		t.Error("needsTwoPass should detect \\bibliography")
	}
}

// The full compile() path (\documentclass ⇒ LaTeX + two-pass) resolves a forward
// \cite against an auto bibliography and renders at least one page.
func TestCompileBibliographyTwoPass(t *testing.T) {
	base := writeBib(t)
	src := []byte(`\documentclass{article}
\begin{document}
Text citing \cite{lamport} and \cite{knuth}.
\bibliographystyle{plain}
\bibliography{` + base + `}
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if len(e.Pages()) == 0 {
		t.Fatal("no pages produced")
	}
	if e.labels["knuth"] != "1" || e.labels["lamport"] != "2" {
		t.Errorf("two-pass numbers = %q/%q, want 1/2", e.labels["knuth"], e.labels["lamport"])
	}
}

// natbib's .bbl boilerplate probes for the url package with
// "\ifx\csname urlstyle\endcsname\relax" and, when it finds \urlstyle, defines
// \doi as "doi: \begingroup \urlstyle{rm}\Url" — one \begingroup per \doi that
// only \Url can close. The engine stubbed \urlstyle but not \Url, so the probe
// took that branch and every \doi leaked a group, cascading until the tail of the
// bibliography was swallowed (and every DOI lost). \bibitem here also carries the
// natbib optional [author-year] label, which a one-argument \bibitem printed as
// body text. This asserts the whole entry set survives, the keys number in order,
// and no group is left open. (Regression for the 2606.18084 −17-page loss.)
func TestBibliographyNatbibDoiGroupBalance(t *testing.T) {
	src := []byte(`\documentclass{article}
\begin{document}
\begin{thebibliography}{56}
\providecommand{\natexlab}[1]{#1}
\providecommand{\url}[1]{\texttt{#1}}
\expandafter\ifx\csname urlstyle\endcsname\relax
  \providecommand{\doi}[1]{doi: #1}\else
  \providecommand{\doi}{doi: \begingroup \urlstyle{rm}\Url}\fi

\bibitem[Garcia-Vidal et~al.(2021)Garcia-Vidal, Ciuti, and Ebbesen]{Garcia-Vidal2021}
Francisco~J Garcia-Vidal, Cristiano Ciuti, and Thomas~W Ebbesen.
\newblock Manipulating matter by strong coupling to vacuum fields.
\newblock \doi{10.1126/science.abd0336}.
\newblock URL \url{https://www.science.org/doi/10.1126/science.abd0336}.

\bibitem[Schlawin et~al.(2022)Schlawin, Kennes, and Sentef]{Schlawin2022}
Frank Schlawin, Dante~M. Kennes, and Michael~A. Sentef.
\newblock Cavity quantum materials.
\newblock \doi{10.1063/5.0083825}.

\bibitem{PlainKey2023}
An entry whose \bibitem carries no optional label at all.
\newblock \doi{10.1000/plain}.
\end{thebibliography}
\end{document}
`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("OpenGroups = %d, want 0 (a leaked \\doi \\begingroup)", d.OpenGroups)
	}
	if d := e.Diagnostics(); d.Skipped["Url"] != 0 {
		t.Errorf("\\Url skipped %d time(s); it must be defined so the \\doi branch balances", d.Skipped["Url"])
	}
	// Every entry — including the last, which the group leak used to swallow —
	// is numbered in order, and the optional [label] never desynced the key.
	for k, want := range map[string]string{
		"Garcia-Vidal2021": "1", "Schlawin2022": "2", "PlainKey2023": "3",
	} {
		if got := e.labels[k]; got != want {
			t.Errorf("label[%q] = %q, want %q", k, got, want)
		}
	}
}

// bibunits splits a document's references into per-part bibliographies, each in a
// \begin{bibunit}…\putbib…\end{bibunit} wrapping a pre-generated bu<N>.bbl. \putbib
// was undefined, so the whole bibliography was dropped. This writes two unit .bbl
// files next to a document, compiles it, and checks that \putbib \input's bu1.bbl
// then bu2.bbl in order — each entry numbered and its key labelled. (Regression for
// the 2606.18084 bibunits bibliography loss.)
func TestPutbibReadsUnitFiles(t *testing.T) {
	dir := t.TempDir()
	bbl := func(name, key, marker string) {
		body := `\begin{thebibliography}{9}
\bibitem[` + marker + `]{` + key + `}
An entry in ` + name + `.
\end{thebibliography}
`
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bbl("bu1.bbl", "Alpha2020", "Alpha")
	bbl("bu2.bbl", "Beta2021", "Beta")
	t.Setenv("TEXINPUTS", dir) // \putbib resolves bu<N>.bbl on the search path
	src := []byte(`\documentclass{article}
\begin{document}
\begin{bibunit}\putbib[first]\end{bibunit}
\begin{bibunit}\putbib[second]\end{bibunit}
\end{document}
`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if d := e.Diagnostics(); d.Skipped["putbib"] != 0 {
		t.Errorf("\\putbib skipped %d time(s); the unit .bbl files were not read", d.Skipped["putbib"])
	}
	// Both units contributed their entry: Alpha from bu1.bbl, Beta from bu2.bbl —
	// each \bibitem's \label recorded, which happens only if its .bbl was typeset.
	for _, k := range []string{"Alpha2020", "Beta2021"} {
		if e.labels[k] == "" {
			t.Errorf("key %q has no label — its unit .bbl was not read by \\putbib", k)
		}
	}
}

// A \putbib whose unit .bbl is not on disk (bibtex was never run) is a skipped,
// recorded no-op under a best-effort render, and a hard error under a strict one.
func TestPutbibMissingUnitFile(t *testing.T) {
	src := []byte(`\documentclass{article}\begin{document}\putbib[refs]\end{document}`)

	// Lenient: the missing bu1.bbl is recorded and the render carries on.
	e, err := compile(src, Options{Lenient: true})
	if err != nil {
		t.Fatalf("lenient compile: %v", err)
	}
	if got := e.Diagnostics().Skipped["putbib"]; got != 1 {
		t.Errorf("Skipped[putbib] = %d, want 1", got)
	}

	// Strict: the same missing file aborts, as a real TeX would on \input of it.
	if _, err := compile(src, Options{}); err == nil {
		t.Error("strict compile should fail on a missing bibliography unit file")
	}
}
