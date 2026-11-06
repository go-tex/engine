// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ A NUMBERED HEADING SETS ITS TITLE ONCE.
//
// \@gxnum calls \csname #1mark\endcsname{⟨title⟩} on every numbered heading, exactly
// as latex.ltx:12796 does. The marks are meant to swallow that argument —
// latex.ltx:12870-12872 is \let\sectionmark\@gobble — and here they did not, because
// \@gobble is defined in LaTeX2eKernelHelpers (kernelhelpers.go:71), which LoadLaTeX
// loads AFTER the block that did the \let. \let copied an UNDEFINED meaning, so all
// three marks stayed undefined for the whole run, an undefined control sequence met
// its argument, and lenient mode set that argument as text.
//
// Measured against tectonic:
//
//	\subsection{BBBTITREBBB}
//	reference   "0.1 BBBTITREBBB"   @11.96
//	before      "BBBTITREBBB 0.1 BBBTITREBBB"   @9.96
//	after       "0.1 BBBTITREBBB"   @11.96
//
// — the size came back with the duplicate, because the stray run was body type and
// came first.
//
// ⭐ The assertion counts OCCURRENCES rather than comparing positions, because that
// is the defect: the title was set twice, in two different sizes, and no single
// coordinate says so.
func TestANumberedHeadingSetsItsTitleOnce(t *testing.T) {
	const marker = "ZZQQTITREQQZZ"
	for _, cmd := range []string{"section", "subsection", "subsubsection"} {
		src := `\documentclass{article}\begin{document}Avant.\` + cmd +
			`{` + marker + `}Apres.\end{document}`
		pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatalf(`\%s: %v`, cmd, err)
		}
		// The SVG puts one <tspan> per word, so the marker appears once per time the
		// title is set — and the title is one word on purpose.
		if n := strings.Count(strings.Join(pages, ""), marker); n != 1 {
			t.Errorf(`\%s: le titre est composé %d fois, want 1 — `+
				`\%smark doit avaler son argument (latex.ltx:12870-12872)`,
				cmd, n, cmd)
		}
	}
}

// ⛔ THE MARKS ARE DEFINED, and this is the half that names the cause rather than the
// symptom. A \let from a control sequence that does not exist yet produces an
// undefined meaning, silently, and the same mistake made anywhere else in this
// kernel would read exactly like the symptom above.
func TestTheSectioningMarksAreDefined(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	if _, err := e.Run(`\message{[\meaning\sectionmark|\meaning\subsectionmark|` +
		`\meaning\subsubsectionmark]}`); err != nil {
		t.Fatal(err)
	}
	got := trimNL(e.out.String())
	if strings.Contains(got, "undefined") {
		t.Errorf("une marque de sectionnement est indéfinie: %s — \\let depuis un "+
			"\\@gobble pas encore chargé copie « undefined »", got)
	}
}

// ⛔ AND THE CONTROL: \section* never touched a mark, so it must not move. \@gxstar
// goes straight to \@gxhead. If this test ever fails alongside the first, the fix
// went into the wrong place.
func TestAStarredHeadingSetsItsTitleOnceToo(t *testing.T) {
	const marker = "XXQQTITREQQXX"
	src := `\documentclass{article}\begin{document}Avant.\section*{` + marker +
		`}Apres.\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(strings.Join(pages, ""), marker); n != 1 {
		t.Errorf(`\section*: le titre est composé %d fois, want 1`, n)
	}
}
