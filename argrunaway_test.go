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

// The same guard must hold on the REAL path that motivated it: a class that
// installs a delimited patch in the 2020 enddocument hook. This mirrors
// revtex/aastex's \rvtx@enddocument@patch, which hunts for a \@checkend{document}
// the engine's simplified \enddocument never emits — without the guard every
// aastex/revtex paper hangs at \end{document}. Driving it through \AddToHook +
// the \enddocument sequence (not just the grab primitive) means a regression in
// the hook machinery that reopened the loop is caught here too.
func TestRunawayEnddocumentHookPatch(t *testing.T) {
	withTempDir(t, map[string]string{
		"rvtxmini.cls": `\NeedsTeXFormat{LaTeX2e}\ProvidesClass{rvtxmini}\LoadClass{article}` +
			`\long\def\patch#1#2\ckend#3{\patchmore{#1#2}{#3}}` +
			`\long\def\patchmore#1#2{\patch{#1\ckend{#2}}}` +
			`\AddToHook{enddocument}{\patch{}first\ckend{x}}`,
	}, func() {
		src := `\documentclass{rvtxmini}\begin{document}BODYWORD one two three.\end{document}`
		done := make(chan *Engine, 1)
		go func() {
			e, _ := compile([]byte(src), Options{Lenient: true})
			done <- e
		}()
		select {
		case e := <-done:
			if !e.runaway {
				t.Error("runaway flag not set — the enddocument-hook patch loop was not bounded")
			}
		case <-time.After(25 * time.Second):
			t.Fatal("compile did not terminate: the enddocument-hook runaway was not caught")
		}
	})
}
