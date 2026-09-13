package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A real TeX puts \message and \typeout on the terminal, and that is how a source
// says which branch it took: which driver a package picked, what an \ifx decided,
// what \the of a length holds. The engine collected the text and no caller could
// read it, so a probe written the way TeX documents it printed NOTHING and a
// diagnosis had to typeset its answers onto a page instead.
func TestRunPrintsWhatTheDocumentSaid(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\message{[branch=A]}\hsize=300pt x\par`), 0644)
	out := filepath.Join(dir, "doc.pdf")

	var so, se bytes.Buffer
	if code := run([]string{"-messages", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if !strings.Contains(se.String(), "[branch=A]") {
		t.Errorf("stderr does not carry the message:\n%s", se.String())
	}
}

// Without the flag the render is as quiet as it was.
func TestRunIsQuietWithoutTheFlag(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\message{[branch=A]}\hsize=300pt x\par`), 0644)
	out := filepath.Join(dir, "doc.pdf")

	var so, se bytes.Buffer
	if code := run([]string{"-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if strings.Contains(se.String(), "branch=A") {
		t.Errorf("printed the message without being asked:\n%s", se.String())
	}
}

// A document that says nothing prints no heading either — the flag must not add a
// line of noise to every quiet render.
func TestRunPrintsNoHeadingWhenTheDocumentIsSilent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\hsize=300pt x\par`), 0644)
	out := filepath.Join(dir, "doc.pdf")

	var so, se bytes.Buffer
	if code := run([]string{"-messages", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if strings.Contains(se.String(), "the document said") {
		t.Errorf("printed a heading with nothing under it:\n%s", se.String())
	}
}
