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
	// o and O{def} were in this list until they were served; what remains is the star,
	// the token test and the delimited/embellished/verbatim kinds.
	// o, O{def}, s and t<c> were each in this list until they were served. What remains
	// is the delimited, embellished and verbatim kinds.
	for _, c := range []struct{ name, src string }{
		{`\Dlm`, `\NewDocumentCommand \Dlm {r() m} {#1#2}$\Dlm(a){b}$`},
		{`\Emb`, `\NewDocumentCommand \Emb {m E{_^}{{}{}}} {#1}$\Emb{a}_b^c$`},
		{`\Vrb`, `\NewDocumentCommand \Vrb {v} {#1}$\Vrb|x|$`},
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

// Optional arguments, each grabbed IN ITS PLACE. 24 census triggers and 324 equations,
// and the reason it could not go through \newcommand's machinery: an ABSENT o yields the
// -NoValue- marker (latex.ltx:2121 normalises o to D[]{-NoValue-}) and the body tests it
// with \IfNoValueTF, where \newcommand would substitute a DEFAULT and make
// \IfNoValueF{#1}{…} take the wrong branch.
func TestAnOptionalArgumentIsGrabbedAndItsAbsenceCarriesTheMarker(t *testing.T) {
	// Present: the body's \IfNoValueF branch must RUN, so the unknown inside it drops.
	d := mathDropRun(t, `\NewDocumentCommand \Dw {o} {\Gamma\IfNoValueF{#1}{\nosuchmaththing}}$\Dw[3]$`)
	if d[`\Dw`] != 0 {
		t.Errorf(`\Dw dropped: the optional argument was not grabbed (%v)`, d)
	}
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`\IfNoValueF took the ABSENT branch although [3] was given (%v)`, d)
	}
	// Absent: the same branch must NOT run, so nothing drops at all. This is the half
	// that a default value would get wrong.
	d = mathDropRun(t, `\NewDocumentCommand \Dw {o} {\Gamma\IfNoValueF{#1}{\nosuchmaththing}}$\Dw$`)
	if d[`\Dw`] != 0 {
		t.Errorf(`\Dw dropped when its optional argument was absent (%v)`, d)
	}
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`\IfNoValueF ran its branch %d time(s) on an ABSENT argument: the marker is not reaching it (%v)`, n, d)
	}
}

// O{default} substitutes the DEFAULT when absent, which is the other half of the pair —
// and the two must not be confused, since o and O differ only in that.
func TestADefaultedOptionalArgumentUsesItsDefault(t *testing.T) {
	// Absent: #1 is the default, which here is itself unknown, so it must drop under it.
	d := mathDropRun(t, `\NewDocumentCommand \Df {O{\nosuchmaththing}} {#1}$\Df$`)
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`the default was not substituted for the absent argument (%v)`, d)
	}
	// Present: the default must NOT be used.
	d = mathDropRun(t, `\NewDocumentCommand \Df {O{\nosuchmaththing}} {#1}$\Df[\alpha]$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`the default was used although [\alpha] was given: %d drop(s) (%v)`, n, d)
	}
}

// ⛔ The specification's ORDER is honoured. \newcommand can only make the FIRST
// parameter optional; "m o m" cannot be expressed by it at all, and a corpus paper
// writes "s O{\lambda} m m". Each case below puts an unknown command in a position that
// only survives if the arguments were matched in order.
func TestTheSpecificationOrderIsHonoured(t *testing.T) {
	// m o m, all supplied: the body keeps #2 only, so the unknowns in #1 and #3 vanish.
	d := mathDropRun(t, `\NewDocumentCommand \Mid {m o m} {#2}`+
		`$\Mid{\nosuchA}[\beta]{\nosuchB}$`)
	for _, bad := range []string{`\nosuchA`, `\nosuchB`} {
		if n := d[bad]; n != 0 {
			t.Errorf(`%s survived: the arguments were not matched in order (%v)`, bad, d)
		}
	}
	if d[`\Mid`] != 0 {
		t.Errorf(`\Mid dropped (%v)`, d)
	}
	// m o m with the optional ABSENT: the two mandatory ones must still be taken from
	// their own places, so {\gamma} lands in #3 and not in #2.
	d = mathDropRun(t, `\NewDocumentCommand \Mid {m o m} {#3}`+
		`$\Mid{\nosuchA}{\gamma}$`)
	if n := d[`\nosuchA`]; n != 0 {
		t.Errorf(`the first mandatory argument leaked into the body: %d drop(s) (%v)`, n, d)
	}
	if d[`\Mid`] != 0 {
		t.Errorf(`\Mid dropped with the optional absent (%v)`, d)
	}
}

// ⛔ \IfNoValueTF and friends had to become EXPANDABLE. The comment beside them in
// xparse.go claimed they already were and NOTHING registered them, so a body carrying
// one survived flattenMathBody and the maths layer was handed \IfNoValueF itself. This
// asserts the registration rather than its effect, because the effect is easy to get by
// accident and the registration is the claim.
func TestTheArgumentTestsAreExpandable(t *testing.T) {
	for _, n := range []string{
		"IfNoValueTF", "IfNoValueT", "IfNoValueF",
		"IfValueTF", "IfValueT", "IfValueF",
		"IfBooleanTF", "IfBooleanT", "IfBooleanF",
	} {
		if !isExpandable(n) {
			t.Errorf(`\%s is not expandable: a conditional that chooses a branch belongs in the gullet`, n)
		}
	}
}

// TeX has nine parameters. A specification asking for more is refused, and this is the
// one clause the OUTER guard alone enforces — parseXparseMathArgs would happily grab ten
// arguments, so removing the guard is invisible for every other refused kind but not for
// this one. That is why the test exists in this shape.
func TestASpecificationBeyondNineParametersIsRefused(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand \Ten {m m m m m m m m m m} {#1}`+
		`$\Ten{a}{b}{c}{d}{e}{f}{g}{h}{i}{j}$`)
	if d[`\Ten`] == 0 {
		t.Errorf(`a ten-argument specification was accepted: TeX has nine parameters (%v)`, d)
	}
	// Nine is fine.
	d = mathDropRun(t, `\NewDocumentCommand \Nine {m m m m m m m m m} {#9}`+
		`$\Nine{a}{b}{c}{d}{e}{f}{g}{h}{\alpha}$`)
	if n := d[`\Nine`]; n != 0 {
		t.Errorf(`nine arguments were refused: %d drop(s) (%v)`, n, d)
	}
}

// s is t* (latex.ltx:2131), so one clause serves both. 14 census triggers and 234
// equations. Both directions of both are checked, because the marker is what
// \IfBooleanTF tests and getting it backwards renders the wrong branch silently.
func TestAStarIsTestedAndConsumedOnlyOnAMatch(t *testing.T) {
	// Star present: the TRUE branch runs, so the unknown inside it drops.
	d := mathDropRun(t, `\NewDocumentCommand \Sb {s m} {\IfBooleanTF{#1}{\nosuchmaththing}{#2}}`+
		`$\Sb*{\alpha}$`)
	if d[`\Sb`] != 0 {
		t.Errorf(`\Sb dropped with a star (%v)`, d)
	}
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`\IfBooleanTF took the FALSE branch although * was given (%v)`, d)
	}
	// Star absent: the FALSE branch runs, so nothing drops.
	d = mathDropRun(t, `\NewDocumentCommand \Sb {s m} {\IfBooleanTF{#1}{\nosuchmaththing}{#2}}`+
		`$\Sb{\alpha}$`)
	if d[`\Sb`] != 0 {
		t.Errorf(`\Sb dropped without a star (%v)`, d)
	}
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`\IfBooleanTF took the TRUE branch with no star: %d drop(s) (%v)`, n, d)
	}
}

// ⛔ The star must be CONSUMED on a match and left alone otherwise. If it were consumed
// either way, the mandatory argument after it would be taken from the wrong place; if it
// were never consumed, the * would stay in the formula. Both are observable.
func TestTheStarIsConsumedExactlyWhenPresent(t *testing.T) {
	// Consumed: the mandatory argument is {\alpha}, not the star, so the body {#2}
	// renders \alpha and nothing drops.
	d := mathDropRun(t, `\NewDocumentCommand \Sc {s m} {#2}$\Sc*{\alpha}$`)
	if len(d) != 0 {
		t.Errorf(`something dropped: the star was not consumed cleanly (%v)`, d)
	}
	// Not consumed when absent: {\nosuchmaththing} must still be #2 and reach the body.
	d = mathDropRun(t, `\NewDocumentCommand \Sc {s m} {#2}$\Sc{\nosuchmaththing}$`)
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`the mandatory argument was not taken when no star was present (%v)`, d)
	}
}

// t<c> tests any single character, not only a star. "t!" is what a corpus paper writes.
func TestATokenTestWorksForACharacterOtherThanAStar(t *testing.T) {
	d := mathDropRun(t, `\NewDocumentCommand \Tb {t! m} {\IfBooleanTF{#1}{\nosuchmaththing}{#2}}`+
		`$\Tb!{\alpha}$`)
	if d[`\nosuchmaththing`] == 0 {
		t.Errorf(`the ! was not recognised as present (%v)`, d)
	}
	d = mathDropRun(t, `\NewDocumentCommand \Tb {t! m} {\IfBooleanTF{#1}{\nosuchmaththing}{#2}}`+
		`$\Tb{\alpha}$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`the ! was seen although absent: %d drop(s) (%v)`, n, d)
	}
}

// The full corpus shape: "s O{\lambda} m m", from a real paper. It exercises the star,
// a defaulted optional and two mandatory arguments in one specification, in that order.
func TestTheCorpusSpecificationWithAStarAnOptionalAndTwoMandatory(t *testing.T) {
	decl := `\NewDocumentCommand \Ren {s O{\lambda} m m}{\IfBooleanTF{#1}{#2#3}{#2#4}}`
	// Star given: #3 is used, so the unknown in #4 must vanish.
	d := mathDropRun(t, decl+`$\Ren*[\mu]{\alpha}{\nosuchmaththing}$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`with the star, #4 was used instead of #3: %d drop(s) (%v)`, n, d)
	}
	// No star, optional absent: #2 is the default \lambda and #4 is used.
	d = mathDropRun(t, decl+`$\Ren{\alpha}{\beta}$`)
	if len(d) != 0 {
		t.Errorf(`the default-plus-two-mandatory form dropped (%v)`, d)
	}
}

// The star is found whatever spacing the SOURCE carries. ⚠ This does not exercise the
// space-skipping loop: scanMathSource collapses runs of spaces and the needle consumes
// the one it emits, so all four forms below reach the parser identically — measured, and
// an ablation of that loop breaks nothing. What the test does pin is the end-to-end
// behaviour a paper depends on, which is worth having whatever resolves it.
func TestSpacesBeforeAStarAreSkipped(t *testing.T) {
	decl := `\NewDocumentCommand \Sd {s m} {\IfBooleanTF{#1}{\nosuchmaththing}{#2}}`
	for _, use := range []string{`$\Sd*{\alpha}$`, `$\Sd *{\alpha}$`, `$\Sd  *{\alpha}$`} {
		d := mathDropRun(t, decl+use)
		if d[`\nosuchmaththing`] == 0 {
			t.Errorf(`%s: the star was not seen through the spaces (%v)`, use, d)
		}
	}
	// And a space does NOT invent a star where there is none.
	d := mathDropRun(t, decl+`$\Sd  {\alpha}$`)
	if n := d[`\nosuchmaththing`]; n != 0 {
		t.Errorf(`spaces alone were read as a star: %d drop(s) (%v)`, n, d)
	}
}

// ⛔ An APPROXIMATED specification stays out of the maths path entirely. parseXparseSpec
// models e/E by consuming the group and appending NO descriptor (so existing #n keep
// their numbers) and v by taking the argument as ordinary mandatory — both long-standing
// text-mode behaviour. The maths layer substitutes into a string and cannot run on an
// arity it does not trust, so it must refuse rather than mangle silently.
func TestAnApproximatedSpecificationStaysOutOfTheMathsPath(t *testing.T) {
	for _, c := range []struct{ name, src string }{
		{`\Emb`, `\NewDocumentCommand \Emb {m E{_^}{{}{}}} {#1}$\Emb{a}$`},
		{`\Vrb`, `\NewDocumentCommand \Vrb {v} {#1}$\Vrb|x|$`},
	} {
		d := mathDropRun(t, c.src)
		if d[c.name] == 0 {
			t.Errorf(`%s was served although its specification is only approximated (%v)`, c.name, d)
		}
	}
	// A specification with NO approximation must still be served, so the flag is not a
	// blanket refusal.
	d := mathDropRun(t, `\NewDocumentCommand \Ok {s o m} {#3}$\Ok{\alpha}$`)
	if n := d[`\Ok`]; n != 0 {
		t.Errorf(`an exact specification was refused: %d drop(s) (%v)`, n, d)
	}
}
