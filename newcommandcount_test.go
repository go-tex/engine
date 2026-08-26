// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \newcommand's argument count is a NUMBER, not a digit. LaTeX scans it the way
// TeX scans any number (\@tempcnta#1\relax), so a count register, a \chardef'd
// constant or \the<register> all name it.
//
// Reading only the literal digits and ignoring anything else was quietly
// destructive: a command written \newcommand\foo[\somecount]{…} took ZERO
// arguments, so every argument the caller passed was left in the input and
// TYPESET. beamer builds its whole overlay layer that way —
// \newcommand\cs[\beamer@argscount]{…} — which is why \defbeamertemplate printed
// each template's body onto the page instead of storing it, and why an empty
// beamer document came out several pages long.

func ncRun(t *testing.T, src string) string {
	t.Helper()
	e, err := buildEngine(Options{}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

func TestNewcommandCountFromARegister(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{"count register", `\newcount\nn \nn=1 \newcommand\c[\nn]{<#1>}\message{[\c{X}]}`, "[<X>]"},
		{"two arguments", `\newcount\nn \nn=2 \newcommand\c[\nn]{<#1|#2>}\message{[\c{X}{Y}]}`, "[<X|Y>]"},
		{"through \\the", `\newcount\nn \nn=1 \newcommand\c[\the\nn]{<#1>}\message{[\c{X}]}`, "[<X>]"},
		{"a dimen is not one", `\newcommand\c[1]{<#1>}\message{[\c{X}]}`, "[<X>]"},
	} {
		if got := ncRun(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.name, got, c.want)
		}
	}
}

func TestNewcommandCountZeroWhenNotANumber(t *testing.T) {
	// Real LaTeX raises "Missing number, treated as zero" here and defines a command
	// of no arguments; what followed stays in the input. The engine matches that
	// rather than inventing a count from the stray digit.
	if got := ncRun(t, `\newcommand\c[\relax 1]{<#1>}\message{[\c A]}`); !strings.Contains(got, "[<>A]") {
		t.Errorf("got %q, want it to contain [<>A]", got)
	}
}

func TestNewcommandCountScanDoesNotEatTheDocument(t *testing.T) {
	// The number scan runs on an isolated input stack: whatever the bracket leaves
	// unread is dropped with it and cannot reach the document.
	out := ncRun(t, `\newcount\nn \nn=1 \newcommand\c[\nn]{<#1>}\message{[after]}`)
	if !strings.Contains(out, "[after]") {
		t.Errorf("got %q — the count scan swallowed what followed", out)
	}
}

func TestNewenvironmentCountFromARegister(t *testing.T) {
	// \newenvironment reads its count through the same reader.
	out := ncRun(t, `\newcount\nn \nn=1 \newenvironment{ev}[\nn]{<#1|}{|>}\message{[\begin{ev}{X}m\end{ev}]}`)
	if !strings.Contains(out, "[<X|m|>]") {
		t.Errorf("got %q, want it to contain [<X|m|>]", out)
	}
}
