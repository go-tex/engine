package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// When a delimited macro argument runs off the end of the file that began it, TeX
// reports "Runaway argument? … File ended while scanning use of \x". The text it
// scanned has been EATEN as the argument; TeX never re-reads it.
//
// The engine stops such a scan at the file-end sentinel and abandons the call so
// the document after the file survives. It must DROP what it scanned, not put it
// back: what a runaway inside a class file scans includes the loader's own splice
// (\UseHook{package/…}\@gotex@endload), and a caller that has changed the catcode
// of \ — beamer's line-by-line comment skipper does \@makeother\\ — then prints
// that internal text as literal characters onto the page.
func TestRunawayArgumentTextIsDroppedNotReinserted(t *testing.T) {
	const (
		before = "BEFOREMARK"
		after  = "BODYAFTERPATCH"
		inside = "TAILOFINCLUDE"
	)
	dir := t.TempDir()
	inc := "\\makeatletter\n" +
		"\\def\\@tempa#1$#2\\@nil{\\def\\zz{#1}}%\n" +
		"\\expandafter\\@tempa\\relax\\@nil\n" +
		inside + "\n\\makeatother\n"
	p := filepath.Join(dir, "gotex_runaway.tex")
	if err := os.WriteFile(p, []byte(inc), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "\\documentclass{article}\n\\begin{document}\n" + before +
		"\n\\input{" + filepath.ToSlash(strings.TrimSuffix(p, ".tex")) + "}\n" +
		after + "\n\\end{document}\n"

	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	// The document survives the runaway on both sides of the file.
	if !strings.Contains(txt, before) || !strings.Contains(txt, after) {
		t.Fatalf("the document was swallowed by the runaway; got %q", txt)
	}
	// The scanned text does not come back onto the page…
	if strings.Contains(txt, inside) {
		t.Errorf("the runaway argument's text was reinserted and typeset; got %q", txt)
	}
	// …and neither does the engine's own splice.
	for _, internal := range []string{"UseHook", "gotex@endload", "gotexeatdate", "endofpackagehook"} {
		if strings.Contains(txt, internal) {
			t.Errorf("the loader's internal splice %q reached the page: %q", internal, txt)
		}
	}
}
