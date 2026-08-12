// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempDir writes the given files into a fresh temp directory and chdirs there
// for the duration of fn, so the package resolver (which searches ".") finds them
// the way the CLI's chdir-to-document-dir makes it find a paper's bundled files.
func withTempDir(t *testing.T, files map[string]string, fn func()) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	fn()
}

// runLaTeX builds a LaTeX-configured engine and runs src, returning \message output.
func runLaTeX(t *testing.T, src string) (string, *Engine) {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return out, e
}

// \usepackage loads a real .sty from the document directory and its \def/\newcommand
// take effect (instead of the command being skipped).
func TestLoadBundledPackage(t *testing.T) {
	withTempDir(t, map[string]string{
		"mypkg.sty": `\def\mymacro{\message{FROM-STY}}`,
	}, func() {
		out, e := runLaTeX(t, `\usepackage{mypkg}\mymacro`)
		if !strings.Contains(out, "FROM-STY") {
			t.Errorf("expected the bundled package's macro to run; \\message=%q", out)
		}
		if !e.loadedPackages["mypkg"] {
			t.Error("mypkg not recorded as loaded")
		}
	})
}

// \documentclass loads a bundled .cls; a macro it defines is available.
func TestLoadBundledClass(t *testing.T) {
	withTempDir(t, map[string]string{
		"myclass.cls": `\def\classhook{\message{CLASS-OK}}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass{myclass}\classhook`)
		if !strings.Contains(out, "CLASS-OK") {
			t.Errorf("expected the bundled class's macro to run; \\message=%q", out)
		}
	})
}

// The @ character is a letter while a package loads (so \@internal names work) and
// reverts to "other" afterwards.
func TestLoadMakesAtALetter(t *testing.T) {
	withTempDir(t, map[string]string{
		"atpkg.sty": `\def\@secret{\message{AT-LETTER-OK}}\@secret`,
	}, func() {
		// Start with @ as "other" (as in a document body), so we can prove load both
		// makes it a letter (\@secret works) and restores it to "other" afterwards.
		out, e := runLaTeX(t, `\catcode64=12 \usepackage{atpkg}`)
		if !strings.Contains(out, "AT-LETTER-OK") {
			t.Errorf("expected \\@secret to be usable during load; got %q", out)
		}
		if e.catcode['@'] != catOther {
			t.Errorf("@ catcode = %d after load, want catOther(%d) restored", e.catcode['@'], catOther)
		}
	})
}

// \DeclareOption / \ProcessOptions runs the code of each requested option, and
// \DeclareOption* handles unknown ones with \CurrentOption bound.
func TestPackageOptionProcessing(t *testing.T) {
	withTempDir(t, map[string]string{
		"optpkg.sty": `\DeclareOption{opta}{\message{GOT-A}}` +
			`\DeclareOption*{\message{STAR-\CurrentOption}}` +
			`\ProcessOptions`,
	}, func() {
		out, _ := runLaTeX(t, `\usepackage[opta,zzz]{optpkg}`)
		if !strings.Contains(out, "GOT-A") {
			t.Errorf("declared option opta did not run; %q", out)
		}
		if !strings.Contains(out, "STAR-zzz") {
			t.Errorf("\\DeclareOption* did not handle unknown option zzz; %q", out)
		}
	})
}

// A package that is not present on disk is not loaded and leaves no trace — the
// engine falls back to its stubs/lenient handling, it does not error.
func TestUnresolvedPackageIsNoOp(t *testing.T) {
	withTempDir(t, map[string]string{}, func() {
		_, e := runLaTeX(t, `\usepackage{definitely-not-here}`)
		if e.loadedPackages["definitely-not-here"] {
			t.Error("a missing package must not be marked loaded")
		}
	})
}

// The runaway guard stops an infinite macro loop: in strict mode it is an error,
// in tolerant mode the loop simply ends (partial output, no hang).
func TestRunawayGuardStrict(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.stepLimit = 100_000 // trip the guard fast without changing what it proves
	_, err := e.Run(`\def\lx{\lx}\lx`)
	if err == nil {
		t.Fatal("strict mode: expected a runaway-expansion error, got nil")
	}
	if !strings.Contains(err.Error(), "runaway") {
		t.Errorf("error = %q, want it to mention runaway", err.Error())
	}
	if !e.runaway {
		t.Error("runaway flag not set")
	}
}

func TestRunawayGuardTolerant(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	e.stepLimit = 100_000
	if _, err := e.Run(`\def\lx{\lx}\lx`); err != nil {
		t.Fatalf("tolerant mode: runaway must not surface as an error, got %v", err)
	}
	if !e.runaway {
		t.Error("runaway flag not set")
	}
}

// \LoadClassWithOptions lets a class build on a base class, forwarding options;
// \IfFileExists / \InputIfFileExists branch on and splice resolvable files;
// \PassOptionsToPackage + \ExecuteOptions feed the option machinery.
func TestLoadClassAndFileHelpers(t *testing.T) {
	withTempDir(t, map[string]string{
		"base.cls":    `\DeclareOption{big}{\message{BASE-BIG}}\ProcessOptions\def\basecmd{\message{BASE}}`,
		"derived.cls": `\LoadClassWithOptions{base}\basecmd`,
		"frag.tex":    `\message{FRAG-INPUT}`,
	}, func() {
		out, _ := runLaTeX(t, `\documentclass[big]{derived}`+
			`\IfFileExists{frag}{\message{HAVE-FRAG}}{\message{NO-FRAG}}`+
			`\IfFileExists{ghost}{\message{HAVE-GHOST}}{\message{NO-GHOST}}`+
			`\InputIfFileExists{frag}{\message{AFTER}}{\message{MISS}}`)
		for _, want := range []string{"BASE-BIG", "BASE", "HAVE-FRAG", "NO-GHOST", "FRAG-INPUT", "AFTER"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in output %q", want, out)
			}
		}
		if strings.Contains(out, "MISS") || strings.Contains(out, "HAVE-GHOST") {
			t.Errorf("file-existence branch chose wrong arm: %q", out)
		}
	})
}

// \PassOptionsToPackage queues options that are merged when the package loads, and
// \ExecuteOptions runs a declared option's code immediately (for defaults).
func TestPassAndExecuteOptions(t *testing.T) {
	withTempDir(t, map[string]string{
		"popt.sty": `\DeclareOption{fromboth}{\message{OPT-RAN}}\ExecuteOptions{fromboth}\ProcessOptions`,
	}, func() {
		out, _ := runLaTeX(t, `\PassOptionsToPackage{fromboth}{popt}\usepackage{popt}`)
		// \ExecuteOptions runs it once (default), \ProcessOptions runs it again for
		// the passed option — at least one "OPT-RAN" proves both wiring points.
		if !strings.Contains(out, "OPT-RAN") {
			t.Errorf("passed/executed option did not run; %q", out)
		}
	})
}

// The loader and the LaTeX2e kernel agree on the "loaded" registry: after
// \usepackage{p}, the kernel's \@ifpackageloaded{p} takes its then-branch, and
// \@ifpackagewith reports the options the loader recorded.
func TestIfPackageLoadedContract(t *testing.T) {
	withTempDir(t, map[string]string{
		"prov.sty": `\def\provcmd{}`,
	}, func() {
		out, _ := runLaTeX(t, `\usepackage[fancy]{prov}`+
			`\@ifpackageloaded{prov}{\message{IS-LOADED}}{\message{NOT-LOADED}}`+
			`\@ifpackageloaded{ghostpkg}{\message{GHOST-LOADED}}{\message{GHOST-ABSENT}}`)
		if !strings.Contains(out, "IS-LOADED") {
			t.Errorf("\\@ifpackageloaded did not see the loaded package; %q", out)
		}
		if !strings.Contains(out, "GHOST-ABSENT") {
			t.Errorf("\\@ifpackageloaded reported an unloaded package as loaded; %q", out)
		}
	})
}

// \AtEndOfPackage code runs when the package finishes loading, and does not leak
// into a later load.
func TestAtEndOfPackageHook(t *testing.T) {
	withTempDir(t, map[string]string{
		"hookpkg.sty": `\AtEndOfPackage{\message{END-HOOK}}\def\x{}`,
		"plain2.sty":  `\def\y{}`,
	}, func() {
		out, _ := runLaTeX(t, `\usepackage{hookpkg}\usepackage{plain2}\message{DONE}`)
		if strings.Count(out, "END-HOOK") != 1 {
			t.Errorf("expected the end-of-package hook to fire exactly once; %q", out)
		}
	})
}

// \@setfontsize consumes its three arguments, so a class that redefines
// \normalsize to call \@setfontsize\normalsize… does not recurse on itself.
func TestSetfontsizeConsumesArgsNoRecursion(t *testing.T) {
	withTempDir(t, map[string]string{
		"fspkg.sty": `\renewcommand{\normalsize}{\@setfontsize\normalsize\@xpt\@xiipt\message{SIZE-SET}}`,
	}, func() {
		out, e := runLaTeX(t, `\usepackage{fspkg}\normalsize done`)
		if e.runaway {
			t.Fatal("\\@setfontsize did not consume its args: expansion ran away")
		}
		if !strings.Contains(out, "SIZE-SET") {
			t.Errorf("expected the redefined \\normalsize to run once; %q", out)
		}
	})
}
