package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeInput puts a file beside the test's working directory and returns the bare
// name to \input. The name is bare and the directory is entered with t.Chdir: a
// path interpolated into TeX source breaks on Windows, where the separator is the
// escape character.
func writeInput(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(t.TempDir(), name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A body scan must stop at the end of the file it began in.
//
// collectEnvBody reads RAW tokens up to \end{name}. When the environment is opened
// by a class or package whose closer arrives another way, that \end is not in the
// input at all — and the scan used to read past the end of the \input'ed file and
// on into the enclosing document, capturing everything into a box nobody places.
//
// arXiv 2304.12934 loses its whole reference list that way: iucr.cls opens
// \begin{minipage}{\linewidth} inside a conditional while the figure is read from
// an \input file, and the scan took the \else branch, the rest of the file, and
// every page after it. 445 words, the worst text deficit in the corpus.
func TestABodyScanStopsAtTheEndOfItsFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inc.tex"),
		[]byte("\\begin{minipage}{3cm}OPENED\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.lenient = true
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`BEFOREZZ\input{inc}AFTERZZ\end{document}`); err != nil {
		t.Fatal(err)
	}
	got := glyphString(e.mvl) + glyphString(e.parList)
	for _, want := range []string{"BEFOREZZ", "AFTERZZ"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from %q — the scan ran past the end of the file", want, got)
		}
	}
}

// And a well-formed environment inside an \input'ed file still works: the guard
// must not fire when the \end IS in the file.
func TestAWellFormedBodyInAnInputFileIsUnaffected(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inc.tex"),
		[]byte("\\begin{minipage}{3cm}INSIDEZZ\\end{minipage}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	e.lenient = true
	if _, err := e.Run(`\documentclass{article}\begin{document}` +
		`BEFOREZZ\input{inc}AFTERZZ\end{document}`); err != nil {
		t.Fatal(err)
	}
	got := glyphString(e.mvl) + glyphString(e.parList)
	for _, want := range []string{"BEFOREZZ", "INSIDEZZ", "AFTERZZ"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from %q", want, got)
		}
	}
}
