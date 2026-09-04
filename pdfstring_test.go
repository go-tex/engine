// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"bytes"
	"strings"
	"testing"
)

// tokensToPlainText resolves accents (both the accent-command and literal-UTF-8
// forms), drops font switches and unknown commands keeping their text, maps known
// text commands to their symbols, and collapses whitespace.
func TestTokensToPlainText(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	cases := []struct{ in, want string }{
		{`Plain title`, "Plain title"},
		{`R\'esum\'e`, "Résumé"},                             // accent commands
		{"Th\\'eor\\`eme de C\\^ot\\'e", "Théorème de Côté"}, // several accents incl. grave
		{`Na\"{\i}ve`, "Naïve"},                              // \"{\i} — braced dotless-i base
		{`A \textbf{bold} word`, "A bold word"},              // wrapper expands; \bfseries dropped
		{`\LaTeX{} and \TeX`, "LaTeX and TeX"},               // known commands
		{`Cost 50\% off`, "Cost 50% off"},                    // \%
		{`A\thanks{grant} title`, "A title"},                 // \thanks gobbles its arg
		{`Stra\ss e`, "Straße"},                              // \ss
		{`\O sterreich`, "Østerreich"},                       // \O
		{`weird \unknowncmd here`, "weird here"},             // unknown command dropped, text kept
		{`a~b`, "a b"},                                       // tie → space
	}
	for _, c := range cases {
		e.Run(`\def\@x{` + c.in + `}`)
		got := e.macroPlainText("@x")
		if got != c.want {
			t.Errorf("plain(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// pdfusetitle derives the PDF /Title and /Author from \title/\author, cleaned.
func TestPDFUseTitleDerivesInfo(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\usepackage{hyperref}\hypersetup{pdfusetitle}` +
		`\title{On Widgets}\author{Ada Lovelace}\begin{document}Body.\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `/Title (On Widgets)`) || !strings.Contains(out, `/Author (Ada Lovelace)`) {
		t.Errorf("pdfusetitle did not derive /Title and /Author from \\title/\\author")
	}
}

// An explicit pdftitle overrides pdfusetitle's derived value.
func TestPDFUseTitleExplicitWins(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\hypersetup{pdfusetitle,pdftitle={Explicit}}` +
		`\title{Derived}\begin{document}Body.\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `/Title (Explicit)`) || strings.Contains(out, `(Derived)`) {
		t.Errorf("explicit pdftitle should override the pdfusetitle-derived title")
	}
}

// An accented title written with accent commands, derived via pdfusetitle, reaches
// the PDF as UTF-16BE — clean, not a backslash-laden byte string.
func TestPDFUseTitleAccentCommandsUTF16(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run(`\documentclass{article}\hypersetup{pdfusetitle}` +
		`\title{R\'esum\'e}\begin{document}Body.\end{document}`); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "<FEFF") {
		t.Errorf("derived accented title was not written UTF-16BE")
	}
	if strings.Contains(out, `\'e`) {
		t.Errorf("accent commands leaked into the PDF /Title")
	}
}

// A section heading written with accent commands now reads cleanly in the PDF
// bookmark sidebar (the fix to #204/#207 for the accent-command case): the outline
// title is UTF-16BE and carries no accent commands.
func TestBookmarkAccentCommandTitleIsClean(t *testing.T) {
	fp := pdfFontOrSkip(t)
	e := New()
	e.LoadLaTeX()
	e.Run(`\font\rm={` + fp + `} at 12pt \rm`)
	if _, err := e.Run("\\documentclass{article}\\begin{document}" +
		"\\section{Th\\'eor\\`eme}Contenu.\\end{document}"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := e.RenderPDF(&buf, 24); err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/Outlines") || !strings.Contains(out, "<FEFF") {
		t.Errorf("the accent-command bookmark title was not written UTF-16BE")
	}
	if strings.Contains(out, `\'e`) || strings.Contains(out, "Th\\") {
		t.Errorf("accent commands leaked into the bookmark /Title")
	}
}
