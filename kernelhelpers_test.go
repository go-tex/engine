// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import "testing"

// khRun loads Plain+LaTeX (which now includes LaTeX2eKernelHelpers) into a fresh
// engine and returns the trimmed \message output of src.
func khRun(t *testing.T, src string) string {
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

// Table of one-\message assertions: helpers whose result is produced purely by
// expansion (so a single wrapping \message captures it).
func TestKernelHelpersExpandable(t *testing.T) {
	cases := []struct{ name, src, want string }{
		// selectors / gobblers
		{"gobble", `\message{[\@gobble{a}b]}`, "[b]"},
		{"gobbletwo", `\message{[\@gobbletwo{a}{b}c]}`, "[c]"},
		{"gobblethree", `\message{[\@gobblethree{a}{b}{c}d]}`, "[d]"},
		{"firstoftwo", `\message{\@firstoftwo{A}{B}}`, "A"},
		{"secondoftwo", `\message{\@secondoftwo{A}{B}}`, "B"},
		{"firstofone", `\message{\@firstofone{Q}}`, "Q"},
		{"iden", `\message{\@iden{Z}}`, "Z"},
		{"empty-ifx", `\message{\ifx\@empty\@empty T\else F\fi}`, "T"},
		// \csname naming
		{"namedef-nameuse", `\@namedef{foo}{X}\message{\@nameuse{foo}}`, "X"},
		// \@ifundefined both branches
		{"ifundef-undefined", `\message{\@ifundefined{undefinedxyz}{Y}{N}}`, "Y"},
		{"ifundef-defined", `\def\defd{}\message{\@ifundefined{defd}{Y}{N}}`, "N"},
		// list dissection
		{"car", `\message{[\@car abc\@nil]}`, "[a]"},
		{"cdr", `\message{[\@cdr abc\@nil]}`, "[bc]"},
		// backslash / percent char tokens
		{"backslashchar", `\message{[\@backslashchar]}`, `[\]`},
		{"percentchar", `\message{[\@percentchar]}`, "[%]"},
		// numeric constants
		{"ne-lt-tw", `\message{\ifnum\@ne<\tw@ Y\else N\fi}`, "Y"},
		{"thr", `\message{\ifnum\thr@@=3 Y\else N\fi}`, "Y"},
		{"mne", `\message{\the\m@ne}`, "-1"},
		// loaded-package registry (expandable predicates)
		{"pkg-notloaded", `\message{\@ifpackageloaded{foo}{Y}{N}}`, "N"},
		{"pkg-loaded", `\@namedef{ver@foo.sty}{2024}\message{\@ifpackageloaded{foo}{Y}{N}}`, "Y"},
		{"cls-notloaded", `\message{\@ifclassloaded{bar}{Y}{N}}`, "N"},
		{"cls-loaded", `\@namedef{ver@bar.cls}{2024}\message{\@ifclassloaded{bar}{Y}{N}}`, "Y"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := khRun(t, c.src); got != c.want {
				t.Errorf("src %q => %q, want %q", c.src, got, c.want)
			}
		})
	}
}

// \newif defines the switch and its true/false setters, in both directions.
func TestKernelNewif(t *testing.T) {
	if got := khRun(t, `\newif\ifqq\qqtrue\message{\ifqq T\else F\fi}`); got != "T" {
		t.Errorf("qqtrue => %q, want T", got)
	}
	if got := khRun(t, `\newif\ifqq\qqtrue\qqfalse\message{\ifqq T\else F\fi}`); got != "F" {
		t.Errorf("qqfalse => %q, want F", got)
	}
	// default is false immediately after \newif
	if got := khRun(t, `\newif\ifqq\message{\ifqq T\else F\fi}`); got != "F" {
		t.Errorf("newif default => %q, want F", got)
	}
	// a switch with an @ in its name (\if@tempswa is pre-declared)
	if got := khRun(t, `\@tempswatrue\message{\if@tempswa T\else F\fi}`); got != "T" {
		t.Errorf("@tempswatrue => %q, want T", got)
	}
}

// The LaTeX kernel while-loops repeat their \do body while the test holds.
// Classes drive frontmatter/list machinery with these; an undefined \@whiledim
// would be skipped and its delimited test/body mis-parse.
func TestKernelWhileLoops(t *testing.T) {
	// \@whilenum: count 0..4, emit a mark each iteration.
	if got := khRun(t, `\newcount\n\n=0 \@whilenum\n<5 \do{x\advance\n1 }\message{\the\n}`); got != "5" {
		t.Errorf("@whilenum final count => %q, want 5", got)
	}
	// \@whiledim: subtract 3pt from 10pt until ≤0 ⇒ 4 iterations (10,7,4,1).
	if got := khRun(t, `\newdimen\d\d=10pt \newcount\k\k=0 `+
		`\@whiledim\d>\z@ \do{\advance\k1 \advance\d-3pt }\message{\the\k}`); got != "4" {
		t.Errorf("@whiledim iterations => %q, want 4", got)
	}
}

// Registers allocated by the helper layer are assignable and readable.
func TestKernelRegisters(t *testing.T) {
	if got := khRun(t, `\@tempcnta=5 \message{\the\@tempcnta}`); got != "5" {
		t.Errorf("@tempcnta => %q, want 5", got)
	}
	// \z@ usable as the number 0
	if got := khRun(t, `\@tempcnta=\z@ \advance\@tempcnta by4 \message{\the\@tempcnta}`); got != "4" {
		t.Errorf("z@ as number => %q, want 4", got)
	}
	// \@tempskipa is a real skip register
	if got := khRun(t, `\@tempskipa=3pt \message{\the\@tempskipa}`); got != "3.0pt" {
		t.Errorf("@tempskipa => %q, want 3.0pt", got)
	}
}

// \g@addto@macro / \@addto@macro append tokens to a macro body.
func TestKernelAddToMacro(t *testing.T) {
	if got := khRun(t, `\def\foo{head}\g@addto@macro\foo{tail}\message{\foo}`); got != "headtail" {
		t.Errorf("g@addto@macro => %q, want headtail", got)
	}
	if got := khRun(t, `\def\foo{h}\@addto@macro\foo{t}\@addto@macro\foo{u}\message{\foo}`); got != "htu" {
		t.Errorf("@addto@macro => %q, want htu", got)
	}
}

// \@for iterates a comma list; \@tfor iterates a token list.
func TestKernelForLoops(t *testing.T) {
	if got := khRun(t, `\def\acc{}\@for\x:=a,b,c\do{\edef\acc{\acc[\x]}}\message{\acc}`); got != "[a][b][c]" {
		t.Errorf("@for => %q, want [a][b][c]", got)
	}
	// empty list body never runs
	if got := khRun(t, `\def\acc{Z}\@for\x:=\do{\edef\acc{\acc!}}\message{\acc}`); got != "Z" {
		t.Errorf("@for empty => %q, want Z", got)
	}
	if got := khRun(t, `\def\acc{}\@tfor\x:=abc\do{\edef\acc{\acc(\x)}}\message{\acc}`); got != "(a)(b)(c)" {
		t.Errorf("@tfor => %q, want (a)(b)(c)", got)
	}
}

// \zap@space removes every space from its argument (terminated by \@empty).
func TestKernelZapSpace(t *testing.T) {
	if got := khRun(t, `\edef\z{\zap@space a b c \@empty}\message{\z}`); got != "abc" {
		t.Errorf("zap@space => %q, want abc", got)
	}
	if got := khRun(t, `\edef\z{\zap@space nospace \@empty}\message{\z}`); got != "nospace" {
		t.Errorf("zap@space nospace => %q, want nospace", got)
	}
}

// \@ifinlist comma-list membership, both branches.
func TestKernelInList(t *testing.T) {
	if got := khRun(t, `\@ifinlist{b}{a,b,c}\message{\ifin@ Y\else N\fi}`); got != "Y" {
		t.Errorf("ifinlist present => %q, want Y", got)
	}
	if got := khRun(t, `\@ifinlist{z}{a,b,c}\message{\ifin@ Y\else N\fi}`); got != "N" {
		t.Errorf("ifinlist absent => %q, want N", got)
	}
	// list supplied as a macro is expanded before scanning
	if got := khRun(t, `\@namedef{mylist}{p,q,r}\@ifinlist{q}{\mylist}\message{\ifin@ Y\else N\fi}`); got != "Y" {
		t.Errorf("ifinlist macro-list => %q, want Y", got)
	}
}

// \@ifnextchar handles the [ case (the peekable one) in both directions.
func TestKernelIfNextChar(t *testing.T) {
	if got := khRun(t, `\def\thenbr[#1]{\message{T:#1}}\@ifnextchar[{\thenbr}{\message{ELSE}}[hi]`); got != "T:hi" {
		t.Errorf("ifnextchar [ present => %q, want T:hi", got)
	}
	if got := khRun(t, `\def\thenbr[#1]{\message{T:#1}}\@ifnextchar[{\thenbr}{\message{ELSE}}x`); got != "ELSE" {
		t.Errorf("ifnextchar [ absent => %q, want ELSE", got)
	}
}

// \@star@or@long consumes an optional * and runs the command either way.
func TestKernelStarOrLong(t *testing.T) {
	if got := khRun(t, `\def\d{\message{RUN}}\@star@or@long\d`); got != "RUN" {
		t.Errorf("star@or@long no-star => %q, want RUN", got)
	}
	if got := khRun(t, `\def\d{\message{RUN}}\@star@or@long\d*`); got != "RUN" {
		t.Errorf("star@or@long star => %q, want RUN", got)
	}
}

// \@ifdefinable runs the body only for an undefined name (best-effort).
func TestKernelIfDefinable(t *testing.T) {
	if got := khRun(t, `\@ifdefinable\brandnewname{\message{DEF}}`); got != "DEF" {
		t.Errorf("ifdefinable undefined => %q, want DEF", got)
	}
	if got := khRun(t, `\def\exists{}\@ifdefinable\exists{\message{DEF}}`); got != "" {
		t.Errorf("ifdefinable defined => %q, want empty", got)
	}
}

// \@ifpackagewith consults the recorded opt@<pkg>.sty option list.
func TestKernelIfPackageWith(t *testing.T) {
	if got := khRun(t, `\@namedef{opt@foo.sty}{a,b,c}\@ifpackagewith{foo}{b}{\message{Y}}{\message{N}}`); got != "Y" {
		t.Errorf("ifpackagewith present => %q, want Y", got)
	}
	if got := khRun(t, `\@namedef{opt@foo.sty}{a,b,c}\@ifpackagewith{foo}{z}{\message{Y}}{\message{N}}`); got != "N" {
		t.Errorf("ifpackagewith absent-opt => %q, want N", got)
	}
	if got := khRun(t, `\@ifpackagewith{foo}{b}{\message{Y}}{\message{N}}`); got != "N" {
		t.Errorf("ifpackagewith not-loaded => %q, want N", got)
	}
}

// The logging family routes to \message instead of aborting.
func TestKernelLogging(t *testing.T) {
	if got := khRun(t, `\PackageWarning{mypkg}{be careful}`); got != "Package mypkg Warning: be careful" {
		t.Errorf("PackageWarning => %q", got)
	}
	if got := khRun(t, `\typeout{hello}`); got != "hello" {
		t.Errorf("typeout => %q, want hello", got)
	}
	if got := khRun(t, `\wlog{note}`); got != "note" {
		t.Errorf("wlog => %q, want note", got)
	}
	// \PackageError gobbles its help (third) argument, here the conventional \@ehc.
	if got := khRun(t, `\PackageError{p}{boom}\@ehc`); got != "Package p Error: boom" {
		t.Errorf("PackageError => %q", got)
	}
	// \MessageBreak inside a warning is harmless (expands to nothing).
	if got := khRun(t, `\ClassWarning{c}{line one\MessageBreak line two}`); got != "Class c Warning: line one line two" {
		t.Errorf("ClassWarning w/ MessageBreak => %q, want single-spaced", got)
	}
}

// \AtBeginDocument / \AtEndDocument hooks fire at \begin{document} /
// \end{document}, in registration order, at the right time.
func TestKernelDocumentHooks(t *testing.T) {
	if got := khRun(t, `\AtBeginDocument{\message{HOOK}}\begin{document}\end{document}`); got != "HOOK" {
		t.Errorf("AtBeginDocument => %q, want HOOK", got)
	}
	if got := khRun(t, `\AtEndDocument{\message{END}}\begin{document}\end{document}`); got != "END" {
		t.Errorf("AtEndDocument => %q, want END", got)
	}
	// begin fires before end; two \message calls are space-joined.
	if got := khRun(t, `\AtBeginDocument{\message{B}}\AtEndDocument{\message{E}}\begin{document}\end{document}`); got != "B E" {
		t.Errorf("both hooks => %q, want \"B E\"", got)
	}
	// a hook registered but document never entered does NOT fire.
	if got := khRun(t, `\AtBeginDocument{\message{HOOK}}\message{plain}`); got != "plain" {
		t.Errorf("unfired hook => %q, want plain", got)
	}
}
