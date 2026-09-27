// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \newtheorem{x}[y]{Z} shares y's counter, and \thex must DELEGATE to \they rather than
// print the raw register. latex.ltx:12728, \@othm:
//
//	\global\@namedef{the#1}{\@nameuse{the#2}}
//
// So \newtheorem{lemme}[theorem]{Lemme} gives \thelemme = \thetheorem, which is itself
// \thesection.\the\c@theorem when theorem was declared [section]. Printing \the\c@theorem
// dropped the section off the front.
//
// Judged against tectonic 0.17.0 on a witness with a section, a [section]-numbered theorem
// and a sharer, both carrying an optional note:
//
//	reference  Theorem 1.1 (MANOTE). CORPS UN  Lemme 1.2 (AUTRENOTE). CORPS DEUX
//	before     Theorem 1.1 (MANOTE). CORPS UN  Lemme 2   (AUTRENOTE). CORPS DEUX
//
// Fourteen corpus papers move on this: \newtheorem{x}[y]{Z} is a common idiom, not an
// acmart one.
func TestSharedTheoremCounterDelegatesItsNumber(t *testing.T) {
	const src = `\documentclass{article}\usepackage{amsthm}` +
		`\newtheorem{theorem}{Theorem}[section]\newtheorem{lemme}[theorem]{Lemme}` +
		`\begin{document}\section{Sec}` +
		`\begin{theorem}[MANOTE]BODYONE\end{theorem}` +
		`\begin{lemme}[AUTRENOTE]BODYTWO\end{lemme}` +
		`\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"Theorem 1.1", "Lemme 1.2", "(MANOTE)", "(AUTRENOTE)"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 80))
		}
	}
}

// acmart declares its whole theorem set at \AtEndPreamble under \if@ACM@acmthm
// (acmart.cls:3042-3070), and acmthm is TRUE by default (\ExecuteOptionsX{acmthm=true},
// acmart.cls:89). A paper that writes \begin{example} without declaring it is relying on the
// class — corpus paper 2402.04392 does, eleven times, and it does NOT bundle acmart.cls, so
// the emulation is what it gets. Its reference prints "Example 3.1", "3.4", "3.5".
//
// (That paper also corrects an earlier finding of mine: I had concluded from six papers that
// the acmart emulation was reachable by NO corpus paper. It is a seventh acmart paper, and it
// does not bundle its class.)
//
// Each declaration keeps acmart's own \@ifundefined guard, so a document's \newtheorem still
// wins — the second case below.
func TestAcmartDeclaresItsTheoremSet(t *testing.T) {
	const fromClass = `\documentclass{acmart}\begin{document}\section{Sec}` +
		`\begin{example}EXBODY\end{example}\begin{definition}DEFBODY\end{definition}` +
		`\end{document}`
	e, err := compile([]byte(fromClass), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	for _, env := range []string{"example", "definition"} {
		if got := e.Diagnostics().UndefinedEnvs[env]; got != 0 {
			t.Errorf("%s still reported undefined (%d)", env, got)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"Example 1.1", "Definition 1.2", "EXBODY", "DEFBODY"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 90))
		}
	}

	// A document's own declaration wins, which is what the \@ifundefined guard is for.
	const ownDecl = `\documentclass{acmart}\newtheorem{example}{Exemple}` +
		`\begin{document}\begin{example}OWN\end{example}\end{document}`
	e2, err := compile([]byte(ownDecl), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile own: %v", err)
	}
	t2 := stripSVGTags(strings.Join(e2.RenderPages(e2.renderMargin(0)), ""))
	if !strings.Contains(t2, "Exemple") {
		t.Errorf("the document's own heading lost to the class's; page reads %q", firstN(t2, 60))
	}
}
