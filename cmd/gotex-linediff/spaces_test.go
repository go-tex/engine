// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ⛔ An interword space in a JUSTIFIED line is routinely narrower than a sixth of
// the size, which is what this reader used to require before putting a space back.
// Computer Modern's space is a third of the size with a third of that to shrink, so
// a tight line puts it at 22% and a very tight one below 17%.
//
// Measured on arXiv 2405.20319, whole two-column lines came back as
// "guage,weintroduceParSEL:Parameterized…" while poppler read the same line with
// its spaces. The words had run together in the REPORT, not on the page — and since
// -align keys on words, every such line also cost that page its key.
// ⚠ The geometry here is MEASURED, not invented: it is the line
//
//	y=605.53  x0=51.81  LinLibertineO @8.97
//	guage, we introduce ParSEL: Parameterized Shape Editing with Lan-
//
// from this engine's own render of 2405.20319, a two-column acmart paper whose
// justified lines shrink their spaces to about an eighth of the size. A witness
// compiled at a narrow measure did NOT reproduce it — its spaces stayed above a
// sixth — so a test built on one passed with the old threshold in place and proved
// nothing.
func TestWordBreaksSurviveAShrunkenLine(t *testing.T) {
	const size = 8.97
	// 1.2pt between runs: 13% of the size — under the old sixth (1.495pt) and over
	// a twentieth (0.449pt). This is the band a justified line lives in.
	runs := []Run{
		{X: 51.81, Y: 186, W: 28, Size: size, Text: "guage,"},
		{X: 81.01, Y: 186, W: 10, Size: size, Text: "we"},
		{X: 92.21, Y: 186, W: 40, Size: size, Text: "introduce"},
	}
	l := lineOf(runs, 792)
	if want := "guage, we introduce"; l.Text != want {
		t.Errorf("ligne = %q, attendu %q — les espaces comprimées d'une ligne justifiée "+
			"sont perdues, et le rapport se lit comme un défaut de la page", l.Text, want)
	}
}

// The same reconstruction end to end, so the report itself is covered and not only
// the helper.
func TestTheReportPutsTheSpacesBack(t *testing.T) {
	dir := t.TempDir()
	src := `\documentclass{article}
\setlength{\textwidth}{150pt}
\begin{document}
\noindent alfa bravo kilo lima mike golf hotel zulu oscar papa romeo sierra tango
\end{document}`
	p := filepath.Join(dir, "a.pdf")
	if err := os.WriteFile(p, pdfOf(t, src), 0o644); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := run([]string{p}, reportOptions{page: 1}, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, w := range []string{"alfa", "bravo", "kilo", "sierra"} {
		if !strings.Contains(out, w) {
			t.Fatalf("%s absent du rapport:\n%s", w, out)
		}
	}
	for _, joined := range []string{"alfabravo", "bravokilo", "kilolima", "limamike"} {
		if strings.Contains(out, joined) {
			t.Errorf("les mots sont collés dans le rapport (%s):\n%s", joined, out)
		}
	}
}

// And the other way: a gap INSIDE a word — a kerning pair — must not become a
// space. A threshold low enough to catch a shrunk space has to stay above these.
func TestAKernDoesNotBecomeASpace(t *testing.T) {
	// Two runs 0.3pt apart at 10pt: 3% of the size, the scale of a kerning pair.
	runs := []Run{
		{X: 0, Y: 700, W: 20, Size: 10, Text: "Wa"},
		{X: 20.3, Y: 700, W: 20, Size: 10, Text: "ter"},
	}
	l := lineOf(runs, 792)
	if l.Text != "Water" {
		t.Errorf("un crénage est devenu une espace: %q", l.Text)
	}
	// A space at the same size: a third of 10pt, shrunk to a fifth.
	runs[1].X = 22
	if l := lineOf(runs, 792); l.Text != "Wa ter" {
		t.Errorf("une espace comprimée n'a pas été rendue: %q", l.Text)
	}
}
