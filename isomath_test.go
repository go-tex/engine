// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// isomath.sty is embedded, so \usepackage{isomath} resolves with no TeX distribution
// present. It was found by the dropped-equation census rather than by reading a
// wishlist: its absence cost 56 equations on one arXiv paper (2305.01199) —
// \matrixsym 30, \vectorsym 19, \tensorsym 7 — which is content the reader never
// sees, and which the page count does not notice, since the paper stays at 27 pages
// either way.
//
// Measured on that paper: 56 dropped equations become 0 and the drawn paths go from
// 52,499 to 54,656. It is the FILE that recovers them, not go-tex/math's new
// alphabets — the same test at math v0.35.0, before \mathbfit and \mathsfbfit
// existed, also drops nothing and draws 54,650. The alphabets are worth 6 paths, and
// what they buy is the right face for a tensor rather than the content.
//
// The engine runs the real file rather than emulating it, which is this repository's
// stated preference and here it matters twice: \vectorsym's digit test
// (\ifnum9<1#1, isomath.sty:260) keeps ISO's upright numbers, and reimplementing
// that in Go would be a second thing to keep in step with upstream.
func TestISOMathPackageLoads(t *testing.T) {
	src := `\documentclass{article}\usepackage{isomath}\begin{document}` +
		`$\vectorsym{v}+\matrixsym{A}+\tensorsym{T}$\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	// mathDropped is the witness, not the page characters: a refused formula leaves
	// the page silently shorter, and under -lenient it does not fail the build either.
	if len(e.mathDropped) != 0 {
		t.Errorf("the math layer refused the formula: %v", e.mathDropped)
	}
}

// isomath opens with \RequirePackage{fixmath} and \RequirePackage{kvoptions}
// (isomath.sty:68, :76), and NEITHER is embedded. The package still has to work: the
// loader is tolerant, so a \RequirePackage it cannot resolve is skipped and the
// \providecommand definitions further down still run.
//
// Asserted because it is the whole reason embedding one 13KB file is enough. If a
// missing \RequirePackage ever became fatal, this package would stop working and the
// 56 equations would come back — silently, since a dropped equation does not fail a
// build.
func TestISOMathLoadsWithoutItsOwnRequirements(t *testing.T) {
	for _, dep := range []string{"fixmath", "kvoptions"} {
		if _, _, ok := (&Engine{}).hostTeXFile(dep + ".sty"); ok {
			t.Errorf("%s.sty is now resolvable — this test no longer proves what it says", dep)
		}
	}
	// \vectorsym on a DIGIT takes the \mathbf branch, on a letter the \mathbfit one.
	// Both must reach the maths layer without the package's requirements present.
	for _, body := range []string{`$\vectorsym{1}$`, `$\vectorsym{x}$`, `$\tensorsym{T}$`} {
		src := `\documentclass{article}\usepackage{isomath}\begin{document}` + body + `\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Errorf("%s: %v", body, err)
			continue
		}
		if len(e.mathDropped) != 0 {
			t.Errorf("%s: refused by the math layer: %v", body, e.mathDropped)
		}
	}
}
