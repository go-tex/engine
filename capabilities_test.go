// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This package compiles UNTRUSTED input. A .tex is a program and this is its interpreter:
// the README offers lenient mode as a preview for "a real third-party paper", the fidelity
// work runs it over arXiv sources, and the playground runs it on whatever a visitor pastes.
//
// Two properties hold today and are worth holding on purpose rather than by accident:
// a document cannot run a process, and it cannot write a file. Both are properties of the
// package's SOURCE — there is no API that could express them — so the test reads the
// source. A plausible-looking commit that reaches for os/exec to shell out to a rasteriser,
// or for os.WriteFile to cache something, fails here and has to argue the case.
//
// cmd/ is deliberately out of scope: the CLI writes the PDF it was asked for.

var forbiddenInEngine = []string{
	// running a process — TeX's \write18 and anything like it
	`"os/exec"`, "exec.Command", "exec.CommandContext", "syscall.Exec", "syscall.ForkExec",
	// writing, creating or removing a file
	"os.Create", "os.WriteFile", "os.OpenFile", "os.Remove", "os.RemoveAll",
	"os.MkdirAll", "os.Mkdir", "os.Rename", "os.Truncate", "os.Chmod",
}

func TestTheEngineCanNeitherRunAProcessNorWriteAFile(t *testing.T) {
	ents, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var scanned int
	for _, e := range ents {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(".", n))
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, bad := range forbiddenInEngine {
			if i := strings.Index(string(b), bad); i >= 0 {
				line := 1 + strings.Count(string(b[:i]), "\n")
				t.Errorf("%s:%d uses %s — a document compiled by this package must not be able to "+
					"run a process or write a file; if this is deliberate, say so here and in the "+
					"docs before allowing it", n, line, bad)
			}
		}
	}
	// ⛔ A scanner that read nothing would pass. Both halves of this guard are asserted:
	// that it saw the package, and that its matcher can actually find a violation.
	if scanned < 50 {
		t.Fatalf("scanned only %d source files — the scan, not the package, is what passed", scanned)
	}
	planted := "func x() { _ = os.Create }"
	var found bool
	for _, bad := range forbiddenInEngine {
		if strings.Contains(planted, bad) {
			found = true
		}
	}
	if !found {
		t.Fatal("the matcher does not detect a planted os.Create: the list or the test is wrong")
	}
}
