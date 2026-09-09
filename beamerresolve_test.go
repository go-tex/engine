package engine

import (
	"strings"
	"testing"
)

// \documentclass{beamer} takes the real class when the file can be RESOLVED and
// the built-in emulation when it cannot. There is nothing to opt into: whether a
// caller supplied beamer is the whole question, and it is one the engine can
// answer by looking.
func TestBeamerFollowsWhetherTheClassResolves(t *testing.T) {
	// A stand-in beamer.cls: enough to be loaded, and marked so the test can see
	// which path ran.
	const fakeClass = "\\def\\zzbeamermark{VRAIE}\\def\\frame#1{#1}\n"

	t.Run("resolvable → la vraie classe", func(t *testing.T) {
		opt := Options{Resolve: func(name string) ([]byte, bool) {
			if name == "beamer.cls" {
				return []byte(fakeClass), true
			}
			return nil, false
		}}
		e, err := buildEngine(opt, true)
		if err != nil {
			t.Fatal(err)
		}
		out, err := e.Run("\\documentclass{beamer}\\message{[\\zzbeamermark]}")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if out != "[VRAIE]" {
			t.Errorf("output = %q — the resolved class was not loaded", out)
		}
	})

	t.Run("absent -> the emulation", func(t *testing.T) {
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatal(err)
		}
		out, err := e.Run("\\documentclass{beamer}\\message{[\\zzbeamermark]}")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		// The emulation defines no such macro, and a lenient run skips it.
		if strings.Contains(out, "VRAIE") {
			t.Errorf("output = %q — the real class was loaded although it is absent", out)
		}
	})

	// GOTEX_BEAMER=0 forces the emulation even when the class resolves, which is
	// how the two paths are compared.
	t.Run("GOTEX_BEAMER=0 forces the emulation", func(t *testing.T) {
		t.Setenv("GOTEX_BEAMER", "0")
		opt := Options{Lenient: true, Resolve: func(name string) ([]byte, bool) {
			if name == "beamer.cls" {
				return []byte(fakeClass), true
			}
			return nil, false
		}}
		e, err := buildEngine(opt, true)
		if err != nil {
			t.Fatal(err)
		}
		out, err := e.Run("\\documentclass{beamer}\\message{[\\zzbeamermark]}")
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if strings.Contains(out, "VRAIE") {
			t.Errorf("output = %q — GOTEX_BEAMER=0 should force the emulation", out)
		}
	})
}
