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
