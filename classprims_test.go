// Copyright (c) the go-tex/engine authors.
// SPDX-License-Identifier: BSD-3-Clause

package engine

import (
	"strings"
	"testing"
)

// run is a small helper: a LaTeX engine, Run, message output (fatal on error).
func mustRun(t *testing.T, src string) string {
	t.Helper()
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	out, err := e.Run(src)
	if err != nil {
		t.Fatalf("run %q: %v", src, err)
	}
	return out
}

// \providecommand defines a command only when it is not already defined, so a
// fallback never clobbers an existing (engine or user) definition — unlike
// \newcommand, which always (re)defines.
func TestProvidecommandKeepsExisting(t *testing.T) {
	out := mustRun(t, `\newcommand{\foo}{ORIG}\providecommand{\foo}{FALLBACK}\message{\foo}`)
	if !strings.Contains(out, "ORIG") || strings.Contains(out, "FALLBACK") {
		t.Errorf("\\providecommand overrode an existing command; %q", out)
	}
	out = mustRun(t, `\providecommand{\bar}{NEW}\message{\bar}`)
	if !strings.Contains(out, "NEW") {
		t.Errorf("\\providecommand did not define an undefined command; %q", out)
	}
}

// A dimension written as <factor><internal dimen> (e.g. 6\p@ = 6×1pt) is a core
// TeX length form used throughout real class files; the factor scales the internal
// dimen, including a fractional factor.
func TestFactorInternalDimen(t *testing.T) {
	cases := map[string]string{
		`\newdimen\p@ \p@=1pt \dimen0=6\p@ \message{\the\dimen0}`:              "6.0pt",
		`\newdimen\p@ \p@=1pt \dimen0=2.5\p@ \message{\the\dimen0}`:            "2.5pt",
		`\newdimen\q \q=4pt \dimen0=3\q \message{\the\dimen0}`:                 "12.0pt",
		`\newdimen\p@ \p@=1pt \dimen0=6\p@ plus2\p@\message{\the\dimen0 done}`: "6.0pt", // trailing glue words not part of a \dimen
	}
	for src, want := range cases {
		if got := mustRun(t, src); !strings.Contains(got, want) {
			t.Errorf("%q => %q, want it to contain %q", src, got, want)
		}
	}
}

// \ifdim compares lengths (both branches), like \ifnum for numbers.
func TestIfdim(t *testing.T) {
	if !strings.Contains(mustRun(t, `\message{\ifdim 3pt<5pt LT\else GE\fi}`), "LT") {
		t.Error("3pt<5pt should be true")
	}
	if !strings.Contains(mustRun(t, `\message{\ifdim 9pt<5pt LT\else GE\fi}`), "GE") {
		t.Error("9pt<5pt should be false")
	}
	if !strings.Contains(mustRun(t, `\message{\ifdim 4pt=4pt EQ\else NE\fi}`), "EQ") {
		t.Error("4pt=4pt should be true")
	}
}

// \divide divides count, dimen and skip registers by an integer.
func TestDivide(t *testing.T) {
	if got := mustRun(t, `\newcount\n \n=20 \divide\n by 6 \message{\the\n}`); !strings.Contains(got, "3") {
		t.Errorf("20/6=3, got %q", got)
	}
	if got := mustRun(t, `\newdimen\d \d=12pt \divide\d by 4 \message{\the\d}`); !strings.Contains(got, "3.0pt") {
		t.Errorf("12pt/4=3pt, got %q", got)
	}
	// division by zero is ignored (no panic, value unchanged)
	if got := mustRun(t, `\newcount\z \z=7 \divide\z by 0 \message{\the\z}`); !strings.Contains(got, "7") {
		t.Errorf("divide by zero should leave 7, got %q", got)
	}
}

// \newbox allocates a box register bound to a bare control sequence.
func TestNewbox(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Run(`\newbox\mybox`); err != nil {
		t.Fatalf("newbox: %v", err)
	}
	m := e.eq["mybox"]
	if m == nil || m.kind != mBoxRef {
		t.Fatalf("\\mybox not bound to a box register: %+v", m)
	}
}

// \everypar and \sfcode accept their assignments (no-ops), and \long is an accepted
// def prefix, so a class body using them does not error.
func TestClassPrimAssignmentsAccepted(t *testing.T) {
	out := mustRun(t, `\everypar={\message{never}}\sfcode`+"`"+`\.=1000 \long\def\foo{\message{FOO}}\foo\message{ok}`)
	if strings.Contains(out, "never") {
		t.Error("\\everypar hook should not fire (no everypar model)")
	}
	if !strings.Contains(out, "FOO") || !strings.Contains(out, "ok") {
		t.Errorf("\\long\\def and following text should run; %q", out)
	}
}

// \hb@xt@ is the "\hbox to" shorthand (an internal @-command, so it needs @ to be a
// letter): it must parse a following dimension and box.
func TestHbxtShorthand(t *testing.T) {
	pages, err := CompileToSVGPages([]byte(`\documentclass{article}\begin{document}`+
		`\makeatletter\noindent\hb@xt@ 40pt{x\hfil y}\makeatother\par done\end{document}`), Options{})
	if err != nil {
		t.Fatalf("\\hb@xt@ compile: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
}

// \@startsection (the best-effort sectioning driver a real class uses) accepts its
// six layout arguments and the *|[toc]|{title} tail, typesetting the title — a real
// class's \section thus produces a heading instead of an undefined-cs error.
func TestStartsectionHeading(t *testing.T) {
	src := `\documentclass{article}\begin{document}\makeatletter` +
		`\def\sect{\@startsection{s}{1}{\z@}{1pt}{1pt}{\bfseries}}\makeatother` +
		`\sect{FirstHead}Body one.\sect*{SecondHead}Body two.\end{document}`
	pages, err := CompileToSVGPages([]byte(src), Options{})
	if err != nil {
		t.Fatalf("\\@startsection compile: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
}

// The generic \list / \item best-effort machinery typesets each item without error.
func TestGenericListItem(t *testing.T) {
	src := `\documentclass{article}\begin{document}\makeatletter` +
		`\list{}{}\item First\item[--] Second\endlist\makeatother done\end{document}`
	if _, err := CompileToSVGPages([]byte(src), Options{}); err != nil {
		t.Fatalf("\\list/\\item compile: %v", err)
	}
}

// The real article.cls (and its size option files) are embedded and resolvable, so
// the engine can load a base class with no TeX distribution present.
func TestEmbeddedBaseFilesResolvable(t *testing.T) {
	for _, name := range []string{"article.cls", "size10.clo", "size11.clo", "size12.clo"} {
		data, ok := embeddedTeXFile(name)
		if !ok {
			t.Errorf("embedded %s not found", name)
			continue
		}
		if !strings.Contains(string(data), "\\ProvidesClass") && !strings.Contains(string(data), "\\ProvidesFile") {
			t.Errorf("embedded %s does not look like a real LaTeX file", name)
		}
	}
	// A standard class stays on the built-in emulation (not routed to the embed).
	if !emulatedClasses["article"] {
		t.Error("article should be an emulated class (not routed to the real .cls yet)")
	}
}

// pageChars walks a compiled engine's main vertical list and paragraph list,
// collecting every typeset character — used to assert nothing leaks onto the page.
func pageChars(e *Engine) string {
	var s []rune
	var walk func(ns []node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case charNode:
				s = append(s, v.ch)
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	walk(e.parList)
	return string(s)
}

// The starred forms \newcommand*/\renewcommand*/\providecommand* must consume the
// '*' (short form) — otherwise it leaks onto the page as a literal asterisk, which
// is exactly how real class files (article.cls's \newcommand*\l@section…) spilled
// stray '*' glyphs.
func TestNewcommandStarConsumed(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`\newcommand*\aa{A}\renewcommand*\aa{B}\providecommand*\bb{C}\aa\bb\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatal(err)
	}
	got := pageChars(e)
	if strings.Contains(got, "*") {
		t.Errorf("a '*' from \\newcommand* leaked onto the page: %q", got)
	}
	if !strings.Contains(got, "B") || !strings.Contains(got, "C") {
		t.Errorf("starred defs did not take effect: %q", got)
	}
}

// A custom class that \LoadClass{article} (the realistic \documentclass flow, which
// sets \@ptsize so size1x.clo loads) typesets \section + itemize with no stray
// characters leaking — the class's \newcommand* and \DeclareOldFontCommand no
// longer spill '*', and the penalty/glue assignments no longer spill digits.
func TestRealClassCleanRender(t *testing.T) {
	withTempDir(t, map[string]string{
		"demoart.cls": `\DeclareOption*{\PassOptionsToClass{\CurrentOption}{article}}` +
			`\ProcessOptions\LoadClass{article}`,
	}, func() {
		src := `\documentclass{demoart}\begin{document}` +
			`\section{Intro}Body words here.\begin{itemize}\item one\item two\end{itemize}\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatalf("compile: %v", err)
		}
		got := pageChars(e)
		if strings.Contains(got, "*") {
			t.Errorf("stray '*' leaked when loading real article.cls: %q", got)
		}
		for _, w := range []string{"Intro", "Body", "one", "two"} {
			if !strings.Contains(got, w) {
				t.Errorf("expected %q in output, got %q", w, got)
			}
		}
	})
}
