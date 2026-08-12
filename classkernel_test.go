// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// ckRun loads Plain+LaTeX (which now includes the LaTeX2eClassKernel substrate)
// into a fresh engine and returns the trimmed \message output of src.
func ckRun(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatalf("LoadLaTeX: %v", err)
	}
	got, err := e.Run(src)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return trimNL(got)
}

// ckRunErr is like ckRun but returns the error instead of failing, so a test can
// assert that a command completes WITHOUT error (best-effort no-ops must never
// abort a class load).
func ckRunErr(t *testing.T, src string) (string, error) {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatalf("LoadLaTeX: %v", err)
	}
	got, err := e.Run(src)
	return trimNL(got), err
}

// Purely expandable results: the constant / size-number / char macros expand
// inside a single wrapping \message.
func TestClassKernelExpandable(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// big integer constants
		{"bigM", `\message{\the\@M}`, "10000"},
		{"smallm", `\message{\the\@m}`, "1000"},
		{"Mi", `\message{\the\@Mi}`, "10001"},
		{"Miv", `\message{\the\@Miv}`, "10004"},
		// font-size number macros
		{"vpt", `\message{\@vpt}`, "5"},
		{"xpt", `\message{\@xpt}`, "10"},
		{"xiipt", `\message{\@xiipt}`, "12"},
		{"xxvpt", `\message{\@xxvpt}`, "25"},
		{"xipt", `\message{\@xipt}`, "10.95"},
		// \p@ as a dimen prints 1.0pt
		{"pat", `\message{\the\p@}`, "1.0pt"},
		// default counter / flag reads
		{"secnumdepth", `\message{\the\c@secnumdepth}`, "3"},
		{"arabic", `\message{\@arabic{\c@secnumdepth}}`, "3"},
		// text symbols
		{"symbols", `\message{[\textbullet\textendash\textemdash\textasteriskcentered\textperiodcentered]}`, "[•–—*·]"},
		// math alphabet aliases are identity text wrappers
		{"mathalias", `\message{[\mathrm{x}\mathbf{y}\mathit{z}\mathsf{s}\mathtt{t}\mathcal{c}\mathnormal{n}]}`, "[xyzstcn]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ckRun(t, c.src); got != c.want {
				t.Errorf("src %q => %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// \if@ boolean flags: \newif created the switch and its true/false setters; test
// BOTH branches so the false default and the explicit true setter are covered.
func TestClassKernelFlags(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"twocolumn-default-false", `\message{\if@twocolumn T\else F\fi}`, "F"},
		{"twocolumn-set-true", `\@twocolumntrue\message{\if@twocolumn T\else F\fi}`, "T"},
		{"twocolumn-set-false", `\@twocolumntrue\@twocolumnfalse\message{\if@twocolumn T\else F\fi}`, "F"},
		{"twoside-default-false", `\message{\if@twoside T\else F\fi}`, "F"},
		{"compatibility-false", `\message{\if@compatibility T\else F\fi}`, "F"},
		{"nobreak-set-true", `\@nobreaktrue\message{\if@nobreak T\else F\fi}`, "T"},
		{"minipage-false", `\message{\if@minipage T\else F\fi}`, "F"},
		{"mparswitch-false", `\message{\if@mparswitch T\else F\fi}`, "F"},
		{"titlepage-false", `\message{\if@titlepage T\else F\fi}`, "F"},
		{"restonecol-false", `\message{\if@restonecol T\else F\fi}`, "F"},
		{"afterindent-set-true", `\@afterindenttrue\message{\if@afterindent T\else F\fi}`, "T"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ckRun(t, c.src); got != c.want {
				t.Errorf("src %q => %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// Registers accept assignments (via \setlength or a bare register write) and read
// back through \the — the substrate's core purpose.
func TestClassKernelRegisters(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// \z@ (a \newdimen from the kernel-helper layer) interoperates with \setlength
		{"setlength-z@", `\setlength{\parindent}{\z@}\message{[\the\parindent]}`, "[0.0pt]"},
		// list / layout dimens
		{"leftmargini", `\setlength{\leftmargini}{2em}\message{[\the\leftmargini]}`, "[20.0pt]"},
		{"fboxsep", `\fboxsep=3pt\relax\message{[\the\fboxsep]}`, "[3.0pt]"},
		{"itemsep", `\itemsep=3pt\relax\message{[\the\itemsep]}`, "[3.0pt]"},
		{"labelsep", `\setlength{\labelsep}{5pt}\message{[\the\labelsep]}`, "[5.0pt]"},
		// penalty counts
		{"clubpenalty", `\clubpenalty=150\relax\message{[\the\clubpenalty]}`, "[150]"},
		{"lowpenalty-default", `\message{[\the\@lowpenalty]}`, "[51]"},
		// counters
		{"c@footnote", `\c@footnote=2\relax\message{[\the\c@footnote]}`, "[2]"},
		// \footins allocated as a high, unused register class: \skip\footins is a
		// valid scratch store that never collides with a \newskip allocation.
		{"skip-footins", `\skip\footins=9pt\relax\message{[\the\skip\footins]}`, "[9.0pt]"},
		{"dimen-footins", `\dimen\footins=7pt\relax\message{[\the\dimen\footins]}`, "[7.0pt]"},
		// a rigid multiple of \p@ reads back correctly (N\p@ = N pt)
		{"rigid-pat", `\abovedisplayskip=10\p@\relax\message{[\the\abovedisplayskip]}`, "[10.0pt]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ckRun(t, c.src); got != c.want {
				t.Errorf("src %q => %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// Documented interop LIMITATION: the classic rubber-length idiom
// \z@ \@plus 3\p@ does NOT assemble its stretch (the engine's glue scanner does
// not expand while matching the plus/minus keywords, and has no factor×register
// product). It must still complete WITHOUT error and store the rigid part, so a
// class load is not aborted. This test pins that actual behaviour — if the engine
// core later teaches the scanner the idiom, update the want to include the
// stretch and this test will flag it.
func TestClassKernelRubberGlueLimitation(t *testing.T) {
	got, err := ckRunErr(t, `\setlength{\abovedisplayskip}{\z@ \@plus 3\p@}\message{[\the\abovedisplayskip]}`)
	if err != nil {
		t.Fatalf("rubber-glue idiom aborted the load: %v", err)
	}
	if got != "[0.0pt]" {
		t.Errorf("rubber idiom => %q, want [0.0pt] (rigid part only; see file header)", got)
	}
}

// The declaration commands a class preamble runs must define / accept without
// aborting, and must not swallow following tokens.
func TestClassKernelDeclarations(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// \DeclareRobustCommand behaves like \newcommand, with or without '*'
		{"declarerobust", `\DeclareRobustCommand\ckfoo{X}\message{[\ckfoo]}`, "[X]"},
		{"declarerobust-star", `\DeclareRobustCommand*\ckbar{Y}\message{[\ckbar]}`, "[Y]"},
		{"declarerobust-optarg", `\DeclareRobustCommand\ckq[1]{<#1>}\message{[\ckq{Z}]}`, "[<Z>]"},
		{"checkcommand", `\CheckCommand\ckbaz{Z}\message{[\ckbaz]}`, "[Z]"},
		// \NeedsTeXFormat eats an optional [date] but never the text that follows
		{"needstex-bracket", `\NeedsTeXFormat{LaTeX2e}[1994/12/01]\message{AFTER}`, "AFTER"},
		{"needstex-nobracket", `\NeedsTeXFormat{LaTeX2e}\message{AFTER2}`, "AFTER2"},
		// \Provides* likewise
		{"providesclass", `\ProvidesClass{article}[2024/01/01 v1]\message{OK}`, "OK"},
		{"providespackage", `\ProvidesPackage{amsmath}\message{OK}`, "OK"},
		{"providesfile", `\ProvidesFile{foo.cfg}[2024]\message{OK}`, "OK"},
		// \DeclareOldFontCommand binds the one-token font command to its text form
		{"oldfontcmd", `\DeclareOldFontCommand\ckrm{ROMAN}{\mathrm}\message{[\ckrm]}`, "[ROMAN]"},
		// running-head marks are accepted and their \...mark macros stay empty
		{"marks", `\markboth{L}{R}\markright{r}\@mkboth{a}{b}\message{[\leftmark|\rightmark]}`, "[|]"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ckRun(t, c.src); got != c.want {
				t.Errorf("src %q => %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// Best-effort no-ops and font-switch aliases must run without aborting the load.
func TestClassKernelNoOps(t *testing.T) {
	srcs := []string{
		`\normalfont\rmfamily\sffamily\ttfamily\bfseries\mdseries\itshape\slshape\scshape\upshape\message{fonts}`,
		`\MakeUppercase{abc}\MakeLowercase{ABC}\message{case}`,
		`\addcontentsline{toc}{section}{Intro}\addtocontents{lof}{x}\message{toc}`,
		`\addvspace{4pt}\addpenalty{5}\message{vspace}`,
		`\MakeRobust\somecmd\@nomath\x\message{robust}`,
		`\nobreakspace\null\message{struct}`,
		`\@fontswitch{a}{b}\message{fontswitch}`,
	}
	wants := []string{"fonts", "case", "toc", "vspace", "robust", "struct", "fontswitch"}
	for i, s := range srcs {
		t.Run(wants[i], func(t *testing.T) {
			got, err := ckRunErr(t, s)
			if err != nil {
				t.Fatalf("Run(%q): %v", s, err)
			}
			if got != wants[i] {
				t.Errorf("src %q => %q, want %q", s, got, wants[i])
			}
		})
	}
}

// \@font@warning routes to \message by design (never aborts a load).
func TestClassKernelFontWarning(t *testing.T) {
	if got := ckRun(t, `\@font@warning{substituted}`); got != "Font Warning: substituted" {
		t.Errorf("@font@warning => %q", got)
	}
}

// The substrate loads cleanly and defines its representative control sequences
// (a guard that LoadLaTeX wires LaTeX2eClassKernel in and it parses).
func TestClassKernelLoads(t *testing.T) {
	// \@ifundefined (from the kernel-helper layer) reports each name defined; a
	// fresh engine per probe keeps the \message capture isolated.
	for _, name := range []string{
		"p@", "@M", "@xpt", "@tempboxa", "@tempdimc", "leftmargini",
		"abovedisplayskip", "footins", "clubpenalty", "if@twocolumn",
		"bfseries", "DeclareRobustCommand", "NeedsTeXFormat", "textbullet",
	} {
		if got := ckRun(t, `\message{\@ifundefined{`+name+`}{UNDEF}{ok}}`); got != "ok" {
			t.Errorf("\\%s reported %q, want ok (should be defined)", name, got)
		}
	}
}
