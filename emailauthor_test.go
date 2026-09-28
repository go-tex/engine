// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \email{a@b} was defined as \def\email#1{} — a stub that threw the address away. A stub is
// not an undefined command, so no census channel counted it.
//
// Measured against the reference PDFs before implementing: 57 of 68 \email values in the
// corpus appear in their own reference, and of the 32 papers that have both an \email and a
// reference, 20 print it on PAGE 1. The eleven that do not appear are template placeholders
// the author never replaced (firstname.lastname@phillips.org, iauthor@gmail.com).
//
// ⛔ And measured AFTER: only ONE corpus paper actually gains an address (2406.09085, llncs,
// 0 -> 2, both matching its reference). The other 24 already printed theirs, because their
// class defines its own \email and never reaches this stub. gobblers.py counts uses in the
// SOURCE; what reaches a stub is a second question, and for this macro the answer is 1 paper
// rather than the 44 the inventory lists.
//
// amsart needs no branch: it prints \email in its BOTTOM matter after the bibliography
// (amsart.cls:509, :525) and defines \email itself, overriding this.
func TestEmailJoinsTheAuthorBlock(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\title{THETITLE}\author{ALICEUN}\email{alice@example.org}` +
		`\maketitle\section{Intro}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"THETITLE", "ALICEUN", "alice@example.org"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 120))
		}
	}
	// ⛔ It joins the AUTHOR, not the body: it must appear before the section that follows
	// \maketitle, or it has merely leaked into the running text where it stands.
	iMail, iBody := strings.Index(text, "alice@example.org"), strings.Index(text, "BODY")
	if iMail < 0 || iBody < 0 || iMail > iBody {
		t.Errorf("the address is not in the title block: alice@%d BODY@%d in %q",
			iMail, iBody, firstN(text, 120))
	}
}

// An \email with no \author must not break: appending to a macro that was never given is
// what the \affil route guards against, and the same guard has to hold here.
func TestEmailWithoutAnAuthorDoesNotBreak(t *testing.T) {
	const src = `\documentclass{article}\begin{document}` +
		`\title{THETITLE}\email{solo@example.org}\maketitle\section{Intro}BODY\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	text := stripSVGTags(strings.Join(e.RenderPages(e.renderMargin(0)), ""))
	for _, want := range []string{"THETITLE", "solo@example.org", "BODY"} {
		if !strings.Contains(text, want) {
			t.Errorf("%q is not on the page; it reads %q", want, firstN(text, 120))
		}
	}
}
