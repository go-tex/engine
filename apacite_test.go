// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// An apacite .bbl entry built the way apacite.bst writes one: an APACrefauthors
// environment for the names, then \APACrefYearMonthDay for the year,
// \APACrefatitle for the title, \APACjournalVolNumPages for the citation tail, and
// a run of short connectives (\BBA, \BCBL, \BPBI). apacite is commonly required by
// a journal class without the .sty being bundled; with it unloaded these macros are
// undefined and the lenient path GOBBLES their braced arguments, so the year,
// title, journal and pages vanish and only the author names survive. The stubs in
// apacite.go recover that content.
const apaciteBBL = `\begin{thebibliography}{}
\bibitem [\protect \citeauthoryear {%
Adeyemi%
\ \BBA {} Moore%
}{%
Adeyemi%
\ \BBA {} Moore%
}{%
{\protect \APACyear {2022}}%
}]{adeyemi2022}
\APACinsertmetastar {adeyemi2022}%
\begin{APACrefauthors}%
Adeyemi, B.%
\BCBT {}\ \BBA {} King, P\BPBI R.%
\end{APACrefauthors}%
\unskip\
\newblock
\APACrefYearMonthDay{2022}{}{}.
\newblock
{\BBOQ}\APACrefatitle {Determining effective permeability} {Determining effective permeability}.{\BBCQ}
\newblock
\APACjournalVolNumPages{Advances in Water Resources}{159}{}{104096}.
\PrintBackRefs{\CurrentBib}
\end{thebibliography}`

// When apacite is used but its .sty is absent, the stubs must recover the
// reference's content — year, title, journal, volume, pages — and leak none of the
// APAC/BB macro names or stray braces onto the page.
func TestApaciteStubsRecoverBibliographyContent(t *testing.T) {
	e, err := compile([]byte(
		`\documentclass{article}\usepackage{apacite}\begin{document}`+
			apaciteBBL+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)

	// The content-bearing fields, previously gobbled with their undefined macros.
	// pageChars concatenates glyphs and drops inter-word glue, so the wanted
	// fragments carry no spaces.
	for _, want := range []string{
		"Adeyemi",                  // author (survives even before, from the env body)
		"2022",                     // \APACrefYearMonthDay year — was dropped
		"Determining",              // \APACrefatitle title — was dropped
		"permeability",             // title tail
		"AdvancesinWaterResources", // \APACjournalVolNumPages journal — was dropped
		"159",                      // volume
		"104096",                   // pages
	} {
		if !strings.Contains(got, want) {
			t.Errorf("bibliography missing %q; apacite content not recovered.\ngot: %q", want, got)
		}
	}

	// Nothing may leak: no APAC/BB macro names, no back-reference plumbing, no the
	// stray braces the raw .bbl carries.
	for _, bad := range []string{
		"APACrefYearMonthDay", "APACrefatitle", "APACjournalVolNumPages",
		"APACinsertmetastar", "citeauthoryear", "PrintBackRefs", "CurrentBib",
		"BBOQ", "BBCQ", "BCBT", "BPBI", "APACyear", "{", "}",
	} {
		if strings.Contains(got, bad) {
			t.Errorf("bibliography leaks %q — an apacite macro/brace reached the page.\ngot: %q", bad, got)
		}
	}
}

// The stub loader must not clobber an apacite already recorded as loaded (the real
// bundled apacite.sty, which resolves and loads before the stub branch is reached):
// its early return leaves whatever that file defined in place. Here \APACrefatitle
// is pre-defined to typeset its FIRST argument (apacite prints the second); after
// loadApaciteStubs on an already-loaded apacite it must keep that first-arg meaning,
// proving the stub did not overwrite it.
func TestApaciteStubsDoNotOverrideAnAlreadyLoadedApacite(t *testing.T) {
	e, err := compile([]byte(
		`\documentclass{article}`+
			`\renewcommand{\APACrefatitle}[2]{#1}`+ // stand-in for the real package's own def
			`\begin{document}\APACrefatitle{FIRST}{second}\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	// Record apacite as already loaded (as a bundled apacite.sty would) and run the
	// stub loader: it must return early and define nothing.
	if e.loadedPackages == nil {
		e.loadedPackages = map[string]bool{}
	}
	e.loadedPackages["apacite"] = true
	if err := e.loadApaciteStubs(); err != nil {
		t.Fatalf("loadApaciteStubs: %v", err)
	}
	if got := pageChars(e); !strings.Contains(got, "FIRST") || strings.Contains(got, "second") {
		t.Errorf("stub loader clobbered an already-loaded apacite's \\APACrefatitle; got %q", got)
	}
}
