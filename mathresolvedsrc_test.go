// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// mathPhrases returns the phrases the text layer stands over the formulas of a page.
func mathPhrases(t *testing.T, src string) []string {
	t.Helper()
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+src+`\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch c := n.(type) {
			case mathNode:
				out = append(out, c.src)
			case *boxNode:
				walk(c.list)
			}
		}
	}
	walk(e.mvl)
	return out
}

// renderMathResolvingMacros expands the commands go-tex/math rejects, so a
// document's own \newcommand renders — but the phrase the text layer stands over the
// formula kept the UNEXPANDED source, and said "\RealXtrain" where the page shows an
// X with a subscript. One corpus paper carries 226 of them. The rendering was always
// right; only the phrase was not (#372).
func TestMathPhraseIsWhatWasDrawn(t *testing.T) {
	got := mathPhrases(t, `\newcommand{\F}{\mathbb{F}}A $\F$ B`)
	if len(got) != 1 {
		t.Fatalf("expected one formula, got %d: %q", len(got), got)
	}
	if strings.Contains(got[0], `\F`) {
		t.Errorf("phrase = %q, still carries the unexpanded macro", got[0])
	}
	if !strings.Contains(got[0], "F") {
		t.Errorf("phrase = %q, want it to carry the F the page shows", got[0])
	}
}

// A formula go-tex/math understands on its own is never expanded — the literal
// source is always tried first — so its phrase is untouched.
func TestMathPhraseUntouchedWhenNothingIsExpanded(t *testing.T) {
	got := mathPhrases(t, `A $\frac{a}{b}$ B`)
	if len(got) != 1 {
		t.Fatalf("expected one formula, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], `\frac`) {
		t.Errorf("phrase = %q, want the source kept as written", got[0])
	}
}
