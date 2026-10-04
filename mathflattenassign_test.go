// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// ⛔ Flattening math source resolves NOTATION, and an assignment is not notation.
//
// A macro whose replacement text begins by REDEFINING ITSELF must stay a token while
// flattenMathBody expands: expanding it hands back the assignment's own tokens, and the
// engine then expands the very name the assignment was about to replace.
//
// ⛔ The test is SELF-reference, not "the body assigns". A first version refused every
// assignment-bodied macro, and the equation census caught what the page count could not:
// \@forloop became a new trigger costing 23 equations, a NET +20 dropped equations over
// the 154-paper list, while pages showed 4 up and none down.
//
// The self-redefining idiom makes that non-terminating. quantumarticle.cls:1108-1109
// is the textbook form — nothing the first time, a comma after that:
//
//	\def\@@@comma{\def\@@@comma{,}}
//	\def\@@commaspacebefore#1{\@@@comma{}#1}
//
// Reached through \ensuremath in an author block, 2607.22466 spun \@@@comma 400 times
// and pushed {,} to 200 001 frames deep: ONE page out of 43KB. With the macro left for
// the stomach it comes out at 8 pages with no group open and no runaway.
//
// The guard is in the expansion path and not in flattenMathBody's noexp pre-pass,
// because the macro is NOT in the source being flattened: it arrives mid-expansion,
// through etoolbox's \forlistloop. Marking names up front cannot reach it — measured,
// the pre-pass version changed nothing at all.

const selfRedefining = `\documentclass{article}\makeatletter` +
	`\def\zzcomma{\def\zzcomma{,}}` +
	`\def\zzbefore#1{\zzcomma{}#1}` +
	`\begin{document}`

func TestMathFlatteningLeavesASelfRedefiningMacroForTheStomach(t *testing.T) {
	e, err := compile([]byte(selfRedefining+`A\ensuremath{\zzbefore{x}}B\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	if e.Diagnostics().Runaway {
		t.Error("the runaway guard tripped: the assignment was expanded instead of performed")
	}
	if len(e.RenderPages(e.renderMargin(0))) == 0 {
		t.Fatal("no page produced")
	}
}

// And the macro must still WORK: leaving it unexpanded is only right if the stomach
// then performs the assignment. The idiom's whole point is that the SECOND call yields
// a comma where the first yielded nothing, so that is the witness — a guard that
// dropped the token would pass the test above and silently lose every separator.
func TestTheSelfRedefiningMacroStillTakesEffect(t *testing.T) {
	e, err := compile([]byte(selfRedefining+
		`\zzcomma\zzcomma\message{[\meaning\zzcomma]}x\end{document}`),
		Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// After one execution the macro has redefined itself to the separator.
	if got := e.Diagnostics().Messages; !strings.Contains(got, "macro:->,") {
		t.Errorf("\\zzcomma did not redefine itself: %q", got)
	}
}
