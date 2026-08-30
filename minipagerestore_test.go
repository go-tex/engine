// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A body collected up front assumes the environment's \end is IN the input. It need
// not be: beamer's rounded block opens \begin{minipage} inside \beamerboxesrounded and
// only produces the matching \end later, from \endbeamerboxesrounded — several tokens
// into a macro the scanner cannot see through. The scanner drained what was pending,
// carried on into the document text and swallowed the rest of it: a talk with one
// rounded block rendered ZERO pages (issue #115).
//
// When the \end never comes, the scan puts back everything it read — a locally wrong
// box, not a lost document.
func TestAMinipageWhoseEndNeverComesDoesNotSwallowTheDocument(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	// The box is never placed, so anything the scan drags into it is off the page.
	if _, err := e.Run(`\hsize=300pt\setbox0=\vbox{\begin{minipage}{100pt}DEDANS}APRESTEXTE\par`); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := mvlText(e.mvl); !strings.Contains(got, "APRESTEXTE") {
		t.Errorf("la page contient %q, want APRESTEXTE — le scanner l'a emporté dans la boîte", got)
	}
}

// A minipage that IS terminated keeps working: its body goes in the box, and the box
// is as wide as it was asked to be.
func TestATerminatedMinipageStillBoxesItsBody(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\hsize=300pt\setbox0=\hbox{\begin{minipage}{100pt}X\end{minipage}}` +
		`\message{[largeur \the\wd0]}`)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := trimNL(out); !strings.Contains(got, "[largeur 100.0pt]") {
		t.Errorf("= %q, want the box to be 100.0pt wide", got)
	}
}
