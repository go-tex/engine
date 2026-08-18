package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRunCompilesPDF(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\hsize=300pt Hello from \TeX\ compiled by gotex.\par`), 0644)
	out := filepath.Join(dir, "doc.pdf")
	var so, se bytes.Buffer
	if code := run([]string{"-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("bad PDF (%d bytes)", len(b))
	}
	if !bytes.Contains(b, []byte("/FontFile2")) && !bytes.Contains(b, []byte("/FontFile3")) {
		t.Error("no embedded font subset")
	}
}

func TestRunSVG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "d.tex")
	os.WriteFile(src, []byte(`\hsize=200pt Short line.\par`), 0644)
	out := filepath.Join(dir, "d.svg")
	if code := run([]string{"-format", "svg", "-o", out, src}, new(bytes.Buffer), new(bytes.Buffer)); code != 0 {
		t.Fatalf("svg run failed")
	}
	b, _ := os.ReadFile(out)
	if !bytes.Contains(b, []byte("<svg")) {
		t.Error("no svg output")
	}
}

func TestRunUsageError(t *testing.T) {
	if code := run(nil, new(bytes.Buffer), new(bytes.Buffer)); code != 2 {
		t.Errorf("no input should exit 2, got %d", code)
	}
}

func TestRunOutdirLatexmkStyle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.tex")
	os.WriteFile(src, []byte(`\hsize=300pt A latexmk-style compile.\par`), 0644)
	build := filepath.Join(dir, ".build")
	os.Mkdir(build, 0755)
	// mirror the loom contract: gotex -pdf -outdir=<build> <main.tex>
	if code := run([]string{"-pdf", "-outdir", build, src}, new(bytes.Buffer), new(bytes.Buffer)); code != 0 {
		t.Fatalf("run exit != 0")
	}
	pdf := filepath.Join(build, "main.pdf")
	b, err := os.ReadFile(pdf)
	if err != nil {
		t.Fatalf("expected %s: %v", pdf, err)
	}
	if len(b) < 500 || string(b[:5]) != "%PDF-" {
		t.Fatalf("bad PDF at outdir")
	}
}

// -report-skipped lists the undefined commands a lenient render skipped, most
// frequent first, so a silently-dropped command (which can take a document's whole
// body with it) is visible instead of hidden.
func TestRunReportSkipped(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}`+
		`\weirdcmd{x} text \anotherundef \weirdcmd{y}\end{document}`), 0644)
	out := filepath.Join(dir, "doc.svg")
	var so, se bytes.Buffer
	if code := run([]string{"-lenient", "-report-skipped", "-format", "svg", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	got := se.String()
	if !bytes.Contains(se.Bytes(), []byte(`\weirdcmd`)) || !bytes.Contains(se.Bytes(), []byte(`\anotherundef`)) {
		t.Errorf("report missing a skipped command; stderr=%q", got)
	}
	// \weirdcmd (twice) must be reported before \anotherundef (once).
	if i, j := bytes.Index(se.Bytes(), []byte(`\weirdcmd`)), bytes.Index(se.Bytes(), []byte(`\anotherundef`)); i < 0 || j < 0 || i > j {
		t.Errorf("skipped commands not ordered by frequency; stderr=%q", got)
	}
}

// -report-skipped surfaces undefined ENVIRONMENTS on their own line: \begin{env}
// of an environment the engine lacks is a silent \relax (via \csname) and never
// counts as an undefined command, so it needs its own reporting.
func TestRunReportUndefinedEnvs(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}`+
		`\begin{itemize}\item a\end{itemize}`+
		`\begin{mysteryenv}body\end{mysteryenv}`+
		`\begin{mysteryenv}more\end{mysteryenv}\end{document}`), 0644)
	out := filepath.Join(dir, "doc.svg")
	var so, se bytes.Buffer
	if code := run([]string{"-lenient", "-report-skipped", "-format", "svg", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	got := se.String()
	if !bytes.Contains(se.Bytes(), []byte("undefined environment")) || !bytes.Contains(se.Bytes(), []byte("mysteryenv")) {
		t.Errorf("report did not list the undefined environment; stderr=%q", got)
	}
	// A defined environment (itemize) must not be reported as undefined.
	if bytes.Contains(se.Bytes(), []byte("itemize")) {
		t.Errorf("defined environment itemize wrongly reported as undefined; stderr=%q", got)
	}
}

// -report-skipped also surfaces the silent-swallow alarms: a runaway that tripped
// is reported even though no command was "undefined".
func TestRunReportRunawayWarning(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}\def\lp{\lp}\lp\end{document}`), 0644)
	out := filepath.Join(dir, "doc.svg")
	var so, se bytes.Buffer
	if code := run([]string{"-lenient", "-report-skipped", "-format", "svg", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if !bytes.Contains(se.Bytes(), []byte("runaway")) {
		t.Errorf("report did not warn about the runaway; stderr=%q", se.String())
	}
}
