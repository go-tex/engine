// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A class may call LaTeX's own \@sect rather than this engine's \@startsection
// path, and the copernicus/EGU family does:
//
//	\def\section{...\@dblarg{\@sect{section}{1}{\z@}{...}{...}{...}}}
//
// Undefined, lenient mode skipped it WITH ITS EIGHT ARGUMENTS — so every heading
// of such a document vanished. 2304.06058 came out with not one of its fifteen
// section titles against a reference that numbers them 1 to 9.
func TestAtSectTypesetsTheHeading(t *testing.T) {
	src := `\documentclass{article}\makeatletter` +
		`\def\mysection{\@dblarg{\@sect{section}{1}{\z@}{10pt}{5pt}{\bfseries}}}` +
		`\makeatother\begin{document}` +
		`\mysection{Un titre reperable}Du texte.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "Untitrereperable") && !strings.Contains(got, "Un titre reperable") {
		t.Errorf("the heading is not on the page: %q", got)
	}
	// …and it is NUMBERED, as the unstarred form must be.
	if !strings.Contains(got, "1") {
		t.Errorf("the heading carries no number: %q", got)
	}
	if n := e.skippedCS["@sect"]; n != 0 {
		t.Errorf("\\@sect was skipped %d times; it is defined now", n)
	}
}

// \@ssect is its starred sibling: same spacing, no number.
func TestAtSsectTypesetsAnUnnumberedHeading(t *testing.T) {
	src := `\documentclass{article}\makeatletter` +
		`\def\mysection{\@ssect{\z@}{10pt}{5pt}{\bfseries}}` +
		`\makeatother\begin{document}` +
		`\mysection{Sans numero}Du texte.\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := pageChars(e); !strings.Contains(strings.ReplaceAll(got, " ", ""), "Sansnumero") {
		t.Errorf("the heading is not on the page: %q", got)
	}
}
