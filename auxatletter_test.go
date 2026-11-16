// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔⛔ A @-BEARING CONTROL SEQUENCE WRITTEN TO THE .aux MUST NOT COME BACK AS TEXT.
//
// The .aux must be read with @ of category LETTER, whatever the preamble left
// behind — \document forces it now (kernelhelpers.go). Without that, the replay
// tokenised \NAT@force@numbers as \NAT — undefined, skipped — followed by the
// CHARACTERS @force@numbers, which reached the page as ordinary text.
//
// ⛔ ORDERING THE SWITCH IS NOT ENOUGH, and that intermediate fix passed this
// test while leaving the corpus paper broken: the kernel leaves @ at 12 for the
// whole preamble (hooks.go), and a document that writes \makeatother — 2201.04865
// does, main.tex:90/:93 — has @ already at 12 when \document runs, so moving the
// switch after the read changes nothing for it. What settled the rule was the
// ORACLE, not more reading: `tectonic` leaks in NEITHER case, so the category
// has to be forced rather than merely sequenced.
//
// natbib writes exactly that from \AtEndDocument (natbib.sty:966-973):
//
//	\immediate\write\@auxout{%
//	 \string\providecommand\string\NAT@force@numbers{}%
//	 \string\NAT@force@numbers}
//
// ⭐ The defect is the KERNEL's, not natbib's: every package that round-trips a
// @-bearing name through the .aux hits it, and natbib is merely the first one
// embedded here that writes such a name. It could not be seen before #712 made
// \@auxout a real stream, and it could not be seen before natbib.sty was
// embedded beside this test.
//
// ⛔⛔ THE WITNESS IS THE WHOLE DIFFICULTY, AND THREE OF MINE COULD NOT FIRE —
// I read their silence as a refutation and published that, wrongly:
//
//  1. The document must NOT contain \makeatletter. A preamble's \makeatletter
//     sets \gotexatasked (#500); the kernel then never switches @ at all, so the
//     leak cannot happen and the test passes either way. Two probes died of this.
//  2. "I checked it on a real corpus paper" does not rescue it: 2201.04865,
//     where the leak was first measured by gotex-worddiff, itself contains
//     \makeatletter. Reordering the switch cannot change anything there, and the
//     zero word-difference I measured was the expected result of exercising a
//     path that cannot fire — not evidence about it.
//  3. The writer must be a real package, where @ is a letter by the loader's own
//     doing, AND it must actually be loaded: a four-line .sty dropped in a temp
//     directory never ran under the two-pass compile() path, so its write never
//     happened and its silence meant nothing either.
//
// What fires is natbib itself, with its write ARMED: \NAT@stdbsttrue is set by a
// \bibitem carrying NO optional label (natbib.sty:860-863, the "standard bst"
// branch), which is why the bibliography below is written that way on purpose.
// Measured, same witness, one line of kernel apart:
//
//	without the fix  "@force@numbers@force@numbersSentineltext[1]here.…"
//	with it          "Sentineltext[1]here.…"
func TestAnAtNameWrittenToTheAuxDoesNotComeBackAsText(t *testing.T) {
	// ⛔ No \makeatletter anywhere, and \bibitem with no optional label: both are
	// load-bearing, see the comment above.
	const src = `\documentclass{article}
\usepackage[numbers]{natbib}
\begin{document}
Sentinel text \citep{k} here.
\begin{thebibliography}{1}
\bibitem{k} A. Author. A title. 2021.
\end{thebibliography}
\end{document}`

	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	page := pageText(e)
	t.Logf("page text: %q", page)

	// The control FIRST: without the body on the page, the assertion below would
	// pass on an empty document.
	if !strings.Contains(page, "Sentineltext") {
		t.Fatalf("the body is not on the page, so this is not measuring the .aux "+
			"replay at all: %q", page)
	}
	// And the second control: the bibliography entry must be there too, because
	// that is what arms natbib's write. A document whose \bibitem never ran
	// writes nothing to the .aux and cannot leak.
	if !strings.Contains(page, "A.Author") {
		t.Fatalf("the bibliography entry is missing, so \\NAT@stdbst was never set and "+
			"natbib wrote nothing to the .aux: %q", page)
	}
	if strings.Contains(page, "force@numbers") {
		t.Errorf("natbib's \\NAT@force@numbers came back from the .aux as TEXT: the file "+
			"is being read with @ of category other, so the name tokenises as \\NAT plus "+
			"the characters @force@numbers:\n%q", page)
	}
}
