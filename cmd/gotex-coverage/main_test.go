// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	engine "github.com/go-tex/engine"
)

// fakeRender returns a canned render, so analyzePaper can be tested without the
// engine.
func fakeRender(pages []string, diag engine.Diagnostics, err error) renderFunc {
	return func(dir, top string) ([]string, engine.Diagnostics, error) {
		return pages, diag, err
	}
}

func TestAnalyzePaperNoToplevel(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "frag.tex"), []byte("no document"), 0o644)
	r := analyzePaper(dir, fakeRender(nil, engine.Diagnostics{}, nil))
	if r.Err == "" {
		t.Fatal("expected an error for a paper with no top-level")
	}
	if r.ID != filepath.Base(dir) {
		t.Errorf("ID = %q", r.ID)
	}
}

func TestAnalyzePaperRenderError(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.tex"),
		[]byte(`\documentclass{article}\begin{document}hi\end{document}`), 0o644)
	r := analyzePaper(dir, fakeRender(nil, engine.Diagnostics{}, fmt.Errorf("kaboom")))
	if r.Err != "kaboom" {
		t.Errorf("Err = %q, want kaboom", r.Err)
	}
	if r.Class != "article" {
		t.Errorf("Class = %q, want article", r.Class)
	}
}

func TestAnalyzePaperFull(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.tex"),
		[]byte(`\documentclass{article}\begin{document}abcd\end{document}`), 0o644)
	pages := []string{`<svg><path/><path/><path/></svg>`}
	diag := engine.Diagnostics{
		Skipped:    map[string]int{"foo": 2, "bar": 3},
		Runaway:    true,
		OpenGroups: 2,
		PageCapHit: true,
	}
	r := analyzePaper(dir, fakeRender(pages, diag, nil))
	if r.Glyphs != 3 {
		t.Errorf("Glyphs = %d, want 3", r.Glyphs)
	}
	if r.Pages != 1 {
		t.Errorf("Pages = %d, want 1", r.Pages)
	}
	if r.Undefined != 5 {
		t.Errorf("Undefined = %d, want 5", r.Undefined)
	}
	if !r.Runaway || r.OpenGroups != 2 || !r.PageCapHit {
		t.Errorf("flags not carried: %+v", r)
	}
	if r.BodyLetters != 4 { // "abcd"
		t.Errorf("BodyLetters = %d, want 4", r.BodyLetters)
	}
	if r.Coverage <= 0 {
		t.Errorf("Coverage = %v, want > 0", r.Coverage)
	}
	if !r.Silent() {
		t.Error("expected Silent() true")
	}
}

func TestRenderWithTimeoutPassthrough(t *testing.T) {
	inner := fakeRender([]string{"<path/>"}, engine.Diagnostics{OpenGroups: 1}, nil)
	rf := renderWithTimeout(time.Second, inner)
	pages, diag, err := rf("dir", "top")
	if err != nil || len(pages) != 1 || diag.OpenGroups != 1 {
		t.Errorf("passthrough failed: %v %v %v", pages, diag, err)
	}
}

func TestRenderWithTimeoutFires(t *testing.T) {
	slow := func(dir, top string) ([]string, engine.Diagnostics, error) {
		time.Sleep(200 * time.Millisecond)
		return []string{"late"}, engine.Diagnostics{}, nil
	}
	rf := renderWithTimeout(5*time.Millisecond, slow)
	_, _, err := rf("dir", "top")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout error, got %v", err)
	}
}

func TestRenderPaperReal(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.tex"),
		[]byte("\\documentclass{article}\n\\begin{document}\nHello world.\n\\end{document}\n"), 0o644)
	pages, _, err := renderPaper(dir, "main.tex")
	if err != nil {
		t.Fatalf("renderPaper: %v", err)
	}
	if countGlyphs(pages) == 0 {
		t.Error("real render produced no glyphs")
	}
}

func TestRenderPaperMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, _, err := renderPaper(dir, "nope.tex")
	if err == nil {
		t.Error("expected an error reading a missing top-level")
	}
}

func TestPaperDirs(t *testing.T) {
	root := t.TempDir()
	// two paper dirs with .tex, one dir without, one loose file
	os.MkdirAll(filepath.Join(root, "p1"), 0o755)
	os.WriteFile(filepath.Join(root, "p1", "a.tex"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(root, "p2"), 0o755)
	os.WriteFile(filepath.Join(root, "p2", "b.tex"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(root, "empty"), 0o755)
	os.WriteFile(filepath.Join(root, "loose.txt"), []byte("x"), 0o644)

	dirs, err := paperDirs(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirs) != 2 {
		t.Fatalf("paperDirs = %v, want 2", dirs)
	}
	// limit
	dirs, _ = paperDirs(root, 1)
	if len(dirs) != 1 {
		t.Errorf("paperDirs(limit 1) = %v", dirs)
	}
	// bad root
	if _, err := paperDirs(filepath.Join(root, "nope"), 0); err == nil {
		t.Error("expected error for missing root")
	}
}

func TestFlagString(t *testing.T) {
	if got := flagString(Result{}); got != "-" {
		t.Errorf("flagString(clean) = %q, want -", got)
	}
	got := flagString(Result{Runaway: true, OpenGroups: 3, PageCapHit: true})
	for _, want := range []string{"RUNAWAY", "OPENGROUPS=3", "PAGECAP"} {
		if !strings.Contains(got, want) {
			t.Errorf("flagString missing %q in %q", want, got)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("abcdefgh", 4); got != "abc…" {
		t.Errorf("truncate = %q, want abc…", got)
	}
	if got := truncate("abc", 1); got != "a" {
		t.Errorf("truncate n=1 = %q, want a", got)
	}
}

func TestReport(t *testing.T) {
	results := []Result{
		{ID: "good1", Class: "article", Coverage: 1000, Glyphs: 1000, BodyLetters: 1024},
		{ID: "good2", Class: "article", Coverage: 1000, Glyphs: 1000, BodyLetters: 1024},
		{ID: "good3", Class: "article", Coverage: 1000, Glyphs: 1000, BodyLetters: 1024},
		{ID: "swallow", Class: "jss", Coverage: 5, Glyphs: 5, BodyLetters: 1024, OpenGroups: 3},
		{ID: "flaggedundef", Class: "article", Coverage: 8, Glyphs: 8, BodyLetters: 1024, OpenGroups: 1, Undefined: 4},
		{ID: "undefheavy", Class: "article", Coverage: 10, Glyphs: 10, BodyLetters: 1024, Undefined: 200},
		{ID: "broken", Err: "panic: boom"},
	}
	var buf bytes.Buffer
	report(&buf, results, 0.4, 20, false)
	out := buf.String()
	if !strings.Contains(out, "swallow") {
		t.Error("report missing the swallow suspect")
	}
	if !strings.Contains(out, "OPENGROUPS=3") {
		t.Error("report missing the flag")
	}
	if !strings.Contains(out, "failed to render") || !strings.Contains(out, "broken") {
		t.Error("report missing the failed section")
	}

	// silent-only mode: keeps swallow (flag + no undefined), drops undefheavy.
	var buf2 bytes.Buffer
	report(&buf2, results, 0.4, 20, true)
	out2 := buf2.String()
	if !strings.Contains(out2, "SILENT-SWALLOW SUSPECTS") {
		t.Error("silent-only header missing")
	}
	if !strings.Contains(out2, "swallow") {
		t.Error("silent-only dropped the true suspect")
	}
	// A flagged paper is a silent suspect even with some undefined commands: the
	// swallow flag is loss the undefined tally cannot explain.
	if !strings.Contains(out2, "flaggedundef") {
		t.Error("silent-only dropped a flagged suspect that also had undefined commands")
	}
	// undefheavy has 200 undefined and NO swallow flag ⇒ its loss is explained by
	// feature gaps, so it is not a silent-swallow suspect and must be absent.
	if strings.Contains(out2, "undefheavy") {
		t.Error("silent-only kept an undefined-heavy paper with no swallow flag")
	}
}

func TestReportTopLimit(t *testing.T) {
	var results []Result
	for i := 0; i < 5; i++ {
		results = append(results, Result{ID: "hi", Coverage: 1000, Glyphs: 1000, BodyLetters: 1024})
	}
	for i := 0; i < 5; i++ {
		results = append(results, Result{ID: fmt.Sprintf("lo%d", i), Coverage: 1, Glyphs: 1, BodyLetters: 1024})
	}
	for i := 0; i < 5; i++ {
		results = append(results, Result{ID: fmt.Sprintf("f%d", i), Err: "boom"})
	}
	var buf bytes.Buffer
	report(&buf, results, 0.4, 2, false) // top=2 caps both suspects and failures
	out := buf.String()
	if !strings.Contains(out, "and 3 more") {
		t.Errorf("expected failure truncation note, got:\n%s", out)
	}
}

func TestRunEndToEnd(t *testing.T) {
	root := t.TempDir()
	// A healthy paper and a near-empty one.
	p1 := filepath.Join(root, "0001.0001")
	os.MkdirAll(p1, 0o755)
	os.WriteFile(filepath.Join(p1, "main.tex"),
		[]byte("\\documentclass{article}\n\\begin{document}\n"+strings.Repeat("word ", 200)+"\n\\end{document}\n"), 0o644)
	p2 := filepath.Join(root, "0002.0002")
	os.MkdirAll(p2, 0o755)
	os.WriteFile(filepath.Join(p2, "main.tex"),
		[]byte("\\documentclass{article}\n\\begin{document}\n\\end{document}\n"), 0o644)

	var out, errb bytes.Buffer
	code := run([]string{"-n", "0", "-top", "5", root}, &out, &errb)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "median coverage") {
		t.Errorf("run output missing summary:\n%s", out.String())
	}
}

func TestRunUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 2 {
		t.Errorf("run(no args) = %d, want 2", code)
	}
	if code := run([]string{"-badflag"}, &out, &errb); code != 2 {
		t.Errorf("run(bad flag) = %d, want 2", code)
	}
}

func TestRunBadRoot(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{filepath.Join(t.TempDir(), "nope")}, &out, &errb); code != 1 {
		t.Errorf("run(missing root) = %d, want 1", code)
	}
	// Existing but empty root ⇒ no paper dirs ⇒ exit 1.
	if code := run([]string{t.TempDir()}, &out, &errb); code != 1 {
		t.Errorf("run(empty root) = %d, want 1", code)
	}
}
