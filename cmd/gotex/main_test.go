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
