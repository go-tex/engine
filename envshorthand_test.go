// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// A shorthand built on another shorthand. lmcs.cls papers open their displays with
//
//	\def\ba{\begin{array}}      \def\ea{\end{array}}
//	\def\bda{\begin{displaymath}\ba}  \def\eda{\ea\end{displaymath}}
//
// so the closing \eda reaches \end through \ea — one hop further than the old
// expandsToEnd rule looked. Read raw, \eda was written into the maths source as a
// literal and the \end{displaymath} behind it never surfaced: the scanner ran to the
// end of the file and took the rest of the paper with it. 2603.13142 rendered 2 pages
// out of 17 that way, and reported "1 group still open at end of document".
//
// The OPENING half needs nothing here: \ba reaches the maths layer as a literal, the
// layer refuses it, and renderMathResolvingMacros substitutes the parameterless macro
// and retries — measured, expanding \ba in the scanner too left the witness at the
// same 40 glyph paths, so that half was dropped rather than shipped.
func TestEnvShorthandChainClosesTheDisplay(t *testing.T) {
	src := `\documentclass{article}
\def\ba{\begin{array}}
\def\ea{\end{array}}
\def\bda{\begin{displaymath}\ba}
\def\eda{\ea\end{displaymath}}
\begin{document}
BEFOREMARK

\bda{lcl}
a & = & b
\eda

AFTERMARK
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	txt := mvlText(e.mvl)
	if !strings.Contains(txt, "BEFOREMARK") {
		t.Errorf("the text before the display is missing; got %q", txt)
	}
	if !strings.Contains(txt, "AFTERMARK") {
		t.Errorf("\\eda did not close the display: everything after it was swallowed; got %q", txt)
	}
	if len(e.mathDropped) != 0 {
		t.Errorf("the maths layer refused the array (%v) — \\ba reached it as a literal", e.mathDropped)
	}
	if n := len(e.groups); n != 0 {
		t.Errorf("%d group(s) still open at the end of the document, want 0 — the scanner ran past \\end{document}", n)
	}
}

// expandsToCloseCS must follow a chain of shorthands and nothing else. The bound is
// what stops \def\a{\b}\def\b{\a}, and the narrowness at every hop — no parameters,
// and the next hop is the FIRST token — is what keeps an ordinary content macro, or a
// macro that merely mentions \end mid-body, verbatim in the scanned source.
func TestExpandsToCloseCSFollowsAChainAndStops(t *testing.T) {
	e := New()
	e.LoadLaTeX()
	def := func(name string, body ...tok) { e.define(name, &meaning{kind: mMacro, body: body}, false) }
	endArray := []tok{csTok("end"), chTok('{', catBegin), chTok('a', catLetter), chTok('}', catEnd)}

	def("ea", endArray...)                          // 1 hop: \ea → \end
	def("eda", csTok("ea"))                         // 2 hops: \eda → \ea → \end
	def("eda3", csTok("eda"))                       // 3 hops
	def("eda4", csTok("eda3"))                      // 4 hops
	def("eda5", csTok("eda4"))                      // 5 hops — past the bound
	def("dc", csTok("]"))                           // the \[…\] scanner's own close
	def("dc2", csTok("dc"))                         // …reached through a chain too
	def("mid", chTok('x', catLetter), csTok("end")) // \end, but not first
	def("arg", csTok("end"))                        // parameterless body…
	e.define("argp", &meaning{kind: mMacro, params: []tok{chTok('#', catParam)}, body: []tok{csTok("end")}}, false)

	for _, c := range []struct {
		name  string
		cs    string
		close string
		want  bool
	}{
		{"one hop", "ea", "end", true},
		{"two hops", "eda", "end", true},
		{"four hops", "eda4", "end", true},
		{"five hops is past the bound", "eda5", "end", false},
		{"a \\] shorthand", "dc", "]", true},
		{"a \\] shorthand through a chain", "dc2", "]", true},
		{"an \\end that is not the first token", "mid", "end", false},
		{"a macro WITH a parameter", "argp", "end", false},
		{"an undefined cs", "nosuchcsxyz", "end", false},
		{"the wrong close cs", "arg", "begin", false},
	} {
		if got := e.expandsToCloseCS(csTok(c.cs), c.close); got != c.want {
			t.Errorf("%s: expandsToCloseCS(\\%s, %q) = %v, want %v", c.name, c.cs, c.close, got, c.want)
		}
	}
}
