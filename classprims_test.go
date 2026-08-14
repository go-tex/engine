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

// \trivlist opens a group that \endtrivlist closes, so a definition made inside
// (as a real class's \trivlist body does) does not leak out. This is what lets
// amsart's \maketitle author block (\trivlist … \endtrivlist) contain itself
// instead of leaving the list open and swallowing the following text.
func TestTrivlistScopes(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\def\gtmp{OUTER}\trivlist\def\gtmp{INNER}\endtrivlist\gtmp\end{document}`), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "OUTER") || strings.Contains(got, "INNER") {
		t.Errorf("\\def inside \\trivlist leaked past \\endtrivlist; page=%q, want OUTER (group restored)", got)
	}
}

// Body-level commands that were dropped (skip census across the corpus) now run:
// \texorpdfstring keeps its TeX form, \ensuremath wraps its argument in real math
// (a math node, not broken text), and algorithmicx keywords render as text.
func TestBodyRobustnessCommands(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\texorpdfstring{KEEPTEX}{dropPDF} \Require cond \Return done \ensuremath{q}\end{document}`), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "KEEPTEX") || strings.Contains(got, "dropPDF") {
		t.Errorf("texorpdfstring: page=%q, want KEEPTEX and not dropPDF", got)
	}
	if !strings.Contains(got, "Require") || !strings.Contains(got, "return") {
		t.Errorf("algorithmicx keywords missing: page=%q", got)
	}
	var hasMath bool
	var walk func([]node)
	walk = func(ns []node) {
		for _, n := range ns {
			switch v := n.(type) {
			case mathNode:
				hasMath = true
			case *boxNode:
				walk(v.list)
			}
		}
	}
	walk(e.mvl)
	walk(e.parList)
	if !hasMath {
		t.Error("\\ensuremath did not produce a math node (its argument was dropped/typeset as text)")
	}
}

// \everypar fires its token list at the start of every paragraph (TeX inserts
// the hook as the first horizontal material), and the setting is group-scoped so
// a list environment can install a per-item hook and have it removed at \endgroup.
func TestEveryparFires(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\everypar{X}\begin{document}`+
		`one\par two\par{\everypar{Y}inner}\par three\end{document}`), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	// Each of the three outer paragraphs is prefixed with X; the grouped paragraph
	// uses Y, and X is restored afterwards (three X's, one Y).
	if strings.Count(got, "X") != 3 {
		t.Errorf("expected 3 \\everypar{X} firings, got %q", got)
	}
	if strings.Count(got, "Y") != 1 {
		t.Errorf("expected 1 grouped \\everypar{Y} firing, got %q", got)
	}
}

// \MakeUppercase (and \MakeLowercase) case-shift the EXPANSION of their argument:
// \uppercase only touches explicit character tokens, so a control sequence such
// as \contentsname (or a running head's \leftmark) must be expanded to its letters
// first. Real classes rely on this — report.cls sets \markboth{\MakeUppercase\…}.
func TestMakeUppercaseExpandsArgument(t *testing.T) {
	e, err := compile([]byte(`\documentclass{article}\begin{document}`+
		`\def\cn{Contents}\MakeUppercase{\cn} \def\wd{HELLO}\MakeLowercase{\wd}\end{document}`), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	if !strings.Contains(got, "CONTENTS") {
		t.Errorf("\\MakeUppercase{\\cn} typeset %q, want it to contain CONTENTS (the cs expansion, shifted)", got)
	}
	if !strings.Contains(got, "hello") {
		t.Errorf("\\MakeLowercase{\\wd} typeset %q, want hello", got)
	}
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
	// article is now routed to the real embedded class (not the emulation).
	if emulatedClasses["article"] {
		t.Error("article should load the real embedded class, not the emulation")
	}
}

// \documentclass{article} loads the REAL embedded article.cls end to end and
// typesets a rich document: a numbered \tableofcontents ("Contents"), a numbered
// \section ("1 Intro") and the body — the class runs on the engine.
func TestArticleLoadsRealClass(t *testing.T) {
	src := "\\documentclass{article}\n\\title{T}\\author{A}\n\\begin{document}\n" +
		"\\maketitle\\tableofcontents\n\\section{Intro}\nBody.\n\\end{document}"
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatalf("real article.cls compile: %v", err)
	}
	if !e.loadedPackages["article"] {
		t.Fatal("article was not loaded as the real class")
	}
	got := pageChars(e)
	for _, w := range []string{"Contents", "1Intro", "Body."} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in %q", w, got)
		}
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

// A real class's \section/\subsection (via \@startsection) produce hierarchical
// numbers — "1 Alpha", "2 Beta", "2.1 Gamma" — with the subsection counter reset
// under each section, matching real LaTeX (this is what brought the geom fidelity
// doc to full 24/24 word parity with tectonic).
func TestRealClassSectionNumbering(t *testing.T) {
	withTempDir(t, map[string]string{
		"demoart.cls": `\DeclareOption*{\PassOptionsToClass{\CurrentOption}{article}}` +
			`\ProcessOptions\LoadClass{article}`,
	}, func() {
		src := `\documentclass{demoart}\begin{document}` +
			`\section{Alpha}x\section{Beta}y\subsection{Gamma}z\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		for _, w := range []string{"1Alpha", "2Beta", "2.1Gamma"} {
			if !strings.Contains(got, w) {
				t.Errorf("missing numbered heading %q in %q", w, got)
			}
		}
	})
}

// When a real class (article.cls via \LoadClass) is loaded, its \@float-based
// figure/table environments number their captions ("Figure 1:", "Table 1:"), the
// numbered \section records a \tableofcontents entry, and \tableofcontents renders
// a "Contents" heading — the class's TOC/float machinery runs on the engine.
func TestRealClassTocAndFloats(t *testing.T) {
	withTempDir(t, map[string]string{
		"demoart.cls": `\DeclareOption*{\PassOptionsToClass{\CurrentOption}{article}}` +
			`\ProcessOptions\LoadClass{article}`,
	}, func() {
		src := `\documentclass{demoart}\begin{document}\tableofcontents\section{Intro}Body.` +
			`\begin{figure}\caption{A figure}\end{figure}\begin{table}\caption{A table}\end{table}\end{document}`
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		for _, w := range []string{"Contents", "1Intro", "Figure1:", "Table1:"} {
			if !strings.Contains(got, w) {
				t.Errorf("missing %q in %q", w, got)
			}
		}
	})
}

// A class/package/\input file with CRLF (Windows) or lone-CR line endings must not
// leave stray carriage returns to be typeset — the engine treats only \n as
// end-of-line, so loaded files are normalised to LF (regression: the embedded
// article.cls checked out as CRLF on Windows spilled \r glyphs and corrupted
// captions).
func TestCRLFNormalized(t *testing.T) {
	if normalizeEOL("a\r\nb\rc\n") != "a\nb\nc\n" {
		t.Fatal("normalizeEOL did not fold CRLF/CR to LF")
	}
	withTempDir(t, map[string]string{"frag.tex": "Hello\r\nWorld\r\n"}, func() {
		e, err := compile([]byte(`\documentclass{article}\begin{document}\input{frag}\end{document}`), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		got := pageChars(e)
		if strings.ContainsRune(got, '\r') {
			t.Errorf("a stray carriage return was typeset: %q", got)
		}
		if !strings.Contains(got, "Hello") || !strings.Contains(got, "World") {
			t.Errorf("CRLF \\input content missing: %q", got)
		}
	})
}

// Loading a class splices its (hundreds of) lines into the input ahead of the
// document, but that must NOT shift the source-line numbers the engine attributes
// to the user's own text: after \LoadClass{article}, a glyph on document line 3
// still reports line 3 (not ~647). This keeps error locations and editor
// source-navigation correct when a real class is loaded.
func TestLoadDoesNotShiftSourceLines(t *testing.T) {
	withTempDir(t, map[string]string{
		"demoart.cls": `\DeclareOption*{\PassOptionsToClass{\CurrentOption}{article}}` +
			`\ProcessOptions\LoadClass{article}`,
	}, func() {
		// "Findme" sits on line 3 of the document source.
		src := "\\documentclass{demoart}\n\\begin{document}\nFindme here.\n\\end{document}"
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Fatal(err)
		}
		line := 0
		var walk func(ns []node)
		walk = func(ns []node) {
			for _, n := range ns {
				switch v := n.(type) {
				case charNode:
					if v.ch == 'F' && line == 0 {
						line = v.srcLine
					}
				case *boxNode:
					walk(v.list)
				}
			}
		}
		walk(e.mvl)
		if line != 3 {
			t.Errorf("glyph 'F' of \"Findme\" reports source line %d, want 3 (class lines must not shift the document)", line)
		}
	})
}

// \documentclass{report} and {book} load their real embedded classes and typeset
// numbered chapters ("Chapter 1"), chapter-scoped section numbers ("1.1"), the
// titles, and the body — \secdef going through \@dblarg is what lets \chapter's
// \@chapter[#1]#2 receive its title.
func TestReportBookLoadRealClass(t *testing.T) {
	for _, cls := range []string{"report", "book"} {
		src := "\\documentclass{" + cls + "}\\begin{document}" +
			"\\chapter{First}\\section{Sub}Body text here.\\end{document}"
		e, err := compile([]byte(src), Options{Lenient: true})
		if err != nil {
			t.Errorf("%s: %v", cls, err)
			continue
		}
		if !e.loadedPackages[cls] {
			t.Errorf("%s not loaded as the real class", cls)
		}
		got := pageChars(e)
		for _, w := range []string{"Chapter", "First", "1.1", "Sub", "Body"} {
			if !strings.Contains(got, w) {
				t.Errorf("%s: missing %q in %q", cls, w, got)
			}
		}
	}
}

// The common LaTeX text symbols render as real glyphs — including the ones whose
// ASCII form has a special catcode (\ ~ ^ _ { }), produced via \char so they work
// in any context. (A real document, and the wasm demo's sample, use these.)
func TestTextSymbolsRender(t *testing.T) {
	src := `\documentclass{article}\begin{document}` +
		`A\textbackslash B\textasciitilde C\textasciicircum D\textunderscore E\textbar ` +
		`F\textdagger G\textsection H\textcopyright I\textdegree\end{document}`
	e, err := compile([]byte(src), Options{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := pageChars(e)
	for _, r := range []string{"\\", "~", "^", "_", "|", "†", "§", "©", "°"} {
		if !strings.Contains(got, r) {
			t.Errorf("text symbol %q did not render: %q", r, got)
		}
	}
}
