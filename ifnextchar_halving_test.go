// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// \@ifnextchar does not INSERT the branch it picks — it stores it first.
// ltdefns.dtx:
//
//	\long\def\@ifnextchar#1#2#3{\let\reserved@d=#1
//	  \def\reserved@a{#2}\def\reserved@b{#3}\futurelet\@let@token\@ifnch}
//
// and \def\reserved@b{#3} SCANS the branch as a macro body, which halves ## a second
// time (tex.web §479: a # followed by a # stores one #).
//
// keyval is built on exactly that:
//
//	\def\define@key#1#2{\@ifnextchar[{\KV@def{#1}{#2}}{\long\@namedef{KV@#1@#2}####1}}
//
// The #### is halved once into \define@key's own body and a second time by
// \@ifnextchar, leaving the one # that \def reads as the parameter text #1. Inserting
// the branch verbatim skipped that halving, so the generated macro's parameter text
// was ##1 and bound nothing — every key VALUE was lost.
//
// Checked against real LaTeX: \meaning\KV@fam@k is `\long macro:#1-><<#1>>` there and
// was `macro:##1-><<#1>>` here.

func ihRun(t *testing.T, src string) string {
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

func TestKeyvalKeepsItsValue(t *testing.T) {
	out := ihRun(t, `\documentclass{article}\usepackage{keyval}\makeatletter`+
		`\define@key{fam}{k}{\edef\rr{<<#1>>}}\setkeys{fam}{k=VAL}\message{[\rr]}`)
	if !strings.Contains(out, "[<<VAL>>]") {
		t.Errorf("got %q, want [<<VAL>>] — the key's value was lost", out)
	}
}

func TestGeneratedKeyMacroHasOneHashInItsParameterText(t *testing.T) {
	out := ihRun(t, `\documentclass{article}\usepackage{keyval}\makeatletter`+
		`\define@key{fam}{k}{<<#1>>}\message{[\meaning\KV@fam@k]}`)
	if strings.Contains(out, "##1->") {
		t.Errorf("parameter text still has the extra #: %q", out)
	}
	if !strings.Contains(out, "macro:#1-><<#1>>") {
		t.Errorf("got %q, want macro:#1-><<#1>>", out)
	}
}

func TestIfstarBranchIsHalvedToo(t *testing.T) {
	// ltdefns.dtx builds \@ifstar on \@ifnextchar — \def\@ifstar#1{\@ifnextchar
	// *{\@firstoftwo{#1}}} — so its branch goes through the same \def and the same
	// halving. This engine picks the branch itself and must halve for the same reason.
	out := ihRun(t, `\documentclass{article}\makeatletter`+
		`\def\mk{\@ifstar{\long\@namedef{ZZ}####1}{\long\@namedef{ZZ}####1}}`+
		`\mk{<<#1>>}\message{[\meaning\ZZ]}`)
	if strings.Contains(out, "##1->") {
		t.Errorf("\\@ifstar's branch was not halved: %q", out)
	}
}

func TestHalvingLeavesOrdinaryBranchesAlone(t *testing.T) {
	// The control: a branch with no parameter hashes must come through untouched.
	out := ihRun(t, `\documentclass{article}\makeatletter`+
		`\@ifnextchar[{\message{[CROCHET]}}{\message{[SANS]}}x`)
	if !strings.Contains(out, "[SANS]") {
		t.Errorf("got %q, want [SANS]", out)
	}
}

// The TARGET of \@ifnextchar is an ARGUMENT, not a token. ltdefns.dtx does
// \let\reserved@d=#1, so a brace group around it is stripped and
// \@ifnextchar{*}{A}{B} tests for a star exactly as \@ifnextchar*{A}{B} does.
//
// Reading it with one getNext took the BRACE as the target: both branches were then
// emitted and the star was typeset. Judged against tectonic 0.17.0 on
//
//	\def\tb{\@ifnextchar{*}{YES}{NO}}   A\tb* B\tb x C
//	\def\tn{\@ifnextchar*{YES}{NO}}     D\tn* E\tn y F
//
// the reference gives "AYES* BNOx C" and "DYES* ENOy F"; the braced line came out
// "A*YESNO* B*YESNOx C". Only the braced form was wrong, which is why every earlier
// witness (all of them bare) passed.
//
// It is not a corner case: xkeyval reaches it through xkvutils' \@ifnextcharacter,
// whose first branch is taken whenever the next token is a brace — \setkeys{fam}{…},
// the common case. Every \setkeys printed "*+" and left two groups open
// (go-tex/engine#306).
func TestIfnextcharTargetIsAnArgumentNotAToken(t *testing.T) {
	for _, c := range []struct{ name, def, want, reject string }{
		{"braced target, star present", `\def\t{\@ifnextchar{*}{YES}{NO}}\t*`, "ranYES", "ranNO"},
		{"braced target, star absent", `\def\t{\@ifnextchar{*}{YES}{NO}}\t x`, "ranNO", "ranYES"},
		{"bare target, star present", `\def\t{\@ifnextchar*{YES}{NO}}\t*`, "ranYES", "ranNO"},
		{"bare target, star absent", `\def\t{\@ifnextchar*{YES}{NO}}\t y`, "ranNO", "ranYES"},
	} {
		// The branches define a macro globally, so the test reads the engine's table
		// rather than the render: \end{document} pops the document group, and a
		// glyph count is not one path per character in this engine's SVG.
		def := strings.NewReplacer(
			"{YES}", `{\gdef\ranYES{}}`, "{NO}", `{\gdef\ranNO{}}`).Replace(c.def)
		src := `\documentclass{article}\makeatletter` + def + `\makeatother\begin{document}x\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true, Size: 11})
		if err != nil {
			t.Fatalf("%s: compile: %v", c.name, err)
		}
		if e.meaningOf(csTok(c.want)) == nil {
			t.Errorf("%s: %s branch did not run", c.name, c.want)
		}
		if e.meaningOf(csTok(c.reject)) != nil {
			t.Errorf("%s: %s branch ALSO ran — both were emitted", c.name, c.reject)
		}
	}
}
