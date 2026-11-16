// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⭐ THE EMBEDDED natbib.sty IS LOADED, AND ITS AUTHOR-YEAR CITATIONS REACH THE
// PAGE. #703: `\citet{k}` came out `[1]` where the reference writes "Nordbotten
// et al. (2021)" — the author's NAME, the thing a reader of an author-year paper
// is looking at, simply absent. 20 of the 154 measured papers `\usepackage{natbib}`
// and none of them ships it, so all twenty fell back to the engine's numeric
// emulation.
//
// What fixes it is not more Go: #712 measured that given the genuine natbib.sty
// the engine already produces `tectonic`'s output word for word. The file was
// merely not there — absent from texmf/ AND from the bundle catalogue, so no
// document could reach it by any path.
//
// ⛔ TWO ASSERTIONS AT TWO LAYERS, because either alone can pass for the wrong
// reason. `\NAT@parse` is an internal only the real file defines, so it says the
// FILE ARRIVED; the page text says its definitions TOOK EFFECT. A stub that
// loads and does nothing would pass the first and fail the second, which is the
// shape this repository has been bitten by (`\def\mark#1{}` cost 27 papers their
// running head while every layer above it was correct).
// ⚠ A \documentclass is REQUIRED here and is not decoration: natbib's \citet
// calls the kernel's \@citex, which only a loaded class defines. Without one the
// document dies on "Undefined control sequence \@citex" — which is also why the
// README records \@citex as a thing this engine does not execute on its own.
func TestEmbeddedNatbibPutsTheAuthorNameOnThePage(t *testing.T) {
	const src = `\documentclass{article}
\usepackage[authoryear]{natbib}
\makeatletter\message{[PARSE=\meaning\NAT@parse]}\makeatother
\begin{document}
As obtained by \citet{nord2021} and also \citep{nord2021}.
\begin{thebibliography}{1}
\bibitem[Nordbotten et al.(2021)]{nord2021} J.~M. Nordbotten.
\newblock A title.
\newblock 2021.
\end{thebibliography}
\end{document}`

	// ⛔ compile(), NOT Run(): natbib resolves a citation from \bibcite in the
	// .aux, which exists only on the SECOND pass. A single Run renders both
	// citations as `?` — and the SVG renderers are no use here either, because
	// they emit text glyph by glyph, so no word is a contiguous substring of an
	// SVG page. compile() is what every exported CompileTo… calls, so this is
	// the real two-pass path, read through the node list.
	ce, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	page := pageText(ce)
	t.Logf("page text: %q", page)

	// Layer 1: the file arrived. \NAT@parse exists only in natbib.sty.
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	msgs, err := e.Run(src)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.Contains(msgs, "[PARSE=undefined]") {
		t.Fatalf("\\NAT@parse is undefined, so the embedded natbib.sty was not loaded at "+
			"all — nothing below can mean anything. messages: %s", msgs)
	}

	// Layer 2: its definitions reached the page. ⛔ THE OBSERVABLE IS "et al.",
	// NOT the author's surname, and that distinction is the whole test: an
	// earlier version asserted "Nordbotten" and PASSED on a run where both
	// citations rendered `?` and `(?)`, because the surname is also in the
	// BIBLIOGRAPHY ENTRY, which needs no citation machinery at all. "et al." is
	// written nowhere in the source except natbib's own \bibitem label, so only
	// a resolved author-year citation can put it on the page.
	// ⚠ "etal", not "et al": this engine's text layer carries no interword
	// spaces (a page reads "Asobtainedby…"), so a term with a space in it
	// matches nothing and would fail for a reason that has nothing to do with
	// natbib. That cost one wrong assertion before this comment existed.
	if n := strings.Count(page, "etal"); n < 2 {
		t.Errorf("%q appears %d time(s), want 2 (one per citation): \\citet and \\citep "+
			"did not render an author-year label, so natbib's \\bibcite did not survive "+
			"the .aux round-trip or the numeric emulation won (#703)", "etal", n)
	}
}
