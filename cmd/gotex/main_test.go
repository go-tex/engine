package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

// -report-skipped also surfaces math equations the go-tex/math layer dropped: an
// unknown command inside a formula takes the whole equation with it, which the
// text-mode "undefined commands" list would never show.
func TestRunReportMathDropped(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}`+
		`A formula $\nosuchmathprimitive$ here.\end{document}`), 0644)
	out := filepath.Join(dir, "doc.svg")
	var so, se bytes.Buffer
	if code := run([]string{"-lenient", "-report-skipped", "-format", "svg", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if !bytes.Contains(se.Bytes(), []byte("dropped by the math layer")) ||
		!bytes.Contains(se.Bytes(), []byte(`\nosuchmathprimitive`)) {
		t.Errorf("report missing the dropped math equation; stderr=%q", se.String())
	}
}

// The header counts EQUATIONS and says so. It used to print len(MathDropped) — the
// number of distinct triggers — under the words "equation group(s) dropped", and on
// one corpus paper that announced 12 above twelve rows summing to 58. Both numbers
// are now printed and each is named.
//
// Three formulas refused by two distinct commands, so the two counts differ and a
// header that confused them would fail here.
func TestRunReportCountsEquationsAndTriggersSeparately(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}`+
		`$\nosuchmathprimitive$ $\nosuchmathprimitive$ $\anotherbadone$`+
		`\end{document}`), 0644)
	out := filepath.Join(dir, "doc.svg")
	var so, se bytes.Buffer
	if code := run([]string{"-lenient", "-report-skipped", "-format", "svg", "-o", out, src}, &so, &se); code != 0 {
		t.Fatalf("run exit=%d stderr=%s", code, se.String())
	}
	if want := "3 equation(s) dropped by the math layer over 2 distinct trigger(s)"; !bytes.Contains(se.Bytes(), []byte(want)) {
		t.Errorf("header does not read %q; stderr=%q", want, se.String())
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

// A command whose name IS a control character must be readable in the census.
// \^^M — control <return>, which a line ending in a backslash makes — was the
// most frequent undefined command in a 157-paper corpus and printed as a bare
// backslash followed by an invisible byte, so it read as noise for months.
func TestPrintableCSNamesAControlCharacter(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"\r", "^^M"},
		{"\n", "^^J"},
		{"\t", "^^I"},
		{"\x7f", "^^?"},
		{"section", "section"},
		{" ", " "},
	} {
		if got := printableCS(c.in); got != c.want {
			t.Errorf("printableCS(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A compile that fails leaves the previous output untouched.
//
// os.Create truncates its target before a single page exists, so a failed, hung
// or killed compile used to leave the destination EMPTY. That is not a small
// matter, because TeX's convention is that `gotex main.tex` writes main.pdf: the
// destination is very often a file somebody wants to keep. On 2026-09-27 a corpus
// sweep running the engine under `timeout` in each paper's own directory emptied
// 136 of the 153 main.pdf it passed through, and those were the published PDFs
// the corpus compared against.
func TestAFailedCompileLeavesTheOldOutputAlone(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	// Strict mode (no -lenient): an undefined control sequence is fatal.
	os.WriteFile(src, []byte(`\documentclass{article}\begin{document}\thisIsNotACommand\end{document}`), 0644)
	out := filepath.Join(dir, "doc.pdf")
	const kept = "A PREVIOUS PDF WORTH KEEPING"
	if err := os.WriteFile(out, []byte(kept), 0644); err != nil {
		t.Fatal(err)
	}

	var so, se bytes.Buffer
	if code := run([]string{"-o", out, src}, &so, &se); code == 0 {
		t.Fatalf("the compile was meant to fail; stderr=%s", se.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("the output is gone: %v", err)
	}
	if string(b) != kept {
		t.Errorf("output is %d bytes %q, want the %d bytes that were there", len(b), b, len(kept))
	}
	// Nothing half-written is left in its place either.
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".gotex-") {
			t.Errorf("a temporary file was left behind: %s", e.Name())
		}
	}
}

// And a compile that succeeds still replaces it.
func TestASuccessfulCompileReplacesTheOldOutput(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "doc.tex")
	os.WriteFile(src, []byte(`\hsize=300pt Replaced.\par`), 0644)
	out := filepath.Join(dir, "doc.pdf")
	os.WriteFile(out, []byte("OLD"), 0644)

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
	// The mode the output has always had, not CreateTemp's 0600.
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %v, want 0644", fi.Mode().Perm())
	}
}
