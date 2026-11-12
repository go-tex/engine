// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"testing"
)

// bodyPrevDepth compiles a document and reports \prevdepth as it stands once
// the class has been loaded and the body has begun, before any text.
func bodyPrevDepth(t *testing.T, class string) int {
	t.Helper()
	// ⛔ \makeatletter AFTER \begin{document}, not before it. In a document body
	// @ is catcode 12, so \gotex@probepd parses as \gotex followed by the
	// characters "@probepd" and the primitive never runs — and \begin{document}
	// itself runs \makeatother, so putting \makeatletter in the preamble is
	// undone before the probe is read. Two versions of this test measured
	// nothing; the "never ran" guard below is what said so both times instead of
	// letting a \prevdepth of 0 pass for a probe that had not executed.
	src := `\documentclass{` + class + `}\begin{document}\makeatletter\gotex@probepd\end{document}`
	var seen int
	probed := false
	// ⛔ buildEngine, not New. New gives a bare TeX engine with no LaTeX loaded,
	// so \documentclass is undefined and the whole document falls apart before
	// the probe is reached. The first three versions of this test used New and
	// measured nothing — three times, each time caught by the guard below rather
	// than by the assertion, which would have read a zero as a pass.
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("%s: buildEngine: %v", class, err)
	}
	e.prim("gotex@probepd", func(e *Engine) { seen, probed = e.prevDepth, true })
	if _, err := e.Run(src); err != nil {
		t.Fatalf("%s: run: %v", class, err)
	}
	if !probed {
		t.Fatalf("%s: the probe never ran — this test measured NOTHING", class)
	}
	return seen
}

// ⛔ TWO-SIDED, AND THAT IS THE POINT OF THE TEST.
//
// #699 offered, as a fallback, setting \prevdepth to 0 in the class lead. That
// would pass the first half of this and fail the second: the reference leaves
// \documentclass{minimal} at ignore_depth, because minimal.cls does not call
// \onecolumn and therefore never reaches \clearpage's empty \hbox. Measured
// with tectonic 0.17.0:
//
//	\RequirePackage{calc}       -> -1000.0pt   the file-loading machinery is innocent
//	\documentclass{minimal}     -> -1000.0pt   it is not every class
//	\documentclass{article}     ->     0.0pt
//
// A change that makes BOTH read 0 has not ported anything; it has hard-coded
// the answer to one of them.
func TestPrevDepthAtBodyStartMatchesTheReferenceForBothClasses(t *testing.T) {
	if got := bodyPrevDepth(t, "article"); got != 0 {
		t.Errorf("article: \\prevdepth = %d sp, want 0 — article.cls:640 runs \\onecolumn, "+
			"which runs \\clearpage, which contributes an empty \\hbox (latex.ltx:15065) "+
			"and that box sets \\prevdepth to its depth", got)
	}
	if got := bodyPrevDepth(t, "minimal"); got != ignoreDepth {
		t.Errorf("minimal: \\prevdepth = %d sp, want ignoreDepth (%d) — minimal.cls never "+
			"reaches \\clearpage, so the reference leaves it alone and so must we. A fix "+
			"that moves THIS one has hard-coded the answer rather than ported the mechanism",
			got, ignoreDepth)
	}
}

// ⛔ AND IT MUST NOT FIRE IN THE MIDDLE OF A DOCUMENT.
//
// LaTeX guards the \hbox with \ifdim\pagetotal<\topskip. \pagetotal is an
// accept-and-ignore register here and is 0 for ever, so porting that guard as
// written would read 0pt < 10pt — always true — and contribute a box at every
// \clearpage in every document. What is used instead is "the main vertical list
// is still empty", which cannot be true once anything has been typeset.
func TestTheStrutDoesNotFireOnceSomethingHasBeenTypeset(t *testing.T) {
	src := `\documentclass{article}\begin{document}\makeatletter Some text.\par\clearpage\gotex@probelen\end{document}`
	var before, after int
	probed := false
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	e.prim("gotex@probelen", func(e *Engine) { after, probed = len(e.mvl), true })
	if _, err := e.Run(src); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !probed {
		t.Fatal("the probe never ran — this test measured NOTHING")
	}
	_ = before
	// The control: a document that reached \clearpage has a non-empty list, so
	// the strut's guard refused. A strut contributed here would be an extra box
	// on the list with nothing to justify it.
	if after == 0 {
		t.Fatal("the vertical list is empty after a paragraph: the probe is in the wrong place")
	}
	// Compile the same document with the \clearpage removed and compare the list
	// length: the difference must be the page break alone, never an extra box.
	plain := `\documentclass{article}\begin{document}\makeatletter Some text.\par\gotex@probelen\end{document}`
	e2, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("control buildEngine: %v", err)
	}
	var plainLen int
	plainProbed := false
	e2.prim("gotex@probelen", func(e *Engine) { plainLen, plainProbed = len(e.mvl), true })
	if _, err := e2.Run(plain); err != nil {
		t.Fatalf("control run: %v", err)
	}
	if !plainProbed {
		t.Fatal("the control's probe never ran — the comparison would be against nothing")
	}
	if after-plainLen > 1 {
		t.Errorf("\\clearpage added %d nodes where at most the forced break is expected: "+
			"the strut fired mid-document", after-plainLen)
	}
	// ⛔ AND NOT AT THE HEAD EITHER. The first version of this fix contributed a
	// real empty hbox, as the reference's source literally says. It cost 81
	// tests: an extra box at the head of the list takes the first page's
	// \topskip and every document's first line drops by a \baselineskip. The
	// reference discards it too — a one-word article is ONE page out of
	// tectonic, not two — so only \prevdepth survives, and only \prevdepth is
	// ported. This pins the list length against a document with no \clearpage at
	// all, which is where a reintroduced box would show up.
	bare := `\documentclass{article}\begin{document}\makeatletter X\par\gotex@probelen\end{document}`
	e3, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("bare buildEngine: %v", err)
	}
	var bareLen int
	bareProbed := false
	e3.prim("gotex@probelen", func(e *Engine) { bareLen, bareProbed = len(e.mvl), true })
	if _, err := e3.Run(bare); err != nil {
		t.Fatalf("bare run: %v", err)
	}
	if !bareProbed {
		t.Fatal("the bare probe never ran")
	}
	if bareLen == 0 {
		t.Fatal("the bare document contributed nothing: the probe is in the wrong place")
	}
}
