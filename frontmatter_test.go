// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \begin{frontmatter} … \end{frontmatter} is elsarticle.cls:1279, and it is one line:
//
//	\newenvironment{frontmatter}{}{\maketitle}
//
// Undefined, \begin{frontmatter} resolved to \relax through \csname, the body was
// typeset as ordinary text, and \maketitle was NEVER CALLED — so the title and the
// author block did not appear at all. Seven corpus papers write their whole front
// matter that way (five elsarticle, one frontiersSCNS, one achemso), and only one of
// the eight bundles its class, so the emulation is what they get.
//
// Measured on the 154-paper corpus: Sigma 325 -> 322, +685 glyphs, nothing lost, and
// 2407.10372 lands exactly on 20.
func TestFrontmatterEmitsTheTitleBlock(t *testing.T) {
	const src = `\documentclass{elsarticle}\begin{document}` +
		`\begin{frontmatter}` +
		`\title{THETITLE}\author{THEAUTHOR}` +
		`\begin{abstract}THEABSTRACT\end{abstract}` +
		`\end{frontmatter}` +
		`THEBODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := e.Diagnostics().UndefinedEnvs["frontmatter"]; got != 0 {
		t.Errorf("frontmatter still reported undefined (%d)", got)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	for _, want := range []string{"THETITLE", "THEAUTHOR", "THEABSTRACT", "THEBODY"} {
		if !strings.Contains(svg, want) {
			t.Errorf("%q is not on the page", want)
		}
	}
}

// KNOWN and not fixed here: the ORDER. elsarticle boxes the abstract and \maketitle
// lays title, authors and abstract out together, so the reference renders
//
//	LE TITRE ICI A. Auteur Abstract RESUME Keywords: MOTCLE CORPS
//
// where this renders the abstract first and the title block after it, because the
// emulation's abstract typesets in place. The content is all there and its sequence is
// not; fixing that needs a deferred abstract, which is its own change and its own
// measurement. This test pins the CONTENT so that work cannot silently lose it.
func TestFrontmatterOrderIsKnownWrong(t *testing.T) {
	const src = `\documentclass{elsarticle}\begin{document}\begin{frontmatter}` +
		`\title{TTT}\begin{abstract}AAA\end{abstract}\end{frontmatter}BBB\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	svg := strings.Join(e.RenderPages(e.renderMargin(0)), "")
	iT, iA := strings.Index(svg, "TTT"), strings.Index(svg, "AAA")
	if iT < 0 || iA < 0 {
		t.Fatal("the witness lost its own markers")
	}
	if iT < iA {
		t.Log("the title now precedes the abstract — the deferred abstract landed, " +
			"update this test and TestFrontmatterEmitsTheTitleBlock's note")
	}
}
