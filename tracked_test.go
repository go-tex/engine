// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// executableMagic is the first bytes of the formats a built Go binary can have.
var executableMagic = map[string][]byte{
	"Mach-O 64-bit":       {0xcf, 0xfa, 0xed, 0xfe},
	"Mach-O 32-bit":       {0xce, 0xfa, 0xed, 0xfe},
	"Mach-O universal":    {0xca, 0xfe, 0xba, 0xbe},
	"ELF":                 {0x7f, 'E', 'L', 'F'},
	"PE (Windows .exe)":   {'M', 'Z'},
	"WebAssembly (.wasm)": {0x00, 'a', 's', 'm'},
}

// No built binary may be tracked. Two were: a 5.8MB `gotex` that has been in the
// tree since the CLI landed, and a 3.5MB `gotex-linediff` that `go build
// ./cmd/gotex-linediff` dropped at the repository root — with no -o, that is
// where it goes, named after the directory — and that a `git add -A` then swept
// up. A clone pays for both, on every platform, forever.
//
// This is the control, not the .gitignore: an ignore rule naming a binary stops
// working the moment the command is renamed or a new one is added, and it does
// nothing at all about a file already tracked. Reading the bytes of what git
// actually holds cannot be out of date.
func TestNoBinaryIsTracked(t *testing.T) {
	files := trackedFiles(t)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil || len(b) < 4 {
			continue
		}
		for name, magic := range executableMagic {
			// A .wasm fixture is content, not a build artefact; the ignore rule
			// keeps built ones out and a tracked one is deliberate.
			if name == "WebAssembly (.wasm)" && strings.HasSuffix(f, ".wasm") {
				continue
			}
			if bytes.HasPrefix(b, magic) {
				t.Errorf("%s is tracked and is a %s executable (%d bytes); build it with -o into a scratch directory", f, name, len(b))
			}
		}
	}
}

// Every command under cmd/ builds, with no -o, to a file of its own name at the
// repository root. Each one needs its line in .gitignore, and a command added
// without one is how the last binary got committed.
func TestEveryCommandIsIgnoredAtTheRoot(t *testing.T) {
	ignore, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Skip("no .gitignore here")
	}
	lines := map[string]bool{}
	for _, l := range strings.Split(string(ignore), "\n") {
		lines[strings.TrimSpace(l)] = true
	}
	dirs, err := os.ReadDir("cmd")
	if err != nil {
		t.Skip("no cmd/ directory here")
	}
	var checked int
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		// ⛔ A DIRECTORY UNDER cmd/ IS NOT NECESSARILY A COMMAND, and this test
		// used to assume it was. cmd/internal/ holds packages SHARED BY the
		// commands (loadguard); `go build` produces no root binary for them, so
		// demanding a .gitignore line would assert something false — and adding
		// "/internal" to .gitignore to quiet this would ignore any internal
		// directory at the root into the bargain.
		//
		// The invariant is unchanged and is in the test's own name: every
		// COMMAND is ignored at the root. Only the way a command is recognised
		// moved, from "is a directory" to "declares package main".
		if !holdsPackageMain(t, filepath.Join("cmd", d.Name())) {
			continue
		}
		checked++
		if !lines["/"+d.Name()] {
			t.Errorf("cmd/%s builds to ./%s but .gitignore has no \"/%s\" line", d.Name(), d.Name(), d.Name())
		}
	}
	// ⛔ And the control, because a premise that narrowed could narrow to nothing:
	// a run that recognised no command at all would pass this test in silence.
	if checked == 0 {
		t.Fatal("no directory under cmd/ was recognised as a command: the package-main " +
			"test is wrong and this guard is checking nothing")
	}
}

// holdsPackageMain reports whether dir contains a non-test .go file declaring
// package main — that is, whether `go build ./dir` yields a binary.
func holdsPackageMain(t *testing.T, dir string) bool {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(l) == "package main" {
				return true
			}
		}
	}
	return false
}

// trackedFiles asks git what it holds. A tree git cannot be asked about is not a
// failure — the package must test the same outside a checkout — but it is said
// out loud, because a guard that silently tests nothing is worse than none.
func trackedFiles(t *testing.T) []string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git on PATH: cannot ask what is tracked")
	}
	out, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Skip("not a git work tree: cannot ask what is tracked")
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" && !strings.HasPrefix(f, ".claude"+string(filepath.Separator)) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		t.Fatal("git ls-files named no file at all; this guard would pass on an empty repository")
	}
	return files
}
