// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A \makeatletter in the PREAMBLE must survive \begin{document}. latex.ltx's \document
// (latex.ltx:6672-6712) assigns no catcode at all, so whatever the preamble leaves is what
// the body gets.
//
// ⛔ This engine loads its formats with the at-sign as a LETTER and never hands it back
// (classkernel.go and amssubstrate.go both end that way), so \document forced catcode 12 at
// begin-document to compensate — and that also undid a \makeatletter the preamble had asked
// for and never revoked. Measured against tectonic on five lines:
//
//	\documentclass{article}\makeatletter\begin{document}[\the\catcode`\@]
//	  tectonic 11    this engine 12
//
// 21 of the 154 corpus papers open \makeatletter in their preamble.
func TestMakeatletterInThePreamblePersistsIntoTheBody(t *testing.T) {
	const src = "\\documentclass{article}\n\\makeatletter\n\\begin{document}\n" +
		"[c=\\the\\catcode`\\@]\n\\end{document}"
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "c=11") {
		t.Errorf("the preamble's \\makeatletter did not survive \\begin{document}; page reads %q",
			firstN(text, 60))
	}
}

// ⛔ And a document that did NOT ask keeps the at-sign OTHER, which is what every other
// corpus paper relies on: the formats leave it a letter and \document has to hand it back.
// Without this half the change would make @ a letter for all 154 papers.
func TestAtSignStaysOtherWhenTheDocumentDidNotAsk(t *testing.T) {
	const src = "\\documentclass{article}\\begin{document}\n[c=\\the\\catcode`\\@]\n\\end{document}"
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "c=12") {
		t.Errorf("the at-sign is a letter in a document that never asked; page reads %q",
			firstN(text, 60))
	}
}

// \makeatother in the preamble revokes the request, so the body gets the at-sign back as
// OTHER — the ordinary shape of a paper that patches an internal and tidies up after itself.
func TestMakeatotherInThePreambleRevokesIt(t *testing.T) {
	const src = "\\documentclass{article}\n\\makeatletter\n\\makeatother\n\\begin{document}\n" +
		"[c=\\the\\catcode`\\@]\n\\end{document}"
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	if !strings.Contains(text, "c=12") {
		t.Errorf("\\makeatother did not revoke the request; page reads %q", firstN(text, 60))
	}
}
