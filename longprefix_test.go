// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \long is part of a macro's MEANING, and it governs whether \par may appear in an
// argument.
//
// tex.web keeps it in the macro's own command code (§4209-4212: call / long_call),
// accumulates the prefix (§1211) and stores it (§1218: define(p, call+(a mod 4), …)).
// At every call §391 reads it back — long_state := eq_type(cur_cs) — and §392 refuses
// a \par in an argument when it is absent, aborting the call (§396: "Paragraph ended
// before \x was complete", back_error).
//
// The prefix was a literal no-op here: e.prim("long", func(e *Engine) {}). Two things
// followed. \meaning did not name it, so \ifx could not tell \long\def\a{} from
// \def\a{}. And, worse, the \par check is TeX's own RUNAWAY DETECTOR: without it an
// argument that has lost its closing brace is scanned on until some generic limit
// stops it, swallowing everything in between, where TeX stops at the end of the
// paragraph — where the mistake actually is.

func lgRun(t *testing.T, src string) string {
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

func TestLongIsPartOfTheMeaning(t *testing.T) {
	// Checked against real LaTeX, which prints \protected BEFORE \long whatever order
	// the prefixes were written in.
	for _, c := range []struct{ name, src, want string }{
		{"plain", `\def\a#1{[#1]}\message{[\meaning\a]}`, "[macro:#1->[#1]]"},
		{"long", `\long\def\a#1{[#1]}\message{[\meaning\a]}`, `[\long macro:#1->[#1]]`},
		{"protected then long", `\protected\long\def\a#1{[#1]}\message{[\meaning\a]}`, `[\protected\long macro:#1->[#1]]`},
		{"long then protected", `\long\protected\def\a#1{[#1]}\message{[\meaning\a]}`, `[\protected\long macro:#1->[#1]]`},
	} {
		if got := lgRun(t, c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to contain %q", c.name, got, c.want)
		}
	}
}

func TestIfxSeparatesLongFromPlain(t *testing.T) {
	out := lgRun(t, `\long\def\a{A}\def\b{A}\message{[\ifx\a\b MEME\else DIFF\fi]}`)
	if !strings.Contains(out, "[DIFF]") {
		t.Errorf("got %q — \\ifx must separate \\long\\def\\a{A} from \\def\\a{A}", out)
	}
	out = lgRun(t, `\long\def\a{A}\long\def\b{A}\message{[\ifx\a\b MEME\else DIFF\fi]}`)
	if !strings.Contains(out, "[MEME]") {
		t.Errorf("got %q — two \\long macros with the same body are the same meaning", out)
	}
}

func TestLongMacroAcceptsParInItsArgument(t *testing.T) {
	out := lgRun(t, `\long\def\L#1{<#1>}\message{[\L{a\par b}]}`)
	if !strings.Contains(out, "<a") || !strings.Contains(out, "b>") {
		t.Errorf("got %q — a \\long macro must accept \\par", out)
	}
}

func TestPlainMacroRefusesParInItsArgument(t *testing.T) {
	// tex.web §392/§396: the call is abandoned and the \par put back. The engine
	// records the recovery rather than stopping, but it must NOT swallow the \par.
	out := lgRun(t, `\def\C#1{<#1>}\message{[\C{a\par b}]}`)
	if strings.Contains(out, "<a") {
		t.Errorf("got %q — the call should have been abandoned, not completed", out)
	}
}

func TestParCheckSeesInsideBraces(t *testing.T) {
	// §392 tests every token of the parameter, and it tests it BEFORE deciding
	// whether the token opens a group — so a \par buried in braces is caught too.
	out := lgRun(t, `\def\C#1{<#1>}\message{[\C{{a\par b}}]}`)
	if strings.Contains(out, "<{a") {
		t.Errorf("got %q — a \\par inside braces must be caught as well", out)
	}
}

func TestTheLongPrefixDoesNotLeak(t *testing.T) {
	out := lgRun(t, `\long\def\a{A}\def\b{B}\message{[\meaning\b]}`)
	if strings.Contains(out, `\long`) {
		t.Errorf("got %q — the \\long prefix leaked onto the next definition", out)
	}
}
