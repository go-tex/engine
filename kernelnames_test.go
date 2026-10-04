// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ Two kernel internals existed under PRIVATE names, so anything that called LaTeX's own
// names found them undefined — and an undefined one is skipped, which loses content silently.
//
//	\restore@protect   latex.ltx:1371, one line: \let\protect\@@protect
//	\@normalcr         latex.ltx:6453, the internal name of \\ (\let\\\@normalcr)
//
// The engine had \gotex@restore@protect / \gotex@@protect, structurally identical to the
// kernel's pair but invisible to a class that calls \restore@protect itself. 2607.18707 skips
// \restore@protect four times and \@normalcr three times.
//
// Both witnesses below were checked against tectonic, and both show the absence LOSING
// something rather than merely leaving a name unset.

func kernelNameText(t *testing.T, body string) string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+body), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(e.RenderPages(e.renderMargin(0)), "")
}

func kernelNameGlyphs(t *testing.T, body string) int {
	t.Helper()
	return strings.Count(kernelNameText(t, body), "<path")
}

// \@normalcr absent, \\ bound through it is undefined and skipped: the break vanishes and the
// words JOIN. Measured against tectonic, which sets "AAA BBB"; without this it was "AAABBB".
//
// ⛔ The assertion counts BASELINES, not glyphs. A line break draws nothing of its own — it
// changes where the next word starts — so a glyph count is identical either way, and a first
// version of this test passed with \@normalcr removed. The rendered page carries one y per
// line: three without the break, four with it.
func svgBaselines(t *testing.T, body string) int {
	t.Helper()
	svg := kernelNameText(t, body)
	seen := map[string]bool{}
	for i := 0; i+3 < len(svg); i++ {
		if svg[i:i+3] != `y="` {
			continue
		}
		j := i + 3
		for j < len(svg) && svg[j] != '"' {
			j++
		}
		seen[svg[i+3:j]] = true
	}
	return len(seen)
}

func TestNormalcrIsTheInternalNameOfTheLineBreak(t *testing.T) {
	broken := svgBaselines(t, `\let\zzbreak\@normalcr\makeatother`+
		`\begin{document}AAA\zzbreak BBB\end{document}`)
	joined := svgBaselines(t, `\makeatother\begin{document}AAA BBB\end{document}`)
	if broken != joined+1 {
		t.Errorf("%d baseline(s) with \\@normalcr against %d for the same words unbroken: "+
			"the line break was lost", broken, joined)
	}
}

// ⛔ \restore@protect absent, the \protect it should have restored stays
// \@unexpandable@protect — and the command it guards is then LOST. Measured against tectonic,
// which sets "AAA OKBBB"; without this it was "AAA BBB", with the OK gone entirely.
func TestRestoreProtectPutsProtectBack(t *testing.T) {
	const body = `\def\zzcmd{OK}\let\@@protect\relax` +
		`\let\protect\@unexpandable@protect\restore@protect\makeatother` +
		`\begin{document}AAA \protect\zzcmd BBB\end{document}`
	// A, A, A, O, K, B, B, B and the page number: nine. Without the restore the OK is lost
	// and only seven are drawn.
	if n := kernelNameGlyphs(t, body); n != 9 {
		t.Errorf("%d glyph path(s), want 9 (AAA OKBBB and the page number): the protected "+
			"command was lost", n)
	}
}

// And the two switches the kernel sets \protect with (latex.ltx:1353-1354), which a class
// calls around a display it writes out. They must exist and must not error.
func TestTheProtectSwitchesExist(t *testing.T) {
	const body = `\set@display@protect\set@typeset@protect\makeatother` +
		`\begin{document}Z\end{document}`
	// Z and the page number. A skipped switch would leave its name in the paragraph.
	if n := kernelNameGlyphs(t, body); n != 2 {
		t.Errorf("%d glyph path(s), want 2 (Z and the page number)", n)
	}
}
