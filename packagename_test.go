// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// \usepackage, \documentclass and \LoadClass EXPAND their {name}.
//
// Checked against real LaTeX in one line: \def\nm{marker}\usepackage{zz\nm} loads
// zzmarker.sty there, and loaded nothing here.
//
// beamer builds every theme file name that way — \beamer@calltheme does
// \usepackage[{#1}]{beamertheme\beamer@themename} — so \usetheme{default}
// (beamer.cls) asked for a package literally named "beamertheme\beamer@themename"
// and NO beamer theme had ever been loaded. The frametitle template lives in
// beamerouterthemedefault.sty, which is why a frame title never reached the page.

func TestPackageNameIsExpanded(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zzmarker.sty"),
		[]byte("\\ProvidesPackage{zzmarker}\n\\def\\ZZLOADED{oui}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOTEX_TEXMF", dir)

	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\documentclass{article}\makeatletter\def\nm{marker}\usepackage{zz\nm}` +
		`\message{[\ifdefined\ZZLOADED\ZZLOADED\else NON\fi]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "[oui]") {
		t.Errorf("got %q — \\usepackage{zz\\nm} did not resolve to zzmarker.sty", out)
	}
}
