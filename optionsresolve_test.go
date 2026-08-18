package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// Options.Resolve is what a host with no filesystem needs: in js/wasm every
// os.ReadFile fails, so without it a browser host can compile nothing beyond the
// small base set embedded in the binary.
//
// The assertions read \message output rather than the rendered page, because the
// SVG is vector paths — there is no literal text in it to grep for.

// runWithResolve builds a LaTeX-capable engine on opt and runs src.
func runWithResolve(t *testing.T, opt Options, src string) string {
	t.Helper()
	e, err := buildEngine(opt, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

// A package that exists nowhere on disk still loads when the host answers for it.
func TestResolveSuppliesAPackage(t *testing.T) {
	asked := ""
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "zzhostpkg.sty" {
			asked = name
			return []byte("\\def\\zzhostword{RESOLU}\n"), true
		}
		return nil, false
	}}
	out := runWithResolve(t, opt, "\\usepackage{zzhostpkg}\\message{[\\zzhostword]}")
	if asked != "zzhostpkg.sty" {
		t.Fatalf("le résolveur n'a pas été interrogé pour zzhostpkg.sty (reçu %q)", asked)
	}
	if out != "[RESOLU]" {
		t.Errorf("sortie = %q, attendu %q", out, "[RESOLU]")
	}
}

// \input goes through the same seam: a package that splits itself across files
// would otherwise load only its first one.
func TestResolveSuppliesAnInputFile(t *testing.T) {
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		switch name {
		case "zzhostmain.sty":
			return []byte("\\input{zzhostpart.tex}\n"), true
		case "zzhostpart.tex":
			return []byte("\\def\\zzhostword{PARTIE}\n"), true
		}
		return nil, false
	}}
	out := runWithResolve(t, opt, "\\usepackage{zzhostmain}\\message{[\\zzhostword]}")
	if out != "[PARTIE]" {
		t.Errorf("sortie = %q, attendu %q — le fichier \\input'é par le paquet résolu n'a pas suivi", out, "[PARTIE]")
	}
}

// A class, not only a package.
func TestResolveSuppliesAClass(t *testing.T) {
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "zzhostcls.cls" {
			return []byte("\\def\\zzhostword{CLASSE}\n"), true
		}
		return nil, false
	}}
	out := runWithResolve(t, opt, "\\documentclass{zzhostcls}\\message{[\\zzhostword]}")
	if out != "[CLASSE]" {
		t.Errorf("sortie = %q, attendu %q", out, "[CLASSE]")
	}
}

// The search path wins: a file the process CAN read overrides the host, so a
// document that ships its own copy of a package still gets it.
func TestSearchPathBeatsResolve(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zzboth.sty"), []byte("\\def\\zzhostword{DISQUE}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTEX_TEXMF", dir)
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "zzboth.sty" {
			return []byte("\\def\\zzhostword{HOTE}\n"), true
		}
		return nil, false
	}}
	out := runWithResolve(t, opt, "\\usepackage{zzboth}\\message{[\\zzhostword]}")
	if out != "[DISQUE]" {
		t.Errorf("sortie = %q, attendu %q — le chemin de recherche doit primer", out, "[DISQUE]")
	}
}

// The host wins over the base set embedded in the binary, so a host that carries
// a real article.cls gets its own rather than the built-in one.
func TestResolveBeatsTheEmbeddedSet(t *testing.T) {
	if _, ok := embeddedTeXFile("article.cls"); !ok {
		t.Fatal("témoin: article.cls devrait être embarqué")
	}
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "article.cls" {
			return []byte("\\def\\zzhostword{ARTICLE HOTE}\n"), true
		}
		return nil, false
	}}
	out := runWithResolve(t, opt, "\\documentclass{article}\\message{[\\zzhostword]}")
	if out != "[ARTICLE HOTE]" {
		t.Errorf("sortie = %q, attendu %q — le résolveur doit primer sur l'embarqué", out, "[ARTICLE HOTE]")
	}
}

// A host that declines a name falls through to the embedded set, and a nil
// resolver changes nothing at all: both compile the same document.
func TestResolveDecliningFallsThroughToEmbedded(t *testing.T) {
	called := 0
	src := []byte("\\documentclass{article}\\begin{document}A\\end{document}")
	for _, opt := range []Options{
		{Resolve: func(string) ([]byte, bool) { called++; return nil, false }},
		{},
	} {
		if _, err := CompileToSVGPages(src, opt); err != nil {
			t.Fatalf("le repli sur l'embarqué a échoué: %v", err)
		}
	}
	if called == 0 {
		t.Error("le résolveur n'a jamais été interrogé")
	}
}

// End to end through the public API, in STRICT mode: without the host the
// document does not compile at all, with it the same source succeeds. That is
// the whole point of the seam, stated as a pass/fail.
func TestResolveMakesAStrictCompileSucceed(t *testing.T) {
	src := []byte("\\documentclass{article}\\usepackage{zzhoststrict}\\begin{document}\\zzhostword\\end{document}")
	if _, err := CompileToSVGPages(src, Options{}); err == nil {
		t.Fatal("témoin: sans résolveur, ce document devrait échouer")
	}
	opt := Options{Resolve: func(name string) ([]byte, bool) {
		if name == "zzhoststrict.sty" {
			return []byte("\\def\\zzhostword{X}\n"), true
		}
		return nil, false
	}}
	if _, err := CompileToSVGPages(src, opt); err != nil {
		t.Errorf("avec le résolveur, la compilation stricte échoue encore: %v", err)
	}
}

// hostTeXFile reports where the bytes came from; the display path is what the
// engine shows for a loaded file.
func TestHostTeXFileReportsItsOrigin(t *testing.T) {
	e := New()
	if _, _, ok := e.hostTeXFile("zzabsent.sty"); ok {
		t.Error("un nom inconnu ne devrait pas se résoudre")
	}
	if _, path, ok := e.hostTeXFile("article.cls"); !ok || path != "<embedded>/article.cls" {
		t.Errorf("article.cls → (%q, %v), attendu <embedded>/article.cls", path, ok)
	}
	e.resolve = func(name string) ([]byte, bool) { return []byte("x"), name == "zzhote.sty" }
	if _, path, ok := e.hostTeXFile("zzhote.sty"); !ok || path != "<host>/zzhote.sty" {
		t.Errorf("zzhote.sty → (%q, %v), attendu <host>/zzhote.sty", path, ok)
	}
}
