// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ An environment NAME can carry any rune, and the catcode table is 256 wide.
//
// setCurrentEnv records \@currenvir with the name's own characters, at their current
// category codes, so a class can \ifx-compare it (beamer's \frame does). It indexed
// e.catcode[r] directly — and \begin{毕}, one Chinese character of the kind an author
// writes in their own name, crashed the engine:
//
//	panic: runtime error: index out of range [27605] with length 256
//
// It was reached from a real paper: 2607.05497 writes
// \author[orcid=…]{Jiaqing Bi \begin{CJK*}{UTF8}{gbsn}(毕嘉擎)\end{CJK*}}, and the
// CJK* body's own characters end up in a name once aas.cls re-expands the field.
// e.catOf is the engine's own accessor and gives any rune past 255 catOther, which is
// what the tokenizer does for every other character it reads.

func TestAnEnvironmentNameBeyondLatin1DoesNotPanic(t *testing.T) {
	// Without the fix this does not fail an assertion — it panics, and the panic names
	// the rune's code point.
	e, err := compile([]byte(`\documentclass{article}`+
		`\begin{document}ALPHA\begin{毕}BETA\end{毕}GAMMA\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(e.RenderPages(e.renderMargin(0))); n == 0 {
		t.Fatal("no page produced")
	}
	// The body of an environment the engine lacks still reaches the page: the frame is
	// what is lost, not the content.
	if d := e.Diagnostics(); d.OpenGroups != 0 {
		t.Errorf("OpenGroups = %d, want 0", d.OpenGroups)
	}
}

// And the name is RECORDED, not dropped: \@currenvir must hold it, or a class that
// picks its syntax with \ifx\@currenvir… takes the wrong branch.
func TestAnEnvironmentNameBeyondLatin1IsRecordedInCurrenvir(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\makeatletter`+
		`\newenvironment{毕}{}{}`+
		`\begin{document}\begin{毕}\message{[\@currenvir]}\end{毕}\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := e.Diagnostics().Messages; !strings.Contains(got, "[毕]") {
		t.Errorf("\\@currenvir = %q, want it to hold 毕", got)
	}
}
