package engine

import (
	"bytes"
	"strings"
	"testing"
)

// A small LaTeX document (documentclass + document environment + section)
// compiles to a PDF via the auto-detected LaTeX layer.
func TestLaTeXDocumentCompiles(t *testing.T) {
	src := []byte(`\documentclass[12pt]{article}
\title{A Title}
\author{An Author}
\begin{document}
\maketitle
\section{Introduction}
This is a LaTeX document compiled by the pure-Go engine. It uses the
environment mechanism and sectioning defined in the minimal LaTeX kernel.
\begin{itemize}
\item First point.
\item Second point.
\end{itemize}
\end{document}`)
	var buf bytes.Buffer
	pages, err := CompileToPDF(src, Options{}, &buf)
	if err != nil {
		t.Fatalf("LaTeX compile: %v", err)
	}
	if pages < 1 {
		t.Fatalf("expected >=1 page, got %d", pages)
	}
	if b := buf.Bytes(); string(b[:5]) != "%PDF-" {
		t.Fatalf("not a PDF")
	}
}

// \begin{env}...\end{env} runs \env and \endenv via \csname (the LaTeX mechanism).
func TestBeginEndEnvironment(t *testing.T) {
	e := New()
	if err := e.LoadLaTeX(); err != nil {
		t.Fatal(err)
	}
	// define a custom environment and check begin/end dispatch to it
	got, err := e.Run(`\def\myenv{[A]}\def\endmyenv{[B]}\message{\begin{myenv}\end{myenv}}`)
	if err != nil {
		t.Fatal(err)
	}
	if trimNL(got) != "[A][B]" {
		t.Errorf("begin/end env got %q want [A][B]", trimNL(got))
	}
}

// xstring's \IfSubStr[<n>]{<string>}{<substring>}{<true>}{<false>} has four mandatory
// arguments after an optional occurrence number (xstring.tex:444). Undefined it
// consumes nothing, and acmart opens its \author with
//
//	\IfSubStr{\detokenize{#2}}{,}{\ClassWarning{…}}{}       acmart.cls:1314
//
// so \author{A Name} in the preamble printed "A Name," as body text — the detokenized
// name and the comma it was being tested for. On a two-column paper that text takes a
// page of its own, because it lands ahead of \maketitle's \twocolumn[...]
// (go-tex/engine#319).
//
// The test is performed, not guessed: a stub answering "false" always would drop
// whichever branch it did not pick, and a branch is content.
func TestIfSubStrPerformsTheTest(t *testing.T) {
	const src = `\documentclass{article}\begin{document}
A\IfSubStr{hello world}{lo w}{YES}{NO}B
C\IfSubStr{hello world}{zebra}{YES}{NO}D
E\IfSubStr[2]{aXbXc}{X}{YES}{NO}F
\end{document}`
	e, err := compile([]byte(src), Options{Lenient: true})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	var b strings.Builder
	for _, p := range e.Pages() {
		b.WriteString(mvlText(p.list))
	}
	got := b.String()
	for _, want := range []string{"AYESB", "CNOD", "EYESF"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	// Neither the string nor the substring reaches the page.
	for _, junk := range []string{"hello", "world", "zebra"} {
		if strings.Contains(got, junk) {
			t.Errorf("an argument was typeset (%q): %q", junk, got)
		}
	}
}
