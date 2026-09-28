// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ⛔ An xparse document command was invisible to the maths layer. xparse binds one to a
// STOMACH primitive — a real \NewDocumentCommand is \protected and must not expand
// inside an \edef — and expandMacroInMathSource only ever looked at mMacro, so every
// such macro reached go-tex/math as a bare NAME and dropped its formula.
//
// Measured on 999 papers of the arXiv corpus: 99 census triggers worth 1193 equations,
// the largest single group in the dropped-equation census (#496).
//
// These tests use MathDropped rather than a rendered shape, because the claim is
// exactly "the formula was not dropped", and they check the two accepted specification
// forms, the refusal of the rest, and — the part a shape cannot show — that the BODY
// was substituted and the ARGUMENTS were consumed.
func mathDropRun(t *testing.T, src string) map[string]int {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(src); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	return e.Diagnostics().MathDropped
}

// Specification {} — no arguments at all. 37 of the 99 census triggers and 293
// equations, the largest of the five groups, and the one my first attempt silently
// failed to serve: keying on "xpSpecs != nil" cannot see an EMPTY specification,
// because it gives a nil slice.
func TestAnXparseCommandWithNoArgumentsReachesTheMathsLayer(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand \dd {} {\mathbf{d}}$\dd + 1$`)
	if n := d[`\dd`]; n != 0 {
		t.Errorf(`\dd dropped %d equation(s): the declaration did not reach the maths layer (%v)`, n, d)
	}
}

// Specification m … m — mandatory only, which is exactly \newcommand's parameter text.
// 18 triggers, 200 equations.
func TestAnXparseCommandWithMandatoryArgumentsReachesTheMathsLayer(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand{\Thr}{ m m m }{#1 \cdot #2 + #3}$\Thr{a}{b}{c}$`)
	if n := d[`\Thr`]; n != 0 {
		t.Errorf(`\Thr dropped %d equation(s) (%v)`, n, d)
	}
	// Spaces in the specification are not significant: the kernel's normaliser is
	// driven by \tl_to_str, so "{ m m m }" and "{mmm}" are the same.
	d = mathDropRun(t, `\NewDocumentCommand{\T}{mmm}{#1#2#3}$\T{a}{b}{c}$`)
	if n := d[`\T`]; n != 0 {
		t.Errorf(`{mmm} dropped %d equation(s) (%v)`, n, d)
	}
}

// ⛔ The BODY must actually be substituted, not merely the name removed. A body whose
// content is itself unknown must drop under THAT name — which is the only observation
// that distinguishes "expanded" from "silently swallowed".
func TestTheBodyIsSubstitutedAndNotSwallowed(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand \zz {} {\nosuchmaththing}$\zz$`)
	if d[`\zz`] != 0 {
		t.Errorf(`dropped under \zz: the declaration was not used at all (%v)`, d)
	}
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`the body was not substituted — nothing dropped under its own unknown command (%v)`, d)
	}
}

// ⛔ And every mandatory argument must be CONSUMED. With the arity right, the second
// argument is grabbed and discarded by the body {#1}; with the arity read as one, the
// leftover {\nosuchmaththing} stays in the source and drops the formula. That asymmetry
// is what makes this test discriminate — asserting "it renders" would pass either way.
func TestEveryMandatoryArgumentIsConsumed(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand \ww {m m} {#1}$\ww{\alpha}{\nosuchmaththing}$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`the second argument was left in the source: %d drop(s) under it (%v)`, n, d)
	}
	if n := d[`\ww`]; n != 0 {
		t.Errorf(`\ww itself dropped %d equation(s) (%v)`, n, d)
	}
}

// ⛔ A specification this cannot serve EXACTLY must leave the command standing, so the
// census reports it. Substituting with the wrong arity would mangle the formula
// silently, which is worse than dropping it.
//
// o / O{default} is the tempting one: the kernel normalises o to D[]{-NoValue-}
// (latex.ltx:2121), so an ABSENT optional argument yields the -NoValue- marker and the
// body tests it with \IfNoValueTF. \newcommand substitutes a DEFAULT instead, and a
// body written \IfNoValueF{#1}{…} would then take the wrong branch. 24 triggers and 324
// equations wait on doing that properly.
func TestASpecificationItCannotServeIsRefusedLoudly(t *testing.T) {
	// ⛔ The inputs matter more than the assertion. My first three were $\Opt$,
	// $\Str{x}$ and $\Dfl{y}$ — all THREE still dropped with the guard removed,
	// because reading "o" as one mandatory argument then finds nothing to grab and the
	// occurrence is left verbatim anyway. The ablation broke 0 tests: a blind witness.
	//
	// Each input below SUPPLIES what a wrong reading would happily consume, so that
	// accepting the specification renders something WRONG instead of failing. With the
	// guard removed, $\Opt[x]$ takes "[" as #1 and sets "Γx]"; $\Str*{x}$ takes "*"
	// and "x" as two mandatory arguments and sets "x"; $\Dfl{a}{b}$ sets "ab". All
	// three then stop being reported, which is exactly the silent mangling the guard
	// exists to prevent.
	for _, c := range []struct{ name, src string }{
		{`\Opt`, `\NewDocumentCommand \Opt {o} {\Gamma}$\Opt[x]$`},
		{`\Str`, `\NewDocumentCommand \Str {s m} {#2}$\Str*{x}$`},
		{`\Dfl`, `\NewDocumentCommand \Dfl {O{z} m} {#1#2}$\Dfl{a}{b}$`},
	} {
		d := mathDropRun(t, c.src)
		if d[c.name] == 0 {
			t.Errorf(`%s was NOT reported: an unservable specification must leave the command standing (%v)`,
				c.name, d)
		}
	}
}

// The three siblings declare the same way, and \ProvideDocumentCommand must not clobber
// an existing definition — which is the one behavioural difference between them.
func TestTheSiblingsDeclareTheSameWay(t *testing.T) {
	for _, decl := range []string{
		`\NewDocumentCommand`, `\DeclareDocumentCommand`, `\ProvideDocumentCommand`,
	} {
		d := mathDropRun(t, decl+` \sib {} {\mathbf{s}}$\sib$`)
		if n := d[`\sib`]; n != 0 {
			t.Errorf(`%s: dropped %d equation(s) (%v)`, decl, n, d)
		}
	}
	// \RenewDocumentCommand over an existing one.
	d := mathDropRun(t, `\NewDocumentCommand \rn {} {\nosuchmaththing}`+
		`\RenewDocumentCommand \rn {} {\mathbf{r}}$\rn$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`\RenewDocumentCommand did not replace the body: %d drop(s) under the old one (%v)`, n, d)
	}
}

// An xparse ENVIRONMENT stays stomach-only. \begin and \end are already marked noexp
// for the maths layer, and flattening one would hand it a half-run environment.
func TestAnXparseEnvironmentIsNotFlattenedIntoAFormula(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.lenient = true
	if _, err := e.Run(`\NewDocumentEnvironment{envx}{}{A}{B}\begin{envx}x\end{envx}`); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// Nothing to assert about maths here: the point is that the environment path is
	// untouched and still runs. A panic or an error would be the failure.
}
