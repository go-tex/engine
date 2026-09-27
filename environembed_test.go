// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// environ.sty is embedded because acmart.cls:284 requires it and builds five environments on
// \Collect@Body, abstract among them (acmart.cls:1661):
//
//	\renewenvironment{abstract}{\Collect@Body\@saveabstract}{}
//
// Nine corpus papers bundle the real acmart.cls, so they run the class rather than this
// engine's emulation, and for all nine \Collect@Body was undefined.
//
// ⛔ The cost was one LETTER. With \Collect@Body skipped, the \@saveabstract that follows it
// grabbed the first TOKEN of the body as its own argument and dropped it: 2311.00921's
// abstract began "tructured dense matrices result from …" where the reference reads
// "Structured". Measured, "Structured" went 0 -> 1 against the reference's 1. Σ cannot see a
// character, and neither can the channel census — \Collect@Body showed up there as one
// undefined command, which reads as costing nothing.
//
// The witness reproduces the class's shape rather than loading acmart: an environment whose
// begin-code is \Collect@Body followed by a one-argument macro.
func TestCollectBodyKeepsTheFirstToken(t *testing.T) {
	const src = `\documentclass{article}\usepackage{environ}\makeatletter` +
		`\long\def\@keepbody#1{[#1]}` +
		`\newenvironment{grabbed}{\Collect@Body\@keepbody}{}\makeatother` +
		`\begin{document}` +
		`\begin{grabbed}SENTINELSTART middle SENTINELEND\end{grabbed}` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := e.Diagnostics().Skipped["Collect@Body"]; n != 0 {
		t.Fatalf("\\Collect@Body is undefined (%d skipped): environ.sty did not load", n)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// ⛔ SENTINELSTART is the assertion. Without \Collect@Body the one-argument macro eats
	// the body's first token, and it is the FIRST token that vanishes — a defect that leaves
	// every other word of the paragraph in place.
	for _, want := range []string{"SENTINELSTART", "middle", "SENTINELEND"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 120))
		}
	}
}

// trimspaces.sty comes with environ (\RequirePackage{trimspaces}, environ.sty:16) and is
// embedded for it. The assertion is that it LOADED — \trim@spaces defined and trimming the
// trailing space — so a future change that drops the file fails here rather than in a corpus
// paper's abstract.
//
// ⛔ It does NOT assert the leading space, because the engine gets that wrong and this test is
// not the place to hide it. Measured against tectonic on the same three lines:
//
//	\edef\x{\trim@spaces{ PADDED }}  ->  tectonic "[PADDED]",  engine "[ PADDED]"
//
// The cause is NOT the obvious one. \trim@spaces trims through \romannumeral-`\q, and every
// piece of that was checked against tectonic and AGREES: \romannumeral of a negative number
// expands to nothing, \number-`\q is -113, the optional space after a scanned number is
// skipped, a macro is expanded while the scanner looks for more digits, and an expansion
// beginning with a space gives "A[ Z]" on both. The remaining suspects are the catcode-3 `Q`
// the package sets as its delimiter and \newcommand's grab of an argument that starts with a
// space. Left as its own change: acmart's abstract does not need it — the first-token defect
// above is fixed and the abstract now matches the reference.
func TestTrimspacesIsEmbeddedWithEnviron(t *testing.T) {
	const src = `\documentclass{article}\usepackage{environ}\makeatletter` +
		`\edef\gotexprobe{\trim@spaces{ PADDED }}\makeatother` +
		`\begin{document}[\gotexprobe]\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if n := e.Diagnostics().Skipped["trim@spaces"]; n != 0 {
		t.Errorf("\\trim@spaces is undefined (%d skipped): trimspaces.sty did not load", n)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	// The trailing space is trimmed; the leading one is the known divergence above.
	if !strings.Contains(text, "PADDED]") {
		t.Errorf("\\trim@spaces did not trim the trailing space; the page reads %q", firstN(text, 80))
	}
}
