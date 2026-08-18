package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// The splicer marks the end of every loaded file with a control sequence
// (\gotexendinput, \@endofpackagehook, \@endofclasshook, \@gotex@endload). Those
// names are not private: LaTeX passes \@endofpackagehook around BY NAME, and a
// delimited scan that runs off the end of a file must still be stopped by them.
//
// The two halves pull in opposite directions, and getting one wrong costs the
// other. This pins both.
//
// History: refusing a sentinel as an UNDELIMITED argument abandoned the kernel's
// \AtEndOfPackage#1{\g@addto@macro\@endofpackagehook{#1}}, so package end-hooks
// never ran; etoolbox's catcode restore is one of those, "|" kept catcode 3, and
// every beamer document rendered zero pages (v0.160.0).
func TestFileEndSentinelsAreOrdinaryArguments(t *testing.T) {
	for _, name := range []string{"gotexendinput", "@endofpackagehook", "@endofclasshook", "@gotex@endload"} {
		src := `\makeatletter\def\take#1{[ok]}\message{\take\` + name + `}`
		if got := ckRun(t, src); got != "[ok]" {
			t.Errorf(`\%s as an undelimited argument = %q, want "[ok]"`, name, got)
		}
	}
}

// …and the guard that still has to hold: a DELIMITED argument whose delimiter
// never comes must not eat the file's end and the document after it.
func TestDelimitedScanStillStopsAtAFileEnd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "runaway.sty"),
		[]byte(`\makeatletter\def\eat#1\@nil{}\eat with-no-delimiter-in-sight`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	out, err := e.Run(`\message{[AVANT]}\usepackage{runaway}\message{[APRES]}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if out != "[AVANT] [APRES]" {
		t.Errorf("a runaway delimited argument swallowed past its file: %q", out)
	}
}
