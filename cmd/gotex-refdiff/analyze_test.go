// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func keys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestContentWordsFoldsLigatures(t *testing.T) {
	// The ligature "ﬁ" must fold to "fi" so "ﬁrst" and "first" are the SAME
	// content word. Without folding these are two distinct tokens and the
	// reference's "first" would count as missing — the exact ToUnicode-gap
	// penalty the tool must NOT levy.
	ref := contentWords("first flow office")
	got := contentWords("ﬁrst ﬂow oﬃce")
	if !reflect.DeepEqual(keys(ref), keys(got)) {
		t.Fatalf("ligature folding mismatch:\n ref=%v\n got=%v", keys(ref), keys(got))
	}
}

func TestContentWordsDropsNumbersAndSingles(t *testing.T) {
	// Pure numbers (page/equation numbers) and single characters (math
	// variables) are dropped; real words survive.
	got := contentWords("Theorem 42 x proves 3.14 the result A")
	want := []string{"proves", "result", "the", "theorem"}
	if !reflect.DeepEqual(keys(got), want) {
		t.Fatalf("normalisation:\n got=%v\n want=%v", keys(got), want)
	}
}

func TestContentWordsSetAndCase(t *testing.T) {
	got := contentWords("Word word WORD")
	if len(got) != 1 {
		t.Fatalf("case-folded dedup expected 1 word, got %v", keys(got))
	}
	if _, ok := got["word"]; !ok {
		t.Fatalf("expected lower-cased 'word', got %v", keys(got))
	}
}

func TestRecall(t *testing.T) {
	ref := contentWords("alpha beta gamma delta")
	got := contentWords("alpha beta zeta")
	ratio, refCount, common := recall(ref, got)
	if refCount != 4 || common != 2 {
		t.Fatalf("counts: refCount=%d common=%d (want 4,2)", refCount, common)
	}
	if ratio != 0.5 {
		t.Fatalf("ratio=%v want 0.5", ratio)
	}
}

func TestRecallEmptyReference(t *testing.T) {
	ratio, refCount, common := recall(map[string]struct{}{}, contentWords("anything here"))
	if ratio != 0 || refCount != 0 || common != 0 {
		t.Fatalf("empty ref: got %v/%d/%d want 0/0/0", ratio, refCount, common)
	}
}

func TestIsContentToken(t *testing.T) {
	cases := map[string]bool{
		"the": true, "a": false, "42": false, "007": false,
		"x": false, "h2o": true, "": false, "co2": true,
	}
	for tok, want := range cases {
		if got := isContentToken(tok); got != want {
			t.Errorf("isContentToken(%q)=%v want %v", tok, got, want)
		}
	}
}

func TestDocumentClass(t *testing.T) {
	cases := map[string]string{
		`\documentclass[11pt,letterpaper]{article}`: "article",
		`\documentclass{revtex4-1}`:                 "revtex4-1",
		`\documentclass [12pt] { report }`:          "report",
		`no class here`:                             "?",
	}
	for src, want := range cases {
		if got := documentClass(src); got != want {
			t.Errorf("documentClass(%q)=%q want %q", src, got, want)
		}
	}
}

func TestResolveToplevelReadme(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "00README.json", `{"sources":[
		{"usage":"ignore","filename":"fig.png"},
		{"usage":"toplevel","filename":"guide.tex"}]}`)
	write(t, dir, "guide.tex", `\documentclass{book}\begin{document}hi\end{document}`)
	write(t, dir, "other.tex", `\begin{document}decoy\end{document}`)
	got, err := resolveToplevel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "guide.tex") {
		t.Fatalf("README toplevel not honoured: %s", got)
	}
}

func TestResolveToplevelBeginDocument(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "intro.tex", `\section{intro}no preamble here`)
	write(t, dir, "main.tex", `\documentclass{article}\begin{document}body\end{document}`)
	got, err := resolveToplevel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "main.tex") {
		t.Fatalf("expected main.tex, got %s", got)
	}
}

func TestResolveToplevelBeginDocumentDeterministic(t *testing.T) {
	dir := t.TempDir()
	// Two files with \begin{document}; the lexicographically first wins.
	write(t, dir, "b.tex", `\begin{document}b\end{document}`)
	write(t, dir, "a.tex", `\begin{document}a\end{document}`)
	got, err := resolveToplevel(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "a.tex") {
		t.Fatalf("expected a.tex (first), got %s", got)
	}
}

func TestResolveToplevelNone(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "frag.tex", `just a fragment, no document environment`)
	if _, err := resolveToplevel(dir); err != errNoToplevel {
		t.Fatalf("expected errNoToplevel, got %v", err)
	}
	if errNoToplevel.Error() == "" {
		t.Fatal("errNoToplevel has empty message")
	}
}

func TestResolveToplevelMissingDir(t *testing.T) {
	if _, err := resolveToplevel(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestReadmeToplevelMalformed(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "00README.json", `{not json`)
	if _, ok := readmeToplevel(dir); ok {
		t.Fatal("malformed README should not yield a toplevel")
	}
}

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
