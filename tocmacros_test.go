// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ A CONTENTS ENTRY KEEPS THE MACROS ITS TITLE WAS WRITTEN WITH.
//
// An entry's title is stored detokenized (readTitleGroupBoth), so
// `\section{M\'ethodes}` arrives at the contents list as the seven characters
// `M\'ethodes`. tocTokens.text appended each rune with its live catcode, which made
// the backslash a character token of category ESCAPE — inert once tokenizing is
// over. The `\` drew nothing and the `'` behind it became a right quote by the
// ordinary ligature rule.
//
// Measured against tectonic on one heading:
//
//	source        \section{M\'ethodes et \`apres et \^etre et \"ou}
//	the heading    Méthodes et àpres et être et öu      ← already right
//	the contents   M’ethodes et ‘apres et etre et "ou   ← the defect
//	tectonic       Méthodes et àpres et être et öu
//
// Every accent, and every other macro a title carries, was lost on the way into the
// contents list while the heading set them correctly — which is what made it easy to
// miss: the page is right everywhere except the one line that repeats it.
func TestAContentsEntryKeepsItsAccents(t *testing.T) {
	const src = `\documentclass{article}\begin{document}\tableofcontents` +
		`\section{M\'ethodes et \` + "`" + `apres et \^etre et \"ou}Texte.\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(pages, "")
	// The accented words must appear TWICE: once in the contents, once in the
	// heading. Counting is what distinguishes "the contents entry is right" from
	// "the heading is right and the entry is missing".
	for _, w := range []string{"é", "à", "ê", "ö"} {
		if n := strings.Count(all, w); n < 2 {
			t.Errorf("%q apparaît %d fois, want au moins 2 (la table des matières "+
				"ET le titre)", w, n)
		}
	}
	// And the mangled forms must not be there at all.
	for _, bad := range []string{"M’e", "‘a"} {
		if strings.Contains(all, bad) {
			t.Errorf("%q est sur la page: le backslash a été posé comme CARACTÈRE "+
				"de catcode escape et l'apostrophe est devenue un guillemet", bad)
		}
	}
}

// ⛔ A CONTROL WORD EATS ITS TRAILING SPACES AND A CONTROL SYMBOL DOES NOT
// (tex.web §354-356). Without that half, `\LaTeX{} et` would lose its space or
// `\'e` would swallow the letter after it.
func TestTheContentsTokenizerFollowsTeXsControlSequenceRule(t *testing.T) {
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	b := &tocTokens{e: e}
	b.text(`A\'eB \relax  C\"oD`)
	var got []string
	for _, tk := range b.ts {
		if tk.cs_ {
			got = append(got, `\`+tk.cs)
			continue
		}
		got = append(got, string(tk.ch))
	}
	const want = `A \' e B   \relax C \" o D`
	if strings.Join(got, " ") != want {
		t.Errorf("got %q\nwant %q", strings.Join(got, " "), want)
	}
}
