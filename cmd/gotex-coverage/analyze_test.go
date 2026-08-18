// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCountGlyphs(t *testing.T) {
	pages := []string{
		`<svg><path d="M0 0"/><path d="M1 1"/><rect/></svg>`,
		`<svg><path d="M2 2"/></svg>`,
	}
	// Three <path across the two pages; a naive line count would report 2.
	if got := countGlyphs(pages); got != 3 {
		t.Errorf("countGlyphs = %d, want 3", got)
	}
	if got := countGlyphs(nil); got != 0 {
		t.Errorf("countGlyphs(nil) = %d, want 0", got)
	}
}

func TestDocumentClass(t *testing.T) {
	cases := map[string]string{
		`\documentclass{article}`:              "article",
		`\documentclass[12pt,a4]{revtex4-2}`:   "revtex4-2",
		`\documentclass [11pt] { report }`:     "report",
		"no class here":                        "",
		`\documentclass[opt]{amsart}% comment`: "amsart",
	}
	for src, want := range cases {
		if got := documentClass(src); got != want {
			t.Errorf("documentClass(%q) = %q, want %q", src, got, want)
		}
	}
}

func TestStripLineComment(t *testing.T) {
	cases := map[string]string{
		"abc % comment":     "abc ",
		"no comment":        "no comment",
		`50\% off`:          `50\% off`,   // escaped percent kept
		`a\\% real comment`: `a\\`,        // even backslashes ⇒ comment
		`x\\\% kept`:        `x\\\% kept`, // odd backslashes ⇒ escaped
		"%whole line":       "",
	}
	for in, want := range cases {
		if got := stripLineComment(in); got != want {
			t.Errorf("stripLineComment(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestStripCommentsMultiline(t *testing.T) {
	in := "keep this % drop\nand this too\n% whole\n"
	want := "keep this \nand this too\n\n"
	if got := stripComments(in); got != want {
		t.Errorf("stripComments = %q, want %q", got, want)
	}
}

func TestCountLetters(t *testing.T) {
	if got := countLetters("abc 123 déf!"); got != 6 { // a,b,c,d,é,f
		t.Errorf("countLetters = %d, want 6", got)
	}
}

func TestBodyRegion(t *testing.T) {
	full := `\documentclass{article}\usepackage{x}\begin{document}BODY HERE\end{document}trailer`
	if got := bodyRegion(full); got != "BODY HERE" {
		t.Errorf("bodyRegion = %q, want %q", got, "BODY HERE")
	}
	// No \begin{document} ⇒ whole thing is body (an \input fragment).
	frag := "just a fragment"
	if got := bodyRegion(frag); got != frag {
		t.Errorf("bodyRegion(fragment) = %q, want whole", got)
	}
	// \begin{document} but no \end{document} ⇒ to end of file.
	open := `\begin{document}tail without end`
	if got := bodyRegion(open); got != "tail without end" {
		t.Errorf("bodyRegion(open) = %q", got)
	}
}

func TestIncludedFiles(t *testing.T) {
	src := `\input{intro} text \include{chap1}
	\input plain.tex
	\subfile{sub/part}
	not\inputfoo{skip}`
	got := includedFiles(src)
	want := []string{"intro", "chap1", "plain.tex", "sub/part"}
	if len(got) != len(want) {
		t.Fatalf("includedFiles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("includedFiles[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveTeX(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "chap.tex"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte("y"), 0o644)

	if p, ok := resolveTeX(dir, "chap"); !ok || filepath.Base(p) != "chap.tex" {
		t.Errorf("resolveTeX(chap) = %q,%v", p, ok)
	}
	if p, ok := resolveTeX(dir, "chap.tex"); !ok || filepath.Base(p) != "chap.tex" {
		t.Errorf("resolveTeX(chap.tex) = %q,%v", p, ok)
	}
	if p, ok := resolveTeX(dir, "data.csv"); !ok || filepath.Base(p) != "data.csv" {
		t.Errorf("resolveTeX(data.csv) = %q,%v", p, ok)
	}
	if _, ok := resolveTeX(dir, "missing"); ok {
		t.Error("resolveTeX(missing) should fail")
	}
}

func TestBodyLettersFollowsIncludes(t *testing.T) {
	dir := t.TempDir()
	// Preamble letters must NOT count; body + included fragment must.
	top := `\documentclass{article}
\usepackage{loadsofpreamblelettershere}
\begin{document}
abcd % ignored comment letters
\input{chap}
\end{document}`
	os.WriteFile(filepath.Join(dir, "main.tex"), []byte(top), 0o644)
	os.WriteFile(filepath.Join(dir, "chap.tex"), []byte("efgh % nope\n"), 0o644)

	got := bodyLetters(dir, "main.tex")
	// body: "abcd" (4) + "input"+"chap" tokens? includedFiles strips \input{chap}
	// but the letters of the control word \input and {chap} remain in the region
	// text, so count them: region = "\nabcd \n\input{chap}\n". Letters: a,b,c,d
	// (4) + i,n,p,u,t (5) + c,h,a,p (4) = 13; plus chap.tex body "efgh" (4) = 17.
	if got != 17 {
		t.Errorf("bodyLetters = %d, want 17", got)
	}
}

func TestBodyLettersCycleAndMissing(t *testing.T) {
	dir := t.TempDir()
	// a includes b, b includes a: the visited set must stop the cycle.
	os.WriteFile(filepath.Join(dir, "a.tex"), []byte(`\begin{document}A\input{b}`), 0o644)
	os.WriteFile(filepath.Join(dir, "b.tex"), []byte(`B\input{a}\input{gone}`), 0o644)
	// Should terminate and count: A(1)+input+b(6) ... just assert it returns and is >0.
	if got := bodyLetters(dir, "a.tex"); got <= 0 {
		t.Errorf("bodyLetters with cycle = %d, want > 0", got)
	}
	// Unreadable top-level ⇒ 0.
	if got := bodyLetters(dir, "nonexistent.tex"); got != 0 {
		t.Errorf("bodyLetters(missing) = %d, want 0", got)
	}
}

func TestToplevelFromReadme(t *testing.T) {
	dir := t.TempDir()
	if got := toplevelFromReadme(dir); got != "" {
		t.Errorf("no readme should give %q", got)
	}
	os.WriteFile(filepath.Join(dir, "00README.json"),
		[]byte(`{"sources":[{"usage":"included","filename":"x.tex"},{"usage":"toplevel","filename":"apssamp.tex"}]}`), 0o644)
	if got := toplevelFromReadme(dir); got != "apssamp.tex" {
		t.Errorf("toplevelFromReadme = %q, want apssamp.tex", got)
	}
	// Malformed JSON ⇒ "".
	os.WriteFile(filepath.Join(dir, "00README.json"), []byte(`{not json`), 0o644)
	if got := toplevelFromReadme(dir); got != "" {
		t.Errorf("malformed readme = %q, want empty", got)
	}
}

func TestPickToplevel(t *testing.T) {
	// readme wins when the file exists.
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "apssamp.tex"), []byte(`\begin{document}x`), 0o644)
	os.WriteFile(filepath.Join(dir, "00README.json"),
		[]byte(`{"sources":[{"usage":"toplevel","filename":"apssamp.tex"}]}`), 0o644)
	if got := pickToplevel(dir); got != "apssamp.tex" {
		t.Errorf("pickToplevel(readme) = %q", got)
	}

	// readme points at a missing file ⇒ fall back to \begin{document} scan.
	dir2 := t.TempDir()
	os.WriteFile(filepath.Join(dir2, "real.tex"), []byte(`\begin{document}x`), 0o644)
	os.WriteFile(filepath.Join(dir2, "00README.json"),
		[]byte(`{"sources":[{"usage":"toplevel","filename":"ghost.tex"}]}`), 0o644)
	if got := pickToplevel(dir2); got != "real.tex" {
		t.Errorf("pickToplevel(readme-missing) = %q, want real.tex", got)
	}

	// Several candidates: the conventional name (main) wins over others.
	dir3 := t.TempDir()
	os.WriteFile(filepath.Join(dir3, "supp.tex"), []byte(`\begin{document}s`), 0o644)
	os.WriteFile(filepath.Join(dir3, "main.tex"), []byte(`\begin{document}m`), 0o644)
	if got := pickToplevel(dir3); got != "main.tex" {
		t.Errorf("pickToplevel(preferred) = %q, want main.tex", got)
	}

	// Several candidates, none preferred: the largest wins.
	dir4 := t.TempDir()
	os.WriteFile(filepath.Join(dir4, "aaa.tex"), []byte(`\begin{document}short`), 0o644)
	os.WriteFile(filepath.Join(dir4, "bbb.tex"), []byte(`\begin{document}`+longString(500)), 0o644)
	if got := pickToplevel(dir4); got != "bbb.tex" {
		t.Errorf("pickToplevel(largest) = %q, want bbb.tex", got)
	}

	// The id-named file wins over conventional names.
	id := filepath.Base(dir4)
	os.WriteFile(filepath.Join(dir4, id+".tex"), []byte(`\begin{document}id`), 0o644)
	if got := pickToplevel(dir4); got != id+".tex" {
		t.Errorf("pickToplevel(id) = %q, want %s.tex", got, id)
	}

	// No .tex with \begin{document} ⇒ "".
	dir5 := t.TempDir()
	os.WriteFile(filepath.Join(dir5, "frag.tex"), []byte(`no document here`), 0o644)
	if got := pickToplevel(dir5); got != "" {
		t.Errorf("pickToplevel(none) = %q, want empty", got)
	}
}

func TestMedian(t *testing.T) {
	if got := median(nil); got != 0 {
		t.Errorf("median(nil) = %v", got)
	}
	if got := median([]float64{3, 1, 2}); got != 2 {
		t.Errorf("median(odd) = %v, want 2", got)
	}
	if got := median([]float64{4, 1, 3, 2}); got != 2.5 {
		t.Errorf("median(even) = %v, want 2.5", got)
	}
}

func TestCoverageOf(t *testing.T) {
	if got := coverageOf(1024, 1024); got != 1024 {
		t.Errorf("coverageOf(1024,1024) = %v, want 1024", got)
	}
	if got := coverageOf(100, 0); got != 0 {
		t.Errorf("coverageOf(_,0) = %v, want 0", got)
	}
}

func TestResultSilent(t *testing.T) {
	if (Result{Runaway: true}).Silent() != true {
		t.Error("runaway should be silent suspect")
	}
	if (Result{OpenGroups: 2}).Silent() != true {
		t.Error("open groups should be silent suspect")
	}
	if (Result{PageCapHit: true}).Silent() != true {
		t.Error("page cap should be silent suspect")
	}
	if (Result{Runaway: true, Err: "boom"}).Silent() != false {
		t.Error("a failed render is not a silent suspect")
	}
	if (Result{Glyphs: 5}).Silent() != false {
		t.Error("clean result is not a silent suspect")
	}
}

func TestRankOutliers(t *testing.T) {
	results := []Result{
		{ID: "a", Coverage: 1000, BodyLetters: 100},
		{ID: "b", Coverage: 1000, BodyLetters: 100},
		{ID: "c", Coverage: 1000, BodyLetters: 100},
		{ID: "low1", Coverage: 100, BodyLetters: 100},
		{ID: "low2", Coverage: 5, BodyLetters: 100},
		{ID: "failed", Coverage: 0, BodyLetters: 100, Err: "boom"}, // excluded (Err)
		{ID: "nobody", Coverage: 0, BodyLetters: 0},                // excluded (no body)
	}
	outliers, med := rankOutliers(results, 0.4)
	if med != 1000 {
		t.Errorf("median = %v, want 1000", med)
	}
	if len(outliers) != 2 {
		t.Fatalf("outliers = %d, want 2 (%v)", len(outliers), outliers)
	}
	if outliers[0].ID != "low2" || outliers[1].ID != "low1" {
		t.Errorf("outliers not sorted worst-first: %v", []string{outliers[0].ID, outliers[1].ID})
	}
}

func longString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'x'
	}
	return string(b)
}
