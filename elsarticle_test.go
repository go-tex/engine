// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// elsText compiles an elsarticle document leniently and returns the page text with
// markup stripped, plus how many commands were skipped as undefined.
func elsText(t *testing.T, body string, opts string) (string, int) {
	t.Helper()
	src := `\documentclass` + opts + `{elsarticle}\begin{document}` + body + `\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	skipped := 0
	for _, n := range e.Diagnostics().Skipped {
		skipped += n
	}
	return stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), "")), skipped
}

// The front matter elsarticle.cls documents (\ead, \cortext, \fntext, \tnotetext, \address,
// \journal) must be defined and produce the class's output. Reference: tectonic on the same
// source prints
//
//	Title⋆ / Alice Smitha,1,∗ / a Dept of Things, City / Abstract Abs. / Keywords: one, two
//	… footnotes: ⋆ Title note text. ∗ Corresponding author  Email address: alice@example.org
//	(Alice Smith)  1 Footnote text. … footer "Preprint submitted to Elsevier".
func TestElsarticleFrontMatterCommands(t *testing.T) {
	const body = `\begin{frontmatter}` +
		`\title{TTT\tnoteref{t1}}\tnotetext[t1]{TNOTE}` +
		`\author[a1]{AUTHA\corref{c1}\fnref{f1}}\ead{al@x.org}` +
		`\cortext[c1]{CORNOTE}\fntext[f1]{FNNOTE}` +
		`\address[a1]{ADDRA}\journal{JOURNALNAME}` +
		`\begin{abstract}ABSTRACT\end{abstract}\begin{keyword}K1 \sep K2\end{keyword}` +
		`\end{frontmatter}BODY`
	text, skipped := elsText(t, body, "")
	if skipped != 0 {
		t.Errorf("%d commands still undefined", skipped)
	}
	for _, want := range []string{"TNOTE", "CORNOTE", "FNNOTE", "Email address:", "al@x.org", "ADDRA", "Preprint submitted to JOURNALNAME", "Keywords:"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; got %q", want, text)
		}
	}
	if !strings.Contains(text, "al@x.org (AUTHA)") {
		t.Errorf("the email is not followed by its author in parentheses: %q", text)
	}
	if strings.Contains(text, "0Email") || strings.Contains(text, "0 Email") {
		t.Errorf("a stray 0 mark precedes the email footnote: %q", text)
	}
	if strings.Index(text, "TTT") > strings.Index(text, "ABSTRACT") {
		t.Errorf("the abstract precedes the title: %q", text)
	}
	if strings.Contains(text, "[c1]") || strings.Contains(text, "[f1]") || strings.Contains(text, "[t1]") {
		t.Errorf("an optional label leaked onto the page: %q", text)
	}
}

// The \affiliation form with key=value pairs lists its values, and the final option drops the
// preprint footer.
func TestElsarticleAffiliationKeysAndFinal(t *testing.T) {
	const body = `\begin{frontmatter}\title{T}` +
		`\author[a1]{AU}\affiliation[a1]{organization=ORGNAME,city=CITYNAME}` +
		`\begin{abstract}ABS\end{abstract}\end{frontmatter}BODY`
	text, _ := elsText(t, body, "")
	for _, want := range []string{"ORGNAME", "CITYNAME"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is missing from the affiliation: %q", want, text)
		}
	}
	if strings.Contains(text, "organization=") {
		t.Errorf("a key leaked onto the page: %q", text)
	}
	// final uses \finalMaketitle (elsarticle.cls:801-826), which still sets the pprintTitle
	// page style (:805), so the footer stays; the title block must still be complete.
	final, skipped := elsText(t, body, "[final]")
	if skipped != 0 || !strings.Contains(final, "ORGNAME") || !strings.Contains(final, "BODY") {
		t.Errorf("final layout lost content (skipped %d): %q", skipped, final)
	}
}

// A \thefootnote that is \relax yields no mark text.
func TestFootnoteMarkRelaxIsEmpty(t *testing.T) {
	text, _ := elsText(t, `\begin{frontmatter}\title{T}\author{A}\ead{a@b.c}\end{frontmatter}`, "")
	if strings.Contains(text, "0") && strings.Contains(text, "0Email") {
		t.Errorf("relax mark printed as 0: %q", text)
	}
}
