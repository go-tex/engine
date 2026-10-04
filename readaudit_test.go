// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

// A document names the files it reads, and both of TeX's read paths reach the filesystem
// with no restriction: \input{/etc/hostname} typesets that file and \openin/\read brings
// it back as tokens. That is ordinary TeX, and not a divergence from the reference —
// tectonic reads an absolute path with a warning — but TeX Live has openin_any and this
// engine has no equivalent. Confining reads would change what callers get; REPORTING them
// changes nothing and lets a host compiling third-party documents refuse the result.
// Audit: go-tex/engine#553.

func TestAReadOutsideTheDocumentsTreeIsReported(t *testing.T) {
	outside := t.TempDir() // a directory the document does not live in
	secret := filepath.Join(outside, "canary.tex")
	if err := os.WriteFile(secret, []byte("CANARY"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	t.Chdir(tree)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`A\input{`+filepath.ToSlash(secret)+`}B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := e.Diagnostics().ReadsOutsideTree
	if len(got) == 0 {
		t.Fatalf("an absolute \\input was not reported: %v", got)
	}
	var found bool
	for p := range got {
		if filepath.Clean(p) == filepath.Clean(secret) {
			found = true
		}
	}
	if !found {
		t.Errorf("reported %v, which does not name %s", got, secret)
	}
}

// ⛔ The other direction matters more than the first: a report that fires on every
// document says nothing, and a host that learned to ignore it would be worse off than
// with no report at all. A document that stays where it lives must come back empty.
func TestADocumentThatStaysInItsTreeReportsNothing(t *testing.T) {
	tree := t.TempDir()
	if err := os.WriteFile(filepath.Join(tree, "part.tex"), []byte("PART"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "sub", "deep.tex"), []byte("DEEP"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tree)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\input{part}\input{sub/deep}X\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().ReadsOutsideTree; len(got) != 0 {
		t.Errorf("a document reading only its own files reported %v", got)
	}
}

// And the walk-out form, which is the one a path-traversal attempt would use.
func TestAParentTraversalIsReported(t *testing.T) {
	parent := t.TempDir()
	if err := os.WriteFile(filepath.Join(parent, "above.tex"), []byte("ABOVE"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(parent, "doc")
	if err := os.MkdirAll(tree, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(tree)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`A\input{../above}B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().ReadsOutsideTree; len(got) == 0 {
		t.Errorf("../above was not reported as a read outside the tree")
	}
}

// ⛔ And a directory the HOST configured is part of the tree, not a reach-out. Judging
// against the working directory alone reported /…/texmf/size10.clo — a class file the
// host pointed the engine at with GOTEX_TEXMF — and a report that fires on the host's own
// configuration is one a host learns to ignore, which is worse than no report.
func TestASearchPathTheHostConfiguredIsNotAReachOut(t *testing.T) {
	texmf := t.TempDir() // stands in for a GOTEX_TEXMF directory, outside the document
	if err := os.WriteFile(filepath.Join(texmf, "hostpart.tex"), []byte("HOST"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	t.Chdir(tree)
	t.Setenv("GOTEX_TEXMF", texmf)

	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`A\input{hostpart}B\end{document}`), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// The file was read from outside the working directory and must NOT be reported.
	if got := e.Diagnostics().ReadsOutsideTree; len(got) != 0 {
		t.Errorf("a read from the configured search path was reported as a reach-out: %v", got)
	}
}
