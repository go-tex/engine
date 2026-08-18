package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// An UNDELIMITED macro argument takes exactly one token and cannot run away, so
// the splicer's file-end sentinels must NOT be refused there. Refusing them broke
// the LaTeX kernel's own
//
//	\AtEndOfPackage#1{\g@addto@macro\@endofpackagehook{#1}}
//
// whose FIRST argument IS \@endofpackagehook: the call was abandoned, so the
// {code} that followed executed on the spot instead of being stored, and the hook
// stayed empty.
//
// The consequence went all the way to the page. etoolbox registers its catcode
// restore that way — \AtEndOfPackage{\etb@catcodes\undef\etb@catcodes} — so the
// catcode 3 it gives "|" was never undone; beamerbasedecode's |-delimited
// \beamer@@decodefind then never matched its argument, ran away, and beamer's
// comment skipper swallowed every document. A real beamer talk rendered ZERO
// pages; it now renders.
func TestAtEndOfPackageDefersItsCode(t *testing.T) {
	dir := t.TempDir()
	// The hook must run AFTER the body, and the catcode it sets must survive.
	body := `\message{[1]}` +
		"\\AtEndOfPackage{\\message{[3-hook]}\\catcode`\\|=12\\relax}" +
		"\\catcode`\\|=3 " +
		`\message{[2]}`
	if err := os.WriteFile(filepath.Join(dir, "hooked.sty"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TEXINPUTS", dir)
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	out, err := e.Run(`\message{[A]}\usepackage{hooked}`)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := "[A] [1] [2] [3-hook]"; out != want {
		t.Errorf("\\AtEndOfPackage ran at the wrong moment:\n got %q\nwant %q", out, want)
	}
	if e.catcode['|'] != catOther {
		t.Errorf("the catcode the hook restored did not stick: | = %d, want %d", e.catcode['|'], catOther)
	}
}

// The accumulator itself: a hook takes several contributions, in order, and holds
// them rather than running them.
func TestEndOfPackageHookAccumulates(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"stores-rather-than-runs", `\makeatletter\AtEndOfPackage{\message{H}}\message{[\meaning\@endofpackagehook]}`,
			`[macro:->\message {H}]`},
		{"two-contributions", `\makeatletter\AtEndOfPackage{\message{A}}\AtEndOfPackage{\message{B}}\message{[\meaning\@endofpackagehook]}`,
			`[macro:->\message {A}\message {B}]`},
		// \AtEndOfClass and \AtEndDocument use the same accumulator and were fine;
		// they are pinned here so the three stay consistent.
		{"class-hook", `\makeatletter\AtEndOfClass{\message{C}}\message{[\meaning\@endofclasshook]}`,
			`[macro:->\message {C}]`},
		{"document-hook", `\makeatletter\AtEndDocument{\message{D}}\message{[\meaning\@enddocumenthook]}`,
			`[macro:->\message {D}]`},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: %s\n = %q, want %q", c.name, c.src, got, c.want)
		}
	}
}
