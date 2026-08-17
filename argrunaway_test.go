// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"testing"
	"time"
)

// A delimited-argument scan that never finds its delimiter and re-pushes a growing
// argument each round is a runaway the expansion guard alone does not catch: the
// churn happens while grabbing an argument, not while expanding a macro, and the
// mouth is already at end of file so no base input is ever consumed. This is the
// shape of revtex/aastex's \rvtx@enddocument@patch (parameter text "#1#2\@checkend#3"),
// which the 2020 format's enddocument hook installs and which hunts for a
// \@checkend{document} the engine's simplified \enddocument never emits — every
// aastex paper hung on it. The argument grab is charged to the same no-progress
// guard as expansion, so the loop is aborted (partial output) instead of hanging.
func TestRunawayDelimitedArgAtEOF(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true // tolerant: the runaway ends the loop rather than erroring
	// \P grabs #2 up to \ck; \Pmore re-invokes \P with the whole prior argument
	// wrapped in braces (so the \ck inside is hidden from the next scan by brace
	// depth), leaving no \ck at top level — the scan runs to EOF, forever.
	src := `\long\def\P#1#2\ck#3{\Pmore{#1#2}{#3}}` +
		`\long\def\Pmore#1#2{\P{#1\ck{#2}}}` +
		`\P{}start\ck{x}`
	done := make(chan struct{})
	go func() {
		e.Run(src)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("compile did not terminate: the runaway delimited-argument grab was not bounded")
	}
	if !e.runaway {
		t.Error("runaway flag not set — the argument grab was not caught by the no-progress guard")
	}
}
