// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A document's own math shorthand — \newcommand\F{\mathbb F}, \def\Z{\mathbb{Z}} —
// is emitted verbatim by the raw-token math scanner, so go-tex/math sees \F/\Z, which
// it never learned, and used to drop the whole equation. The render-fallback resolves
// such a parameterless user macro to its replacement (which uses commands go-tex/math
// DOES know, here \mathbb) and retries, so the equation renders and nothing is dropped.
func TestUserMathMacroResolves(t *testing.T) {
	cases := []struct {
		def, use string
	}{
		{`\newcommand{\F}{\mathbb F}`, `$\F_p$`},
		{`\def\Z{\mathbb{Z}}`, `$x\in\Z$`},
		{`\newcommand{\R}{\mathbb{R}}`, `$\R^n$`},
	}
	for _, c := range cases {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.lenient = true
		if _, err := e.Run(c.def + c.use); err != nil {
			t.Errorf("%s%s: unexpected error %v", c.def, c.use, err)
			continue
		}
		if len(e.SkippedCommands()) != 0 {
			t.Errorf("%s%s: expected to render, but dropped: %v", c.def, c.use, e.SkippedCommands())
		}
	}
}

// Resolution is recursive: a shorthand whose replacement is itself another shorthand
// (\X → \R → \mathbb R) resolves through both levels, one retry per still-unknown name.
func TestUserMathMacroResolvesRecursively(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\newcommand{\R}{\mathbb R}\newcommand{\X}{\R}$\X_2$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(e.SkippedCommands()) != 0 {
		t.Errorf("expected recursive resolve, dropped: %v", e.SkippedCommands())
	}
}

// The fallback must never touch a command go-tex/math already understands: the literal
// source is tried first, so a kernel math macro like \cdot (which the engine defines as
// a macro but go-tex/math renders natively) is never expanded and keeps rendering.
func TestKnownMathCommandNotExpanded(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`$a \cdot b$ and $\frac{1}{2}$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if len(e.SkippedCommands()) != 0 {
		t.Errorf("known commands must not be dropped/expanded, got: %v", e.SkippedCommands())
	}
}

// A user macro whose replacement is STILL unknown to go-tex/math cannot be rescued: the
// equation is dropped and recorded under the unknown command the expansion exposed, and
// the per-name guard means a single expansion attempt, not a loop.
func TestUserMathMacroExpandsToStillUnknown(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\newcommand{\bad}{\nosuchmathprimitive}$\bad$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if e.SkippedCommands()["\\nosuchmathprimitive"] == 0 {
		t.Errorf("expected drop recorded under the exposed unknown, got: %v", e.SkippedCommands())
	}
}

// A self-referential macro (\def\loopy{\loopy}) must not loop the resolver: the per-name
// seen guard stops after one expansion of \loopy, and the equation is dropped, not hung.
func TestSelfReferentialMathMacroDoesNotLoop(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\def\loopy{\loopy}$\loopy$`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if e.SkippedCommands()["\\loopy"] == 0 {
		t.Errorf("expected \\loopy dropped after the guard stopped, got: %v", e.SkippedCommands())
	}
}

// zeroParamMacroExpansion accepts only a parameterless macro; a primitive, an undefined
// name, and a macro that takes arguments each return (,"" false) so the resolver leaves
// them verbatim for go-tex/math.
func TestZeroParamMacroExpansion(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\newcommand{\Zero}{\mathbb Z}\newcommand{\One}[1]{|#1|}`); err != nil {
		t.Fatal(err)
	}
	if exp, ok := e.zeroParamMacroExpansion("Zero"); !ok || exp != `\mathbb Z` {
		t.Errorf("0-param macro: got (%q,%v), want (%q,true)", exp, ok, `\mathbb Z`)
	}
	if _, ok := e.zeroParamMacroExpansion("One"); ok {
		t.Error("a macro with a parameter must not be resolved by string substitution")
	}
	if _, ok := e.zeroParamMacroExpansion("relax"); ok {
		t.Error("a primitive is not a resolvable macro")
	}
	if _, ok := e.zeroParamMacroExpansion("thereisnosuchname"); ok {
		t.Error("an undefined name is not a resolvable macro")
	}
}

// unknownMathCommand pulls the command name out of go-tex/math's error, spanning letters
// and @, and returns "" for any other message.
func TestUnknownMathCommand(t *testing.T) {
	cases := []struct{ in, want string }{
		{`texmath: unknown command \mathbb`, "mathbb"},
		{`texmath: unknown command \@currenvir followed`, "@currenvir"},
		{`unknown command \F`, "F"},
		{`unknown command \ with no name`, ""},
		{`some other failure`, ""},
	}
	for _, c := range cases {
		if got := unknownMathCommand(c.in); got != c.want {
			t.Errorf("unknownMathCommand(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// replaceMathCS substitutes \name only as a whole control sequence (matched with the
// trailing space the math scanner always emits), so \F is replaced but \Foo is not.
func TestReplaceMathCS(t *testing.T) {
	got := replaceMathCS(`\F _p + \Foo + \F `, "F", `\mathbb F`)
	want := `\mathbb F _p + \Foo + \mathbb F `
	if got != want {
		t.Errorf("replaceMathCS: got %q, want %q", got, want)
	}
}
