// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// TestPackageConfigStubsRound2: the second batch of package-config gobbles
// (\DeclareGraphicsExtensions, \sisetup, \setuptodonotes, \selectlanguage, \tikzstyle
// with its =[…] value, \addbibresource with and without an optional argument, and the
// deprecated \sc font switch) all complete without error and emit nothing.
func TestPackageConfigStubsRound2(t *testing.T) {
	if out := ckRun(t, `\DeclareGraphicsExtensions{.pdf}\sisetup{a}\setuptodonotes{b}`+
		`\selectlanguage{english}\tikzstyle{n}=[draw,fill]\addbibresource{r.bib}`+
		`\addbibresource[l=x]{m.bib}{\sc small}\message{OK}`); out != "OK" {
		t.Errorf("round-2 config gobbles: message=%q, want OK", out)
	}
}

// TestGeneralCommandStubs: the package-config gobbles (\mathtoolsset, \setkeys,
// \lstset, \lstdefinestyle, \enlargethispage) complete without error and emit
// nothing, while \IEEEPARstart keeps its drop-cap letter and the rest of the word.
func TestGeneralCommandStubs(t *testing.T) {
	if out := ckRun(t, `\mathtoolsset{x}\setkeys{a}{b}\lstset{c}\lstdefinestyle{d}{e}`+
		`\enlargethispage{1cm}\enlargethispage*{1cm}\message{OK}`); out != "OK" {
		t.Errorf("config gobbles: message=%q, want OK", out)
	}
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	e.SetFont(spMock{})
	if _, err := e.Run(`\hsize=300pt \IEEEPARstart{T}{his} sentence.`); err != nil {
		t.Fatal(err)
	}
	if txt := treeText(e); !strings.Contains(txt, "This") {
		t.Errorf("\\IEEEPARstart dropped its text; got %q", txt)
	}
}

// TestTitlesecStubs: every titlesec \titleformat / \titlespacing form — star and
// non-star, with and without the shape / after / right optionals — is gobbled
// (completes without error, emits nothing), so the following \message fires.
func TestTitlesecStubs(t *testing.T) {
	src := `\titleformat{\section}{\large}{\thesection}{1em}{}` +
		`\titleformat{\subsection}[block]{\normalsize}{\thesubsection}{1em}{}[after]` +
		`\titleformat*{\paragraph}{\bfseries}` +
		`\titlespacing{\section}{0pt}{2ex}{1ex}[right]` +
		`\titlespacing*{\subsection}{0pt}{1ex}{1ex}` +
		`\message{OK}`
	if out := ckRun(t, src); out != "OK" {
		t.Errorf("titlesec gobbles: message=%q, want OK", out)
	}
}

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

// The LaTeX list-machinery booleans (\if@nmbrlist, \if@newlist, \if@noparitem,
// \if@noparlist, \if@inlabel) are defined, so a real class's \trivlist/\@item
// code that toggles them does not hit an undefined control sequence (which,
// undefined, leaves the list open and swallows the following input).
func TestListMachineryBooleans(t *testing.T) {
	for _, b := range []string{"@nmbrlist", "@newlist", "@noparitem", "@noparlist", "@inlabel"} {
		// Set true → \if… takes the true branch; set false → false branch; and none
		// of \if@X, \@Xtrue, \@Xfalse may be an undefined control sequence.
		src := `\@nameuse{` + b + `true}\csname if` + b + `\endcsname\message{T}\fi` +
			`\@nameuse{` + b + `false}\csname if` + b + `\endcsname\else\message{F}\fi`
		out, err := ckRunErr(t, src)
		if err != nil {
			t.Errorf("\\if%s not usable: %v", b, err)
			continue
		}
		if out != "T F" {
			t.Errorf("\\if%s toggling: message=%q, want \"T F\"", b, out)
		}
	}
}

// \loop … \repeat must run, and — critically — \repeat is \let to \fi so that
// TeX's skip over a FALSE \if branch treats a \loop\ifnum…\repeat body as
// balanced. A skipped branch containing an unclosed \ifnum (had \repeat not
// counted as \fi) would overrun its \else/\fi and swallow the text that follows
// — the mechanism behind the acl.sty conference-style 0-page bug.
func TestLoopRepeat(t *testing.T) {
	// Executed loop: count from 1, five iterations, emit a mark each time.
	if out := ckRun(t, `\newcount\n \n=0 \loop\advance\n 1 \message{x}\ifnum\n<5 \repeat`); out != "x x x x x" {
		t.Errorf("executed \\loop: message=%q, want \"x x x x x\"", out)
	}
	// Skipped false branch whose body holds \loop\ifnum…\repeat: the \message
	// AFTER the \fi must still fire (skip stayed balanced, nothing swallowed).
	if out := ckRun(t, `\newif\ifX \ifX \loop\ifnum1<2 \repeat\else\fi\message{after}`); out != "after" {
		t.Errorf("skipped \\loop\\repeat branch swallowed following text: message=%q, want \"after\"", out)
	}
}

// \@ifnextchar peeks the next token and compares it (\ifx) to a target that may
// be a control sequence — not just '['. A list scanner that stops on a control-
// sequence sentinel (elsarticle/bmvc2k author lists use exactly this) must
// terminate; the old bracket-only fallback always took ELSE and looped forever.
func TestIfnextcharTargets(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// list map that stops on a \stop (=\relax) sentinel
		{"cs-sentinel", `\makeatletter\let\stop\relax\def\R{}` +
			`\def\ml#1{\edef\R{\R#1}\@ifnextchar\stop{\@gobble}{\ml}}` +
			`\ml{a}{b}{c}\stop\message{\R}`, "abc"},
		// '[' still detected (optional-argument look-ahead)
		{"bracket-present", `\makeatletter\def\c{\@ifnextchar[{\def\R{B}}{\def\R{N}}}\c[x]\message{\R}`, "B"},
		{"bracket-absent", `\makeatletter\def\c{\@ifnextchar[{\def\R{B}}{\def\R{N}}}\c y\message{\R}`, "N"},
		// a \relax target is matched by a following \relax
		{"relax-target", `\makeatletter\def\c{\@ifnextchar\relax{\def\R{R}}{\def\R{N}}}\c\relax\message{\R}`, "R"},
	}
	for _, c := range cases {
		if got := ckRun(t, c.src); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
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
		{"xxvpt", `\message{\@xxvpt}`, "24.88"}, // latex.ltx:7863, not a round 25
		{"xipt", `\message{\@xipt}`, "10.95"},
		// \p@ as a dimen prints 1.0pt
		{"pat", `\message{\the\p@}`, "1.0pt"},
		// default counter / flag reads
		{"secnumdepth", `\message{\the\c@secnumdepth}`, "3"},
		{"arabic", `\message{\@arabic{\c@secnumdepth}}`, "3"},
		// text symbols
		{"symbols", `\message{[\textbullet\textendash\textemdash\textasteriskcentered\textperiodcentered]}`, "[•–—*·]"},
		// math alphabet aliases are identity text wrappers
		{"mathalias", `\message{[\mathrm{x}\mathbf{y}\mathit{z}\mathsf{s}\mathtt{t}\mathcal{c}\mathnormal{n}\mathfrak{f}\mathscr{r}]}`, "[xyzstcnfr]"},
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

// The classic rubber-length idiom \z@ \@plus 3\p@ now assembles its full glue: the
// engine's glue scanner expands while matching the plus/minus keywords and supports
// TeX's factor×internal-dimen products (6\p@ = 6×1pt). Both the \setlength form and
// the direct register-assignment form (as size1x.clo uses) are covered.
func TestClassKernelRubberGlue(t *testing.T) {
	got, err := ckRunErr(t, `\setlength{\abovedisplayskip}{\z@ \@plus 3\p@}\message{[\the\abovedisplayskip]}`)
	if err != nil {
		t.Fatalf("rubber-glue idiom aborted the load: %v", err)
	}
	if got != "[0.0pt plus 3.0pt]" {
		t.Errorf("rubber idiom => %q, want [0.0pt plus 3.0pt]", got)
	}
	got, err = ckRunErr(t, `\abovedisplayskip 6\p@ \@plus 1.5\p@ \@minus 4\p@\message{[\the\abovedisplayskip]}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "[6.0pt plus 1.5pt minus 4.0pt]" {
		t.Errorf("direct rubber form => %q, want [6.0pt plus 1.5pt minus 4.0pt]", got)
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
		// \DeclareOldFontCommand is a no-op: it must NOT redefine the engine's own
		// \rm/\bf/… (real LaTeX's article.cls rebinds \rm to \normalfont\rmfamily,
		// which with our aliases would loop). It completes without looping.
		{"oldfontcmd", `\DeclareOldFontCommand{\rm}{\normalfont\rmfamily}{\mathrm}\message{OK}`, "OK"},
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

// \tikzstyle is written with spaces around its = as often as without, and the
// bracket group usually runs over several lines:
//
//	\tikzstyle{thmbox} = [
//	  draw=black, fill=white
//	]
//
// A delimited \def\tikzstyle#1=[#2]{} matches those characters EXACTLY, so with a
// space before the = it scans on for a literal "=[" that never comes and swallows
// the rest of the file. One arXiv paper lost all 18 of its pages that way.
func TestTikzstyleSpacedFormKeepsTheDocument(t *testing.T) {
	for _, src := range []string{
		"\\tikzstyle{thmbox} = [\n  draw=black, fill=white\n]\n",
		`\tikzstyle{thmbox}=[draw]`,
		`\tikzstyle{thmbox}`, // neither = nor [ follows: nothing to gobble
	} {
		e := New()
		if err := e.LoadLaTeX(); err != nil {
			t.Fatal(err)
		}
		e.SetFont(spMock{})
		if _, err := e.Run(`\documentclass{article}\begin{document}` + src + `APRES\par\end{document}`); err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if txt := mvlText(e.mvl); !strings.Contains(txt, "APRES") {
			t.Errorf("%q swallowed the document: page carries %q", src, txt)
		}
	}
}
