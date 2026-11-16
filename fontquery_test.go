// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// say runs a document and returns what it put through \message.
func say(t *testing.T, body string) string {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(`\documentclass{article}\begin{document}` + body + `\end{document}`)
	if err != nil {
		t.Fatalf("run %q: %v", body, err)
	}
	return out
}

// ⛔ BOTH WERE ASSIGNABLE AND NEITHER WAS READABLE. \fontdimen2\font=8pt has
// worked since #365 and \hyphenchar since #767; `\the` of either produced the
// LITERAL TOKENS, so a document asking its own engine for a metric got
// "\fontdimen 2\font" where tectonic answers "3.33pt" (#762).
//
// ⭐ MEASURED AGAINST tectonic 0.17.0, one document, six questions. The first
// three are the control — ordinary dimension registers, which always worked —
// so a failure below is about fonts and not about \message or \the:
//
//	\the\textheight        550.0pt   550.0pt
//	\the\baselineskip       12.0pt    12.0pt
//	\the\parindent          15.0pt    15.0pt
//	\the\fontdimen2\font     3.33pt    3.33pt
//	\the\fontdimen6\font     10.0pt    10.0pt
func TestAFontsOwnQuantitiesAreReadable(t *testing.T) {
	got := say(t, `\message{[A=\the\textheight]}`+
		`\message{[B=\the\fontdimen2\font]}`+
		`\message{[C=\the\fontdimen6\font]}`)
	// The control first: if an ordinary register does not read back, nothing
	// below is about fonts.
	if !strings.Contains(got, "[A=550.0pt]") {
		t.Fatalf("the control register did not read back: %q", got)
	}
	for _, want := range []string{"[B=3.33pt]", "[C=10.0pt]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %q — tectonic gives those values for the default face", want, got)
		}
	}
}

// ⭐ \fontname ANSWERS THE QUESTION #449 AND #612 TURN ON, and before this the
// only way to find out which face was selected was to render to SVG and
// subtract glyph positions.
//
// Measured against tectonic 0.17.0, the whole ladder — ten for ten, the same
// file at every step:
//
//	\tiny         lmroman5-regular     \large  lmroman12-regular
//	\scriptsize   lmroman7-regular     \Large  lmroman12-regular
//	\footnotesize lmroman8-regular     \LARGE  lmroman17-regular
//	\small        lmroman9-regular     \huge   lmroman17-regular
//	\normalsize   lmroman10-regular    \ttfamily lmmono10-regular
//
// ⚠ The STRING differs and that is deliberate: tectonic prints
// `"[lmroman10-regular]:mapping=tex-text;" at 10.0pt`, where the brackets and
// the mapping are XeTeX's own file-request syntax (tuenc.def:101,104) rather
// than anything about the font. This engine resolves no .fd files and has no
// such request to quote, so it prints the file it opened. The part that carries
// the information is the same.
func TestFontnameSaysWhichOpticalSizeWasChosen(t *testing.T) {
	for _, tc := range []struct{ cmd, want string }{
		{`\tiny`, "lmroman5-regular"},
		{`\scriptsize`, "lmroman7-regular"},
		{`\footnotesize`, "lmroman8-regular"},
		{`\small`, "lmroman9-regular"},
		{`\normalsize`, "lmroman10-regular"},
		{`\large`, "lmroman12-regular"},
		{`\Large`, "lmroman12-regular"},
		{`\LARGE`, "lmroman17-regular"},
		{`\huge`, "lmroman17-regular"},
		{`\ttfamily`, "lmmono10-regular"},
	} {
		got := say(t, `{`+tc.cmd+`\message{[F=\fontname\font]}}`)
		if !strings.Contains(got, tc.want) {
			t.Errorf("%s: \\fontname said %q, want it to name %s", tc.cmd, got, tc.want)
		}
	}
}

// ⛔ \fontname IS A CONVERT PRIMITIVE AND CONVERT PRIMITIVES EXPAND (tex.web
// §468-472, where it sits with \string, \number and \jobname). Registered as an
// ordinary primitive it printed its own tokens in exactly the place it was
// wanted — inside \message — which is how the first version of this failed.
func TestFontnameExpands(t *testing.T) {
	got := say(t, `\message{[F=\fontname\font]}`)
	if strings.Contains(got, `\fontname`) {
		t.Errorf("\\fontname did not expand, it printed itself: %q", got)
	}
}

// §472 appends " at <size>pt" only when the face is used at other than its
// design size — so lmroman10 at 10pt prints bare and lmroman17 at 20.74pt
// carries the size.
func TestTheSizeSuffixFollowsTheDesignSize(t *testing.T) {
	if got := say(t, `\message{[F=\fontname\font]}`); strings.Contains(got, " at ") {
		t.Errorf("the default face is a 10pt design used at 10pt and must print bare: %q", got)
	}
	if got := say(t, `{\huge\message{[F=\fontname\font]}}`); !strings.Contains(got, " at ") {
		t.Errorf("\\huge is lmroman17 used at 20.74pt and must carry its size: %q", got)
	}
}

// \the\hyphenchar, the third of the family: assignable since #767, readable now.
func TestHyphencharIsReadable(t *testing.T) {
	got := say(t, `\message{[H=\the\hyphenchar\font]}`)
	if !strings.Contains(got, "[H=45]") {
		t.Errorf("\\the\\hyphenchar\\font = %q, want 45 (`\\- , what latex.ltx:523 "+
			"puts in \\defaulthyphenchar)", got)
	}
	got = say(t, `\hyphenchar\font=-1 \message{[H=\the\hyphenchar\font]}`)
	if !strings.Contains(got, "[H=-1]") {
		t.Errorf("after \\hyphenchar\\font=-1, \\the reads %q, want -1", got)
	}
}
