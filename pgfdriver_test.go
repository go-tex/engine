package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// \input searches the same places \usepackage does — the document's directory,
// the TEXINPUTS/GOTEX_TEXMF path, and the embedded set. A package splits itself
// across files and pulls them in with \input, so a package found on the path
// whose own parts were not would load only its first file.
func TestInputSearchesTheTeXPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bit.tex"), []byte(`\def\frombit{OK}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	out, err := e.Run(`\input{bit}\message{[\frombit]}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "[OK]" {
		t.Errorf("\\input from TEXINPUTS = %q, want [OK]", out)
	}
}

// An \input of a file that is nowhere is an error in strict mode (and skipped in
// a tolerant one), not a silent success.
func TestInputMissingFile(t *testing.T) {
	t.Setenv("TEXINPUTS", "")
	if _, err := New().Run(`\input{nosuchfileanywhere}`); err == nil {
		t.Error("a missing \\input file must be reported")
	}
}

// An absolute path is read as given, not joined onto the search path.
func TestInputAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "abs.tex")
	if err := os.WriteFile(p, []byte(`\message{ABS}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// A path is spliced into TeX source, where a backslash begins a control
	// sequence, so it goes in with forward slashes (which os.ReadFile accepts on
	// every platform, Windows included).
	out, err := New().Run(`\input{` + filepath.ToSlash(p) + `}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "ABS" {
		t.Errorf("absolute \\input = %q", out)
	}
}

// The pgf system-layer driver ships in the binary, so \input{pgfsys-gotex.def}
// resolves with no files on disk — the file pgf's system layer loads once
// \pgfsysdriver names it.
func TestDriverFileIsEmbedded(t *testing.T) {
	data, ok := embeddedTeXFile("pgfsys-gotex.def")
	if !ok {
		t.Fatal("pgfsys-gotex.def is not embedded")
	}
	src := string(data)
	for _, want := range []string{
		`\input{pgfsys-common-svg.def}`, // the operations come from pgf itself
		`\def\pgfsys@invoke`,            // every operation is sent through here
		`\special{gotex:`,               // …into this engine's driver namespace
		`<gotex:origin/>`,               // a picture declares its origin
		`\def\pgfsys@beginpicture`,
		`\def\pgfsys@endpicture`,
		`\def\pgfsys@hbox`,
		`\def\pgfsys@typesetpicturebox`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the driver does not define %s", want)
		}
	}
}

// The LaTeX kernel names this engine's driver, which is what makes pgf's system
// layer load it instead of guessing a backend it cannot drive.
func TestPgfsysdriverIsNamed(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\message{\pgfsysdriver}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "pgfsys-gotex.def" {
		t.Errorf("\\pgfsysdriver = %q, want pgfsys-gotex.def", out)
	}
}

// The drawing packages use their stubs by default and their real sources only
// when GOTEX_PGF asks for them; the other emulated packages are unaffected.
func TestRealPGFIsOptIn(t *testing.T) {
	t.Setenv("GOTEX_PGF", "")
	for _, name := range []string{"tikz", "pgf", "pgfplots"} {
		if !emulateOnly(name) {
			t.Errorf("%s must use its stub by default", name)
		}
	}
	t.Setenv("GOTEX_PGF", "1")
	for _, name := range []string{"tikz", "pgf", "pgfplots"} {
		if emulateOnly(name) {
			t.Errorf("%s must load for real when asked", name)
		}
	}
	for _, name := range []string{"hyperref", "xcolor", "geometry", "graphicx"} {
		if !emulateOnly(name) {
			t.Errorf("%s is emulated whatever GOTEX_PGF says", name)
		}
	}
	if emulateOnly("amsmath") {
		t.Error("a package with no stub must still load from disk")
	}
}

// With the stubs in force a picture is still gobbled and stood in for, so the
// opt-in switch cannot change what an ordinary document renders.
func TestPictureStillGobbledByDefault(t *testing.T) {
	t.Setenv("GOTEX_PGF", "")
	e := New()
	e.SetFont(spMock{})
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`A\begin{tikzpicture}\draw (0,0)--(1,1);\end{tikzpicture}B`); err != nil {
		t.Fatal(err)
	}
	if got := glyphString(e.mvl); !strings.Contains(got, "A") || !strings.Contains(got, "B") ||
		strings.Contains(got, "draw") {
		t.Errorf("the picture body leaked into the text: %q", got)
	}
}
