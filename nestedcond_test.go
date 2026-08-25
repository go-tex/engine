// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// A conditional's operand can expand into a conditional of its own. TeX runs the
// inner one while scanning the operand, and its \fi is then still in the input
// when the outer one comes to skip a false branch. Taking that \fi for one's own
// ends the skip at the wrong place, and the outer conditional executes the very
// branch it decided against.
//
// Every case below was measured against a real LaTeX (tectonic), the leading
// space in "[ EGAL]" included — the space after \if's two operands belongs to
// the branch, and it was the expectation that was wrong, not the engine.
func TestConditionalInsideAConditionalOperand(t *testing.T) {
	for _, c := range []struct{ name, src, want string }{
		{
			"l'interne rend l, donc les opérandes diffèrent",
			`\def\zn{1}\def\zp{\if 3\zn u\else l\fi}\message{[\if u\zp EGAL\else DIFF\fi]}`,
			"[DIFF]",
		},
		{
			"l'interne rend u, donc ils sont égaux",
			`\def\zn{3}\def\zp{\if 3\zn u\else l\fi}\message{[\if u\zp EGAL\else DIFF\fi]}`,
			"[EGAL]",
		},
		{
			"la même chose à travers \\csname",
			`\expandafter\def\csname zn\endcsname{1}` +
				`\expandafter\def\csname zp\endcsname{\if 3\csname zn\endcsname u\else l\fi}` +
				`\message{[\if u\csname zp\endcsname EGAL\else DIFF\fi]}`,
			"[DIFF]",
		},
		{
			"deux niveaux d'imbrication",
			`\def\zn{1}\def\zq{\if 3\zn a\else b\fi}\def\zp{\if b\zq x\else y\fi}` +
				`\message{[\if x\zp EGAL\else DIFF\fi]}`,
			"[EGAL]",
		},
		{
			"le conditionnel interne seul, hors opérande",
			`\def\zn{1}\def\zp{\if 3\zn u\else l\fi}\message{[\zp]}`,
			"[l]",
		},
		{
			"sans imbrication, rien ne change",
			`\message{[\if aa EGAL\else DIFF\fi][\if ab EGAL\else DIFF\fi]}`,
			"[ EGAL][DIFF]",
		},
		{
			"l'interne prend SA branche vraie : son \\else ET son \\fi restent",
			`\def\zn{3}\def\zp{\if 3\zn u\else l\fi}\message{[\if l\zp EGAL\else DIFF\fi]}`,
			"[DIFF]",
		},
		{
			"imbriqué dans la branche vraie d'un autre conditionnel",
			`\def\zn{3}\def\zp{\if 3\zn u\else l\fi}` +
				`\message{[\iftrue\if u\zp OUI\else NON\fi\else RIEN\fi]}`,
			"[OUI]",
		},
		{
			"\\ifcat prend le même chemin",
			`\def\zn{1}\def\zp{\if 3\zn u\else l\fi}\message{[\ifcat u\zp EGAL\else DIFF\fi]}`,
			"[EGAL]",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, err := buildEngine(Options{Lenient: true}, true)
			if err != nil {
				t.Fatal(err)
			}
			out, err := e.Run(`\documentclass{article}` + c.src)
			if err != nil {
				t.Fatal(err)
			}
			if got := trimNL(out); got != c.want {
				t.Errorf("%s\n  obtenu %q, attendu %q", c.src, got, c.want)
			}
		})
	}
}

// What it cost in practice. pgfplots asks which side an axis' tick labels go on
// with \if u\pgfplots@xticklabel@pos, and that macro IS a conditional:
//
//	\if 3\csname pgfplots@xtickposnum\endcsname u\else l\fi
//
// It answers l — the lower side — but the outer \if read u, so the tick label
// axis came out as the TOP of the plot instead of the bottom, the outer normal
// pointed the wrong way, and every tick label sat on the far side of its axis.
func TestPgfplotsTickLabelSideIdiom(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\makeatletter` +
		`\def\pgfplots@xtickposnum{1}` +
		`\def\pgfplots@xticklabel@pos{\if 3\pgfplots@xtickposnum u\else l\fi}` +
		`\message{[pos=\pgfplots@xticklabel@pos]}` +
		`\message{[haut=\if u\pgfplots@xticklabel@pos OUI\else NON\fi]}` +
		`\makeatother`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "[pos=l] [haut=NON]"; trimNL(out) != want {
		t.Errorf("obtenu %q, attendu %q", trimNL(out), want)
	}
}

// The other direction: an operand scan can CLOSE a conditional that was already
// open, by expanding a \fi that belongs to it. There is then nothing pending —
// fewer conditionals are open than when the scan began — and the count must not
// go negative. Measured against a real LaTeX, which prints X here.
func TestOperandScanClosingAnOuterConditional(t *testing.T) {
	e, err := buildEngine(Options{Lenient: true}, true)
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(`\documentclass{article}\message{[\iftrue\if u\fi X\fi]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := trimNL(out); got != "[X]" {
		t.Errorf("obtenu %q, attendu \"[X]\"", got)
	}
}
