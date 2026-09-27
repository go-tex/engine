// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \IEEEauthorblockN and \IEEEauthorblockA were undefined, so a conference paper's names and
// affiliations were skipped as commands and their arguments ran into ONE paragraph. The
// class defines them as pass-throughs outside conference mode (IEEEtran.cls:4676-4677) and
// through \@IEEEauthorhalign inside it; the halign's \crcr is what separates the blocks, so
// a pass-through alone reproduces the run-together. Each block is its own paragraph here.
//
// \IEEEoverridecommandlockouts restores commands conference mode locked out
// (IEEEtran.cls:6274-6285). Nothing is locked out in this emulation, but the command must
// exist: a paper that calls it in its preamble stopped on an undefined control sequence.
func TestIEEEtranAuthorBlocks(t *testing.T) {
	const src = `\documentclass[conference]{IEEEtran}` +
		`\IEEEoverridecommandlockouts` +
		`\begin{document}\title{THETITLE}` +
		`\author{\IEEEauthorblockN{ALICEUN}\IEEEauthorblockA{AFFILUN}` +
		`\IEEEauthorblockN{BOBDEUX}\IEEEauthorblockA{AFFILDEUX}}` +
		`\maketitle\section{Intro}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	d := e.Diagnostics()
	for _, cs := range []string{"IEEEauthorblockN", "IEEEauthorblockA", "IEEEoverridecommandlockouts"} {
		if n := d.Skipped[cs]; n != 0 {
			t.Errorf("\\%s is still undefined (%d skipped)", cs, n)
		}
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"ALICEUN", "AFFILUN", "BOBDEUX", "AFFILDEUX"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 160))
		}
	}
	// ⛔ The point of the \par: the first block's name must not be glued to its affiliation
	// in one run. Rendered text has the two separated by more than a single space.
	if strings.Contains(text, "ALICEUN AFFILUN") {
		t.Error("the name and the affiliation ran into one paragraph — the \\par is missing")
	}
}
