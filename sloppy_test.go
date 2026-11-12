// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ \sloppy AND \fussy WERE BOTH EMPTY, and \sloppy is in 775 of the 2935
// corpus papers — 26.4%, comments stripped. A quarter of the corpus asked the
// line breaker to loosen and this engine did nothing.
//
// latex.ltx:13367-13378 is the whole of them:
//
//	\sloppy  \tolerance 9999  \emergencystretch 3em  \hfuzz .5\p@  \vfuzz\hfuzz
//	\fussy   \emergencystretch \z@  \tolerance 200   \hfuzz .1\p@  \vfuzz\hfuzz
//
// \tolerance is the one this engine acts on, so it is the one asserted. The
// others are accept-and-ignore parameters here — the same as they are when a
// document assigns them directly — and are set so the definitions stay honest
// for a document that reads them back.
func TestSloppyAndFussySetTheTolerance(t *testing.T) {
	probe := func(body string) int {
		t.Helper()
		var seen int
		probed := false
		e, err := buildEngine(Options{Lenient: true}, true)
		if err != nil {
			t.Fatalf("buildEngine: %v", err)
		}
		e.prim("gotex@probetol", func(e *Engine) {
			seen, probed = e.namedInt("tolerance"), true
		})
		src := `\documentclass{article}\begin{document}\makeatletter` + body + `\end{document}`
		if _, err := e.Run(src); err != nil {
			t.Fatalf("run %q: %v", body, err)
		}
		if !probed {
			t.Fatalf("the probe never ran for %q — this case measured NOTHING", body)
		}
		return seen
	}

	// The starting value, which is also \fussy's. Asserting it first means a
	// \fussy that did nothing could not be mistaken for one that worked.
	if got := probe(`\gotex@probetol`); got != 200 {
		t.Fatalf("\\tolerance starts at %d, want 200 — the control for everything below", got)
	}
	if got := probe(`\sloppy\gotex@probetol`); got != 9999 {
		t.Errorf("after \\sloppy, \\tolerance = %d, want 9999 (latex.ltx:13368)", got)
	}
	if got := probe(`\sloppy\fussy\gotex@probetol`); got != 200 {
		t.Errorf("after \\sloppy\\fussy, \\tolerance = %d, want 200 (latex.ltx:13376) — "+
			"\\fussy must undo \\sloppy, and starting at 200 is not evidence that it did", got)
	}
	// ⚠ Group-scoped, like every register assignment: {\sloppy …} must not leak.
	// A document that loosens one paragraph has not loosened the rest.
	if got := probe(`{\sloppy}\gotex@probetol`); got != 200 {
		t.Errorf("\\tolerance = %d after a group that set \\sloppy, want 200 — the "+
			"assignment escaped its group", got)
	}
}

// ⚠ AND \sloppy DOES NOT ALWAYS CHANGE THE OUTPUT, which is worth pinning so
// that a future reader does not take a no-change as a broken command.
//
// Measured against tectonic 0.17.0: the same paragraph at \hsize=100pt comes
// out IDENTICAL under \fussy and \sloppy, because it already fits within
// \tolerance=200. The two part only when the paragraph genuinely fails at 200 —
// at \hsize=45pt the reference goes from 5 lines to 8.
func TestSloppyIsConditionalOnTheParagraphBeingHard(t *testing.T) {
	set := func(cmd string) string {
		src := `\documentclass{article}\begin{document}\hsize=100pt{` + cmd +
			`\noindent Typography affects every line of every paper, and a measure ` +
			`that is wrong is wrong on every line it sets.\par}\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		return pageChars(e)
	}
	fussy, sloppy := set(`\fussy`), set(`\sloppy`)
	if fussy == "" {
		t.Fatal("the \\fussy paragraph produced no page text: this measures nothing")
	}
	if fussy != sloppy {
		t.Errorf("at a measure the paragraph already fits, \\sloppy changed the setting:\n"+
			"  fussy  %q\n  sloppy %q\n"+
			"the reference gives the same text for both here", firstN(fussy, 120), firstN(sloppy, 120))
	}
}
