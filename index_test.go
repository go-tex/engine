// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \makeindex enables collection, after which each \index{term} records an entry
// with the raw term; without \makeindex nothing is recorded (the argument is
// still gobbled, so it never leaks into the text).
func TestIndexEntriesRecorded(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\makeindex
apple\index{apple}
banana\index{fruit!banana}
cherry\index{cherry}`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	wantTerms := []string{"apple", "fruit!banana", "cherry"}
	if len(e.indexEntries) != len(wantTerms) {
		t.Fatalf("recorded %d entries, want %d: %+v", len(e.indexEntries), len(wantTerms), e.indexEntries)
	}
	for i, w := range wantTerms {
		if g := e.indexEntries[i].term; g != w {
			t.Errorf("entry %d term = %q, want %q", i, g, w)
		}
	}
}

// \index without a preceding \makeindex records nothing but still consumes its
// argument, so the braced term never appears in the typeset output.
func TestIndexWithoutMakeIndex(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt visible\index{secret}text`); err != nil {
		t.Fatal(err)
	}
	if len(e.indexEntries) != 0 {
		t.Fatalf("no \\makeindex, but %d entries recorded: %+v", len(e.indexEntries), e.indexEntries)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	got := b.String()
	if strings.Contains(got, "secret") {
		t.Errorf("index term leaked into output: %q", got)
	}
	if !strings.Contains(got, "visible") || !strings.Contains(got, "text") {
		t.Errorf("surrounding text lost: %q", got)
	}
}

// The full two-pass compile carries aux-pass entries (with resolved pages) into
// the render pass, where \printindex typesets an "Index" heading and the sorted
// entries with page numbers; a '!' subentry nests under its main entry.
func TestIndexPrintTwoPass(t *testing.T) {
	src := []byte(`\documentclass{article}
\makeindex
\begin{document}
Zebra\index{zebra} and apple\index{apple}.
\newpage
More about the apple\index{apple} and animal\index{animal!cat}.
\printindex
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(e.indexSource) == 0 {
		t.Fatalf("render engine carried no index entries")
	}
	for _, en := range e.indexSource {
		if en.page < 1 {
			t.Errorf("entry %q has non-positive page %d", en.term, en.page)
		}
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	full := b.String()
	// Restrict ordering checks to the printed index (the body text also mentions
	// "apple"/"animal", which would otherwise match ahead of the Index section).
	h := strings.Index(full, "Index")
	if h < 0 {
		t.Fatalf("no Index heading in output: %q", full)
	}
	text := full[h:]
	// Entries are sorted case-insensitively: animal, apple, zebra. The "cat"
	// subentry follows its "animal" parent.
	idx := func(s string) int { return strings.Index(text, s) }
	order := []string{"Index", "animal", "cat", "apple", "zebra"}
	for i := 1; i < len(order); i++ {
		if idx(order[i-1]) < 0 || idx(order[i]) < 0 {
			t.Fatalf("missing %q or %q in %q", order[i-1], order[i], text)
		}
		if idx(order[i-1]) >= idx(order[i]) {
			t.Errorf("%q should precede %q in index: %q", order[i-1], order[i], text)
		}
	}
	// A dot leader was emitted for the entries.
	if !hasDotLeader(e.mvl) {
		t.Error("no dot leader (\\dotfill) found in the printed index")
	}
}

// The same term indexed on two different pages lists both page numbers, sorted
// and de-duplicated; a duplicate (term, page) collapses to a single mention.
func TestIndexPageNumbers(t *testing.T) {
	src := []byte(`\documentclass{article}
\makeindex
\begin{document}
alpha\index{alpha}\index{alpha}
\newpage
alpha again\index{alpha}
\printindex
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	tree := buildIndexTree(e.indexList())
	node := tree.children["alpha"]
	if node == nil {
		t.Fatalf("no 'alpha' entry in tree: %+v", tree.children)
	}
	if got := pagesString(node.pages); got != "1, 2" {
		t.Errorf("alpha pages = %q, want %q", got, "1, 2")
	}
}

// The '@' sort key orders by the key while the display text is printed: an entry
// keyed "aardvark" sorts before "banana" even though its display starts with 'Z'.
func TestIndexSortKey(t *testing.T) {
	entries := []indexEntry{
		{term: "banana", page: 1},
		{term: "aardvark@Zzz", page: 2},
	}
	keys, displays := parseIndexTerm("aardvark@Zzz")
	if len(keys) != 1 || keys[0] != "aardvark" || displays[0] != "Zzz" {
		t.Fatalf("parseIndexTerm('aardvark@Zzz') = %v/%v, want [aardvark]/[Zzz]", keys, displays)
	}
	tree := buildIndexTree(entries)
	// The lower-cased sort keys are the tree keys.
	if tree.children["aardvark"] == nil || tree.children["banana"] == nil {
		t.Fatalf("sort keys not used as tree keys: %+v", tree.children)
	}
	if d := tree.children["aardvark"].display; d != "Zzz" {
		t.Errorf("display for keyed entry = %q, want %q", d, "Zzz")
	}
}

// parseIndexTerm drops empty levels (stray '!' / trailing '!') and handles empty
// or degenerate sort keys without panicking.
func TestParseIndexTermEdgeCases(t *testing.T) {
	cases := []struct {
		in    string
		keys  []string
		disps []string
	}{
		{"", nil, nil},
		{"!", nil, nil},
		{"animal!", []string{"animal"}, []string{"animal"}},
		{"!cat", []string{"cat"}, []string{"cat"}},
		{"@only", []string{"only"}, []string{"only"}}, // empty sort key falls back to display
		{"key@", []string{"key"}, []string{"key"}},    // empty display falls back to sort key
		{"a!b!c", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
	}
	for _, c := range cases {
		keys, disps := parseIndexTerm(c.in)
		if !eqStrs(keys, c.keys) || !eqStrs(disps, c.disps) {
			t.Errorf("parseIndexTerm(%q) = %v/%v, want %v/%v", c.in, keys, disps, c.keys, c.disps)
		}
	}
}

// buildIndexTree skips entries whose term parses to no usable level (an empty or
// all-separator term), so a stray recorded blank never produces a tree node.
func TestBuildIndexTreeSkipsEmpty(t *testing.T) {
	tree := buildIndexTree([]indexEntry{
		{term: "", page: 1},
		{term: "!", page: 2},
		{term: "real", page: 3},
	})
	if len(tree.children) != 1 || tree.children["real"] == nil {
		t.Fatalf("empty terms should be skipped, tree = %+v", tree.children)
	}
}

func eqStrs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A \printindex with no entries still prints the Index heading and does not crash
// (an empty index is valid, as in LaTeX). The malformed \index (empty group)
// records nothing and does not panic.
func TestIndexEmptyAndMalformed(t *testing.T) {
	src := []byte(`\documentclass{article}
\makeindex
\begin{document}
Body only.\index{}
\printindex
\end{document}`)
	e, err := compile(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The empty \index{} group records nothing.
	for _, en := range e.indexSource {
		if en.term == "" {
			t.Errorf("empty \\index{} recorded a blank entry: %+v", e.indexSource)
		}
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "Index") {
		t.Errorf("empty index should still print the Index heading: %q", b.String())
	}
}

// doIndex tolerates a bare \index with no group: it records nothing and pushes no
// token back that would break the run (error-branch coverage of readIndexTerm).
func TestIndexNoGroup(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt \makeindex \index x`); err != nil {
		t.Fatal(err)
	}
	if len(e.indexEntries) != 0 {
		t.Fatalf("bare \\index recorded %d entries, want 0: %+v", len(e.indexEntries), e.indexEntries)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	if !strings.Contains(b.String(), "x") {
		t.Errorf("stray token after bare \\index should still typeset: %q", b.String())
	}
}

// indexList falls back to the live entries when no aux-pass table was carried, so
// a single-engine run still renders whatever preceded \printindex.
func TestIndexFallbackSinglePass(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	e.SetFont(spMock{})
	src := `\hsize=300pt
\makeindex
gamma\index{gamma}
\printindex`
	if _, err := e.Run(src); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	collectChars(e.mvl, &b)
	got := b.String()
	if !strings.Contains(got, "Index") || !strings.Contains(got, "gamma") {
		t.Errorf("single-pass fallback index missing heading or entry: %q", got)
	}
}

// A term that only ever appears as a parent of subentries prints its heading with
// no page number, while its child carries the page (LaTeX's grouped layout).
func TestIndexParentWithoutPage(t *testing.T) {
	entries := []indexEntry{{term: "animal!cat", page: 3}}
	tree := buildIndexTree(entries)
	animal := tree.children["animal"]
	if animal == nil {
		t.Fatal("no 'animal' parent node")
	}
	if len(animal.pages) != 0 {
		t.Errorf("parent 'animal' has pages %v, want none", animal.pages)
	}
	cat := animal.children["cat"]
	if cat == nil {
		t.Fatal("no 'cat' subentry")
	}
	if got := pagesString(cat.pages); got != "3" {
		t.Errorf("cat pages = %q, want %q", got, "3")
	}
}
