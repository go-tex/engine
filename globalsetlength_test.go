// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \global in front of a LaTeX length command.
//
// In TeX a prefix applies to the assignment it finds after EXPANDING what follows:
// tex.web §1211 takes "the next non-blank non-relax NON-CALL token", so a macro is
// expanded away. \setlength is a macro there — ltlength.dtx: \def\setlength#1#2{#1
// #2\relax} — and \global therefore reaches the register assignment by itself.
//
// Here \setlength and its family are primitives, which getXToken does not expand,
// so the prefix has to be handed to them. Without that, \global\setlength was a
// LOCAL assignment: it took effect inside the group and was undone on the way out,
// silently, which is the worst way for it to fail.

func gslRun(t *testing.T, src string) string {
	t.Helper()
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatalf("buildEngine: %v", err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return out
}

func TestGlobalLengthCommandsOutliveTheirGroup(t *testing.T) {
	const decl = `\documentclass{article}\newlength\xx\setlength\xx{1pt}`
	for _, c := range []struct{ name, body, want string }{
		{"setlength", `{\global\setlength\xx{5pt}}`, "5.0pt"},
		{"addtolength", `{\global\addtolength\xx{4pt}}`, "5.0pt"},
		{"settowidth", `{\global\settowidth\xx{XXXX}}`, ""}, // just must not be 1.0pt
	} {
		out := gslRun(t, decl+c.body+`\message{[\the\xx]}`)
		if c.want != "" && !strings.Contains(out, "["+c.want+"]") {
			t.Errorf("%s: got %q, want [%s]", c.name, out, c.want)
		}
		if c.want == "" && strings.Contains(out, "[1.0pt]") {
			t.Errorf("%s: the global assignment was undone by the group (%q)", c.name, out)
		}
	}
}

func TestLocalLengthCommandsStillDieWithTheirGroup(t *testing.T) {
	// The control: without \global nothing outlives the group.
	out := gslRun(t, `\documentclass{article}\newlength\xx\setlength\xx{1pt}`+
		`{\setlength\xx{5pt}\message{[in \the\xx]}}\message{[out \the\xx]}`)
	if !strings.Contains(out, "[in 5.0pt]") {
		t.Errorf("got %q, want the local value inside the group", out)
	}
	if !strings.Contains(out, "[out 1.0pt]") {
		t.Errorf("got %q — a LOCAL \\setlength escaped its group", out)
	}
}

func TestGlobalOnALengthParameterOutlivesTheGroup(t *testing.T) {
	// \textwidth is \let to \hsize, so this goes through the engine's dimension
	// parameters rather than a register — a different path, same rule.
	out := gslRun(t, `\documentclass{article}{\global\setlength\textwidth{123pt}}\message{[\the\textwidth]}`)
	if !strings.Contains(out, "[123.0pt]") {
		t.Errorf("got %q, want [123.0pt]", out)
	}
}
